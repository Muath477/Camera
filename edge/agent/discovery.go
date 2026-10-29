package agent

import (
	"context"
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// Discovered is a camera found on the camera LAN by WS-Discovery.
type Discovered struct {
	Host      string   `json:"host"`
	XAddrs    []string `json:"xaddrs"`
	Vendor    string   `json:"vendor,omitempty"`
	Model     string   `json:"model,omitempty"`
	Name      string   `json:"name,omitempty"`
	ProfileID string   `json:"profile_id,omitempty"`
}

const wsDiscoveryAddr = "239.255.255.250:3702"

const probeTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<e:Envelope xmlns:e="http://www.w3.org/2003/05/soap-envelope"
 xmlns:w="http://schemas.xmlsoap.org/ws/2004/08/addressing"
 xmlns:d="http://schemas.xmlsoap.org/ws/2005/04/discovery"
 xmlns:dn="http://www.onvif.org/ver10/network/wsdl">
 <e:Header>
  <w:MessageID>uuid:%s</w:MessageID>
  <w:To e:mustUnderstand="true">urn:schemas-xmlsoap-org:ws:2005:04:discovery</w:To>
  <w:Action e:mustUnderstand="true">http://schemas.xmlsoap.org/ws/2005/04/discovery/Probe</w:Action>
 </e:Header>
 <e:Body><d:Probe><d:Types>dn:NetworkVideoTransmitter</d:Types></d:Probe></e:Body>
</e:Envelope>`

type probeMatches struct {
	Matches []struct {
		Scopes string `xml:"Scopes"`
		XAddrs string `xml:"XAddrs"`
	} `xml:"Body>ProbeMatches>ProbeMatch"`
}

// Discover sends a WS-Discovery probe on the given interface address
// (empty = all) and collects answers until ctx is done.
func Discover(ctx context.Context, localAddr string, profiles *Profiles) ([]Discovered, error) {
	laddr, err := net.ResolveUDPAddr("udp4", localAddr+":0")
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	dst, _ := net.ResolveUDPAddr("udp4", wsDiscoveryAddr)
	if _, err := conn.WriteToUDP([]byte(fmt.Sprintf(probeTemplate, newUUID())), dst); err != nil {
		return nil, err
	}

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(3 * time.Second)
	}
	_ = conn.SetReadDeadline(deadline)

	seen := map[string]bool{}
	var out []Discovered
	buf := make([]byte, 64*1024)
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			break // deadline reached
		}
		for _, d := range parseProbeMatches(buf[:n]) {
			if seen[d.Host] {
				continue
			}
			seen[d.Host] = true
			if profiles != nil {
				if p := profiles.Match(d.Vendor, d.Model); p != nil {
					d.ProfileID = p.ID
				}
			}
			out = append(out, d)
		}
	}
	return out, nil
}

// parseProbeMatches extracts cameras from a ProbeMatches reply. ONVIF scopes
// carry the vendor and model, e.g. onvif://www.onvif.org/hardware/DS-2CD2143G2.
func parseProbeMatches(raw []byte) []Discovered {
	var pm probeMatches
	if err := xml.Unmarshal(raw, &pm); err != nil {
		return nil
	}
	var out []Discovered
	for _, m := range pm.Matches {
		d := Discovered{XAddrs: strings.Fields(m.XAddrs)}
		for _, x := range d.XAddrs {
			if u, err := url.Parse(x); err == nil && u.Hostname() != "" {
				d.Host = u.Hostname()
				break
			}
		}
		if d.Host == "" {
			continue
		}
		for _, s := range strings.Fields(m.Scopes) {
			val := func(prefix string) string {
				v, _ := url.PathUnescape(strings.TrimPrefix(s, prefix))
				return v
			}
			switch {
			case strings.HasPrefix(s, "onvif://www.onvif.org/hardware/"):
				d.Model = val("onvif://www.onvif.org/hardware/")
			case strings.HasPrefix(s, "onvif://www.onvif.org/name/"):
				d.Name = val("onvif://www.onvif.org/name/")
			case strings.HasPrefix(s, "onvif://www.onvif.org/mfr/"):
				d.Vendor = val("onvif://www.onvif.org/mfr/")
			}
		}
		if d.Vendor == "" {
			d.Vendor = guessVendor(d.Name + " " + d.Model)
		}
		d.Vendor = normalizeVendor(d.Vendor)
		out = append(out, d)
	}
	return out
}

// Many cameras put the brand in the name scope only.
func guessVendor(s string) string {
	s = strings.ToLower(s)
	for _, v := range []string{"hikvision", "ezviz", "dahua", "imou", "reolink", "axis", "hanwha", "uniview", "tp-link", "tapo"} {
		if strings.Contains(s, v) {
			return v
		}
	}
	switch {
	case strings.HasPrefix(s, "ds-"):
		return "hikvision"
	case strings.HasPrefix(s, "ipc-") || strings.HasPrefix(s, "dh-"):
		return "dahua"
	case strings.HasPrefix(s, "rlc-"):
		return "reolink"
	}
	return ""
}

func normalizeVendor(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch {
	case strings.Contains(v, "hikvision"):
		return "hikvision"
	case strings.Contains(v, "dahua"):
		return "dahua"
	case strings.Contains(v, "samsung") || strings.Contains(v, "hanwha") || strings.Contains(v, "wisenet"):
		return "hanwha"
	case strings.Contains(v, "tapo") || strings.Contains(v, "tp-link"):
		return "tp-link"
	}
	return v
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
