package agent

import (
	"bufio"
	"context"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestParseSubnets(t *testing.T) {
	got, err := ParseSubnets([]string{"192.168.1.0/24", "10.0.5.7", " ", "172.16.0.0/16"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"192.168.1.0/24", "10.0.5.0/24", "172.16.0.0/24"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i].String() != want[i] {
			t.Errorf("subnet %d = %s, want %s", i, got[i], want[i])
		}
	}
	for _, bad := range []string{"8.8.8.0/24", "example.com", "fd00::/64"} {
		if _, err := ParseSubnets([]string{bad}); err == nil {
			t.Errorf("ParseSubnets(%q) accepted", bad)
		}
	}
}

func TestHostsInSkipsNetworkBroadcastAndSelf(t *testing.T) {
	p := netip.MustParsePrefix("192.168.1.0/24")
	self := netip.MustParseAddr("192.168.1.20")
	hosts := hostsIn([]netip.Prefix{p}, map[netip.Addr]bool{self: true})
	if len(hosts) != 253 {
		t.Fatalf("got %d hosts, want 253", len(hosts))
	}
	if hosts[0].String() != "192.168.1.1" || hosts[len(hosts)-1].String() != "192.168.1.254" {
		t.Errorf("range %s..%s", hosts[0], hosts[len(hosts)-1])
	}
	for _, h := range hosts {
		if h == self {
			t.Error("own address scanned")
		}
	}
}

func TestParseRTSPServer(t *testing.T) {
	resp := "RTSP/1.0 200 OK\r\nCSeq: 1\r\nServer: Hikvision-Webs\r\nPublic: OPTIONS, DESCRIBE\r\n\r\n"
	if got := parseRTSPServer(bufio.NewReader(strings.NewReader(resp))); got != "Hikvision-Webs" {
		t.Errorf("server = %q", got)
	}
	if got := parseRTSPServer(bufio.NewReader(strings.NewReader("HTTP/1.1 200 OK\r\n\r\n"))); got != "" {
		t.Errorf("an HTTP reply gave %q", got)
	}
	if v := vendorFromBanner("Dahua Rtsp Server"); v != "dahua" {
		t.Errorf("vendor = %q", v)
	}
	if v := vendorFromPorts([]int{554, 2020}); v != "tp-link" {
		t.Errorf("vendor from ports = %q", v)
	}
}

// A fake camera on localhost: an RTSP server that answers OPTIONS.
func fakeRTSP(t *testing.T, server string) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				r := bufio.NewReader(c)
				for {
					line, err := r.ReadString('\n')
					if err != nil || strings.TrimSpace(line) == "" {
						break
					}
				}
				c.Write([]byte("RTSP/1.0 200 OK\r\nCSeq: 1\r\nServer: " + server + "\r\n\r\n"))
			}()
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestScanFindsRTSPHostsOnly(t *testing.T) {
	port := fakeRTSP(t, "Reolink Rtsp Server")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	found := scanHosts(ctx, []netip.Addr{netip.MustParseAddr("127.0.0.1")}, []int{port}, nil, nil)
	if len(found) != 1 {
		t.Fatalf("found %+v", found)
	}
	d := found[0]
	if d.Host != "127.0.0.1" || d.RTSPPort != port || d.Vendor != "reolink" || d.Method != "scan" {
		t.Errorf("found %+v", d)
	}

	// A host with only a web port open is not reported as a camera.
	web := fakeRTSP(t, "nginx")
	none := scanHosts(ctx, []netip.Addr{netip.MustParseAddr("127.0.0.1")}, []int{1}, []int{web}, nil)
	if len(none) != 0 {
		t.Errorf("a web-only host was reported: %+v", none)
	}
}

func TestMergeKeepsONVIFDetails(t *testing.T) {
	onvif := []Discovered{{Host: "192.168.1.10", Vendor: "hikvision", Model: "DS-2CD2143G2-I"}}
	scanned := []Discovered{
		{Host: "192.168.1.10", RTSPPort: 554, Ports: []int{80, 554}, Method: "scan"},
		{Host: "192.168.1.11", Vendor: "tp-link", RTSPPort: 554, Method: "scan"},
	}
	got := mergeDiscovered(onvif, scanned)
	if len(got) != 2 {
		t.Fatalf("merged %+v", got)
	}
	if got[0].Method != "onvif" || got[0].Model != "DS-2CD2143G2-I" || got[0].RTSPPort != 554 {
		t.Errorf("onvif camera lost detail: %+v", got[0])
	}
	if got[1].Host != "192.168.1.11" || got[1].Method != "scan" {
		t.Errorf("scanned camera: %+v", got[1])
	}
}
