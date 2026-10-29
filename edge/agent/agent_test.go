package agent

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Muath477/Camera/contract"
)

func testProfiles(t *testing.T) *Profiles {
	t.Helper()
	ps, err := LoadProfiles("../profiles")
	if err != nil {
		t.Fatal(err)
	}
	return ps
}

func TestProfileMatch(t *testing.T) {
	ps := testProfiles(t)
	cases := []struct{ vendor, model, want string }{
		{"hikvision", "DS-2CD2143G2-I", "hikvision"},
		{"Reolink", "RLC-810A", "reolink-h265"},
		{"reolink", "E1 Zoom", "reolink"},
		{"unknown-brand", "X1", "generic"},
	}
	for _, c := range cases {
		p := ps.Match(c.vendor, c.model)
		if p == nil || p.ID != c.want {
			t.Errorf("Match(%q,%q) = %v, want %s", c.vendor, c.model, p, c.want)
		}
	}
}

func TestStreamURLs(t *testing.T) {
	p := testProfiles(t).Get("hikvision")
	main, sub := p.StreamURLs("192.168.50.11", "admin", "p@ss:word", 1)
	if main != "rtsp://admin:p%40ss%3Aword@192.168.50.11:554/Streaming/Channels/101" {
		t.Errorf("main = %s", main)
	}
	if !strings.HasSuffix(sub, "/Streaming/Channels/102") {
		t.Errorf("sub = %s", sub)
	}

	dahua := testProfiles(t).Get("dahua")
	main, _ = dahua.StreamURLs("10.0.0.5", "", "", 2)
	if main != "rtsp://10.0.0.5:554/cam/realmonitor?channel=2&subtype=0" {
		t.Errorf("dahua main = %s", main)
	}
}

func TestParseProbeMatches(t *testing.T) {
	raw := `<?xml version="1.0"?>
<env:Envelope xmlns:env="http://www.w3.org/2003/05/soap-envelope" xmlns:d="http://schemas.xmlsoap.org/ws/2005/04/discovery">
<env:Body><d:ProbeMatches><d:ProbeMatch>
<d:Scopes>onvif://www.onvif.org/type/video_encoder onvif://www.onvif.org/name/HIKVISION%20DS-2CD2143G2-I onvif://www.onvif.org/hardware/DS-2CD2143G2-I</d:Scopes>
<d:XAddrs>http://192.168.50.11/onvif/device_service http://[fe80::1]/onvif/device_service</d:XAddrs>
</d:ProbeMatch></d:ProbeMatches></env:Body></env:Envelope>`
	got := parseProbeMatches([]byte(raw))
	if len(got) != 1 {
		t.Fatalf("got %d matches", len(got))
	}
	d := got[0]
	if d.Host != "192.168.50.11" || d.Vendor != "hikvision" || d.Model != "DS-2CD2143G2-I" {
		t.Errorf("unexpected %+v", d)
	}
}

func TestFrigateConfig(t *testing.T) {
	cfg := &Config{RetentionDays: 45}
	cfg.Frigate.Detector = "openvino"
	cams := []Camera{
		{Name: "entrance", Host: "192.168.50.11", Vendor: "hikvision", ProfileID: "hikvision", Username: "admin", Password: "x", Audio: true},
		{Name: "store", RTSPMain: "rtsp://192.168.50.20/live"},
	}
	out, err := FrigateConfig(cfg, cams, testProfiles(t))
	if err != nil {
		t.Fatal(err)
	}
	var fc frigateConfig
	if err := yaml.Unmarshal(out, &fc); err != nil {
		t.Fatal(err)
	}
	if fc.Record.Retain.Days != 45 {
		t.Errorf("retain days = %d", fc.Record.Retain.Days)
	}
	if len(fc.Go2RTC.Streams) != 3 {
		t.Errorf("streams = %v", fc.Go2RTC.Streams)
	}
	ent := fc.Cameras["entrance"]
	if len(ent.FFmpeg.Inputs) != 2 || ent.FFmpeg.Inputs[1].Roles[0] != "detect" {
		t.Errorf("entrance inputs = %+v", ent.FFmpeg.Inputs)
	}
	if ent.Audio == nil || !ent.Audio.Enabled {
		t.Error("audio not enabled")
	}
	if got := fc.Cameras["store"].FFmpeg.Inputs; len(got) != 1 || len(got[0].Roles) != 2 {
		t.Errorf("store inputs = %+v", got)
	}

	if _, err := FrigateConfig(cfg, []Camera{{Name: "bad name", RTSPMain: "rtsp://x"}}, testProfiles(t)); err == nil {
		t.Error("expected invalid name error")
	}
}

func TestMapFrigateEvent(t *testing.T) {
	end := `{"type":"end","after":{"id":"1730000000.1-abc","camera":"entrance","label":"person","top_score":0.84,"score":0.7,
		"start_time":1730000000.5,"end_time":1730000012.0,"entered_zones":["door"],"has_snapshot":true}}`
	ev, snap, err := MapFrigateEvent([]byte(end))
	if err != nil || ev == nil {
		t.Fatalf("ev=%v err=%v", ev, err)
	}
	if !snap || ev.Kind != contract.EventObject || ev.Score != 0.84 || ev.EndTime == nil || ev.Zones[0] != "door" {
		t.Errorf("unexpected %+v snap=%v", ev, snap)
	}
	if ev.StartTime.UnixMilli() != 1730000000500 {
		t.Errorf("start = %v", ev.StartTime)
	}

	ev, _, _ = MapFrigateEvent([]byte(`{"type":"new","after":{"id":"2","camera":"c","label":"glass","score":0.9,"start_time":1}}`))
	if ev == nil || ev.Kind != contract.EventAudio {
		t.Errorf("audio event = %+v", ev)
	}

	ev, _, _ = MapFrigateEvent([]byte(`{"type":"update","after":{"id":"3"}}`))
	if ev != nil {
		t.Error("update should be dropped")
	}
}

func TestRetentionDays(t *testing.T) {
	// 8 cameras at 2 Mbps on a 4 TB disk.
	var rates []float64
	for i := 0; i < 8; i++ {
		rates = append(rates, KbpsToMBPerHour(2048))
	}
	days := RetentionDays(4e12, rates)
	if math.Abs(days-20.35) > 0.1 {
		t.Errorf("days = %.2f", days)
	}
	if RetentionDays(4e12, nil) != 0 {
		t.Error("no cameras should give 0")
	}
}

func TestOutboxOrderAndFailure(t *testing.T) {
	o, err := OpenOutbox(filepath.Join(t.TempDir(), "o.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	for i := 0; i < 5; i++ {
		_ = o.Push("t", []byte(fmt.Sprint(i)))
	}
	var got []string
	n, err := o.Drain(10, func(_ string, p []byte) error {
		if string(p) == "3" {
			return fmt.Errorf("link down")
		}
		got = append(got, string(p))
		return nil
	})
	if n != 3 || err == nil || strings.Join(got, "") != "012" {
		t.Errorf("n=%d err=%v got=%v", n, err, got)
	}
	if o.Len() != 2 {
		t.Errorf("len = %d", o.Len())
	}
}

func TestStoreEncrypted(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(filepath.Join(dir, "c.enc"), filepath.Join(dir, "k"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Put(Camera{Name: "a", Password: "secret-pass"}); err != nil {
		t.Fatal(err)
	}
	s2, err := OpenStore(filepath.Join(dir, "c.enc"), filepath.Join(dir, "k"))
	if err != nil {
		t.Fatal(err)
	}
	if l := s2.List(); len(l) != 1 || l[0].Password != "secret-pass" {
		t.Errorf("reload = %+v", l)
	}
}

func TestTopic(t *testing.T) {
	topic := contract.Topic("ten1", "dev9", contract.KindCmdAck)
	ten, dev, kind, err := contract.ParseTopic(topic)
	if err != nil || ten != "ten1" || dev != "dev9" || kind != "cmd/ack" {
		t.Errorf("%s -> %s %s %s %v", topic, ten, dev, kind, err)
	}
}
