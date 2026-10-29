package agent

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Profile describes how to talk to one camera vendor or model family.
type Profile struct {
	ID           string   `yaml:"id"`
	Vendor       string   `yaml:"vendor"`
	ModelPattern string   `yaml:"model_pattern"` // regexp, empty matches every model of the vendor
	API          string   `yaml:"api"`           // onvif | isapi | dahua | vapix | sunapi | rtsp
	RTSPMain     string   `yaml:"rtsp_main"`     // template with {host} {port} {channel}
	RTSPSub      string   `yaml:"rtsp_sub"`
	RTSPPort     int      `yaml:"rtsp_port"`
	ONVIFEvents  bool     `yaml:"onvif_events"`
	ONVIFPTZ     bool     `yaml:"onvif_ptz"`
	Quirks       []string `yaml:"quirks"`
	Recommended  struct {
		MainCodec string `yaml:"main_codec"`
		MainKbps  int    `yaml:"main_kbps"`
		SubWidth  int    `yaml:"sub_width"`
		SubHeight int    `yaml:"sub_height"`
		SubFPS    int    `yaml:"sub_fps"`
	} `yaml:"recommended"`

	model *regexp.Regexp
}

// Profiles is the set loaded from the profiles directory.
type Profiles struct {
	list []*Profile
}

// LoadProfiles reads every *.yaml file in dir. Each file holds a list of profiles.
func LoadProfiles(dir string) (*Profiles, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	ps := &Profiles{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var batch []*Profile
		if err := yaml.Unmarshal(raw, &batch); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		for _, p := range batch {
			if err := p.compile(); err != nil {
				return nil, fmt.Errorf("%s: profile %s: %w", f, p.ID, err)
			}
			ps.list = append(ps.list, p)
		}
	}
	return ps, nil
}

func (p *Profile) compile() error {
	if p.ID == "" || p.Vendor == "" {
		return fmt.Errorf("id and vendor are required")
	}
	if p.RTSPPort == 0 {
		p.RTSPPort = 554
	}
	if p.ModelPattern != "" {
		re, err := regexp.Compile("(?i)" + p.ModelPattern)
		if err != nil {
			return err
		}
		p.model = re
	}
	return nil
}

// Match returns the most specific profile for a vendor and model. A profile
// with a model pattern wins over the vendor-wide one. The generic profile
// (vendor "generic") is the fallback.
func (ps *Profiles) Match(vendor, model string) *Profile {
	var vendorWide, generic *Profile
	for _, p := range ps.list {
		if strings.EqualFold(p.Vendor, "generic") {
			generic = p
			continue
		}
		if !strings.EqualFold(p.Vendor, vendor) {
			continue
		}
		if p.model == nil {
			vendorWide = p
			continue
		}
		if p.model.MatchString(model) {
			return p
		}
	}
	if vendorWide != nil {
		return vendorWide
	}
	return generic
}

// Get returns a profile by id.
func (ps *Profiles) Get(id string) *Profile {
	for _, p := range ps.list {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// HasQuirk reports whether the profile lists a known quirk.
func (p *Profile) HasQuirk(q string) bool {
	for _, x := range p.Quirks {
		if x == q {
			return true
		}
	}
	return false
}

// StreamURLs fills the RTSP templates for a camera.
func (p *Profile) StreamURLs(host, user, pass string, channel int) (main, sub string) {
	return p.fill(p.RTSPMain, host, user, pass, channel), p.fill(p.RTSPSub, host, user, pass, channel)
}

func (p *Profile) fill(tmpl, host, user, pass string, channel int) string {
	if tmpl == "" {
		return ""
	}
	if channel == 0 {
		channel = 1
	}
	r := strings.NewReplacer(
		"{host}", host,
		"{port}", fmt.Sprint(p.RTSPPort),
		"{channel}", fmt.Sprint(channel),
		"{channel0}", fmt.Sprint(channel-1),
	)
	u, err := url.Parse(r.Replace(tmpl))
	if err != nil {
		return ""
	}
	if user != "" {
		u.User = url.UserPassword(user, pass)
	}
	return u.String()
}
