package agent

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"
)

// Cameras on a home network are often reachable only through the router:
// many (Tapo, Ezviz, Imou, budget models) ship with ONVIF off, so they never
// answer WS-Discovery. ScanSubnets finds them by trying the ports cameras
// listen on, then asks the RTSP server who it is.

// rtspPorts mark a host as a camera; extraPorts only add detail.
var (
	rtspPorts  = []int{554, 8554}
	extraPorts = []int{80, 2020, 8000, 8899, 34567, 37777}
)

const (
	dialTimeout = 400 * time.Millisecond
	scanWorkers = 128
	// maxHosts keeps a mistyped /16 from turning into a 65k-host sweep.
	maxHosts = 1024
)

// LocalSubnets lists the private IPv4 networks this machine is on, each
// narrowed to at most a /24 around the machine's own address.
func LocalSubnets() []netip.Prefix {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []netip.Prefix
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip, ok := netip.AddrFromSlice(ipn.IP.To4())
			if !ok || !ip.IsPrivate() {
				continue
			}
			bits, _ := ipn.Mask.Size()
			out = append(out, narrow(netip.PrefixFrom(ip, bits)))
		}
	}
	return dedupe(out)
}

// ParseSubnets reads CIDR strings such as "192.168.1.0/24". A bare address
// means its /24. Only private IPv4 ranges are accepted: the agent must never
// be turned into a scanner of someone else's network.
func ParseSubnets(list []string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, s := range list {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !strings.Contains(s, "/") {
			s += "/24"
		}
		p, err := netip.ParsePrefix(s)
		if err != nil || !p.Addr().Is4() {
			return nil, fmt.Errorf("not an IPv4 network: %q", s)
		}
		if !p.Addr().IsPrivate() {
			return nil, fmt.Errorf("%s is not a private network", p)
		}
		out = append(out, narrow(p))
	}
	return dedupe(out), nil
}

func narrow(p netip.Prefix) netip.Prefix {
	if p.Bits() < 24 {
		p = netip.PrefixFrom(p.Addr(), 24)
	}
	return p.Masked()
}

func dedupe(ps []netip.Prefix) []netip.Prefix {
	seen := map[netip.Prefix]bool{}
	var out []netip.Prefix
	for _, p := range ps {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// hostsIn lists the usable addresses of each network, skipping the network
// and broadcast addresses and this machine's own addresses.
func hostsIn(subnets []netip.Prefix, skip map[netip.Addr]bool) []netip.Addr {
	var out []netip.Addr
	for _, p := range subnets {
		first := p.Masked().Addr()
		for a := first.Next(); p.Contains(a) && len(out) < maxHosts; a = a.Next() {
			if !p.Contains(a.Next()) {
				break // broadcast
			}
			if !skip[a] {
				out = append(out, a)
			}
		}
	}
	return out
}

func ownAddrs() map[netip.Addr]bool {
	own := map[netip.Addr]bool{}
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok {
			if ip, ok := netip.AddrFromSlice(ipn.IP.To4()); ok {
				own[ip] = true
			}
		}
	}
	return own
}

// ScanSubnets probes every host in the networks and returns the ones that
// run an RTSP server.
func ScanSubnets(ctx context.Context, subnets []netip.Prefix, profiles *Profiles) []Discovered {
	return scanHosts(ctx, hostsIn(subnets, ownAddrs()), rtspPorts, extraPorts, profiles)
}

func scanHosts(ctx context.Context, hosts []netip.Addr, camPorts, infoPorts []int, profiles *Profiles) []Discovered {
	// Phase one sweeps only the camera ports and keeps part of the time
	// budget back, so phase two (detail ports and the RTSP banner, on the
	// few hosts that answered) always has time to run.
	sweep := ctx
	if dl, ok := ctx.Deadline(); ok {
		var cancel context.CancelFunc
		sweep, cancel = context.WithTimeout(ctx, time.Until(dl)*6/10)
		defer cancel()
	}
	open := probe(sweep, hosts, camPorts)

	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		out []Discovered
	)
	for host, ps := range open {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rtsp := ps[0]
			for _, p := range ps {
				rtsp = min(rtsp, p)
			}
			more := probe(ctx, []netip.Addr{host}, infoPorts)[host]
			all := append(append([]int{}, ps...), more...)
			sort.Ints(all)
			d := Discovered{Host: host.String(), Ports: all, RTSPPort: rtsp, Method: "scan"}
			d.Vendor = normalizeVendor(vendorFromBanner(rtspServer(ctx, host.String(), rtsp)))
			if d.Vendor == "" {
				d.Vendor = vendorFromPorts(all)
			}
			if profiles != nil {
				if p := profiles.Match(d.Vendor, ""); p != nil {
					d.ProfileID = p.ID
				}
			}
			mu.Lock()
			out = append(out, d)
			mu.Unlock()
		}()
	}
	wg.Wait()
	sort.Slice(out, func(i, j int) bool {
		a, _ := netip.ParseAddr(out[i].Host)
		b, _ := netip.ParseAddr(out[j].Host)
		return a.Less(b)
	})
	return out
}

// probe tries every port on every host and returns the open ones by host.
func probe(ctx context.Context, hosts []netip.Addr, ports []int) map[netip.Addr][]int {
	type job struct {
		host netip.Addr
		port int
	}
	jobs := make(chan job)
	var mu sync.Mutex
	open := map[netip.Addr][]int{}

	var wg sync.WaitGroup
	for range min(scanWorkers, max(1, len(hosts)*len(ports))) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var d net.Dialer
			for j := range jobs {
				dctx, cancel := context.WithTimeout(ctx, dialTimeout)
				c, err := d.DialContext(dctx, "tcp", netip.AddrPortFrom(j.host, uint16(j.port)).String())
				cancel()
				if err != nil {
					continue
				}
				c.Close()
				mu.Lock()
				open[j.host] = append(open[j.host], j.port)
				mu.Unlock()
			}
		}()
	}
feed:
	for _, port := range ports {
		for _, h := range hosts {
			select {
			case jobs <- job{h, port}:
			case <-ctx.Done():
				break feed
			}
		}
	}
	close(jobs)
	wg.Wait()
	return open
}

// rtspServer sends an RTSP OPTIONS request and returns the Server header.
// It needs no credentials: cameras answer OPTIONS before authentication.
func rtspServer(ctx context.Context, host string, port int) string {
	var d net.Dialer
	dctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	c, err := d.DialContext(dctx, "tcp", net.JoinHostPort(host, fmt.Sprint(port)))
	if err != nil {
		return ""
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))
	fmt.Fprintf(c, "OPTIONS rtsp://%s:%d/ RTSP/1.0\r\nCSeq: 1\r\nUser-Agent: camera-agent\r\n\r\n", host, port)
	return parseRTSPServer(bufio.NewReader(c))
}

func parseRTSPServer(r *bufio.Reader) string {
	line, err := r.ReadString('\n')
	if err != nil || !strings.HasPrefix(line, "RTSP/") {
		return ""
	}
	for i := 0; i < 32; i++ {
		line, err = r.ReadString('\n')
		line = strings.TrimSpace(line)
		if err != nil || line == "" {
			return ""
		}
		if k, v, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(k), "server") {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// vendorFromBanner maps RTSP Server headers seen on common cameras.
func vendorFromBanner(s string) string {
	l := strings.ToLower(s)
	switch {
	case l == "":
		return ""
	case strings.Contains(l, "hikvision"):
		return "hikvision"
	case strings.Contains(l, "dahua"):
		return "dahua"
	case strings.Contains(l, "reolink"):
		return "reolink"
	case strings.Contains(l, "axis"):
		return "axis"
	case strings.Contains(l, "tp-link") || strings.Contains(l, "tapo"):
		return "tp-link"
	case strings.Contains(l, "ezviz"):
		return "ezviz"
	case strings.Contains(l, "uniview"):
		return "uniview"
	case strings.Contains(l, "hanwha") || strings.Contains(l, "wisenet"):
		return "hanwha"
	}
	return guessVendor(s)
}

// vendorFromPorts uses ports that only one family opens.
func vendorFromPorts(ps []int) string {
	for _, p := range ps {
		switch p {
		case 37777:
			return "dahua"
		case 2020:
			return "tp-link" // Tapo's ONVIF port
		case 34567:
			return "xiongmai"
		}
	}
	return ""
}

// DiscoverAll runs WS-Discovery and the port scan together and merges the
// results by address; ONVIF answers keep their richer vendor and model.
func DiscoverAll(ctx context.Context, interfaceAddr string, subnets []netip.Prefix, profiles *Profiles) ([]Discovered, error) {
	var (
		wg      sync.WaitGroup
		onvif   []Discovered
		scanned []Discovered
		werr    error
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		// ONVIF cameras answer the probe within a second or two; Discover
		// listens until its deadline, so do not let it hold the whole budget.
		octx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		onvif, werr = Discover(octx, interfaceAddr, profiles)
	}()
	go func() {
		defer wg.Done()
		scanned = ScanSubnets(ctx, subnets, profiles)
	}()
	wg.Wait()
	out := mergeDiscovered(onvif, scanned)
	if len(out) == 0 && werr != nil {
		return nil, werr
	}
	return out, nil
}

func mergeDiscovered(onvif, scanned []Discovered) []Discovered {
	byHost := map[string]int{}
	var out []Discovered
	for _, d := range onvif {
		d.Method = "onvif"
		byHost[d.Host] = len(out)
		out = append(out, d)
	}
	for _, s := range scanned {
		if i, ok := byHost[s.Host]; ok {
			out[i].Ports, out[i].RTSPPort = s.Ports, s.RTSPPort
			if out[i].Vendor == "" {
				out[i].Vendor, out[i].ProfileID = s.Vendor, s.ProfileID
			}
			continue
		}
		out = append(out, s)
	}
	return out
}
