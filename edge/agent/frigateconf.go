package agent

import (
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

var cameraNameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// ValidCameraName reports whether name is usable as a Frigate camera id.
func ValidCameraName(name string) bool { return cameraNameRe.MatchString(name) }

type frigateConfig struct {
	MQTT struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"mqtt"`
	Detectors map[string]map[string]string `yaml:"detectors"`
	Model     map[string]any               `yaml:"model,omitempty"`
	Go2RTC    struct {
		Streams map[string][]string `yaml:"streams"`
	} `yaml:"go2rtc"`
	Record struct {
		Enabled bool `yaml:"enabled"`
		Retain  struct {
			Days int    `yaml:"days"`
			Mode string `yaml:"mode"`
		} `yaml:"retain"`
	} `yaml:"record"`
	Snapshots struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"snapshots"`
	SemanticSearch struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"semantic_search"`
	Cameras map[string]frigateCamera `yaml:"cameras"`
}

type frigateInput struct {
	Path      string   `yaml:"path"`
	InputArgs string   `yaml:"input_args"`
	Roles     []string `yaml:"roles"`
}

type frigateCamera struct {
	FFmpeg struct {
		Inputs []frigateInput `yaml:"inputs"`
	} `yaml:"ffmpeg"`
	Detect struct {
		Width  int `yaml:"width"`
		Height int `yaml:"height"`
		FPS    int `yaml:"fps"`
	} `yaml:"detect"`
	Objects struct {
		Track []string `yaml:"track"`
	} `yaml:"objects"`
	Audio *struct {
		Enabled bool     `yaml:"enabled"`
		Listen  []string `yaml:"listen"`
	} `yaml:"audio,omitempty"`
}

var defaultTrack = []string{"person", "car", "motorcycle", "bicycle"}

// defaultAudio is what the platform listens for when audio is enabled.
var defaultAudio = []string{"scream", "yell", "glass", "smoke_detector", "fire_alarm", "siren", "gunshot"}

// FrigateConfig renders the Frigate configuration for the given cameras.
// go2rtc holds the single connection to each camera; Frigate reads the
// restream so cheap cameras never see more than two sessions.
func FrigateConfig(cfg *Config, cams []Camera, profiles *Profiles) ([]byte, error) {
	var fc frigateConfig
	fc.MQTT.Host = "mosquitto"
	fc.MQTT.Port = 1883
	fc.Detectors = detectorFor(cfg.Frigate.Detector)
	fc.Go2RTC.Streams = map[string][]string{}
	fc.Record.Enabled = true
	fc.Record.Retain.Days = cfg.RetentionDays
	fc.Record.Retain.Mode = "all"
	fc.Snapshots.Enabled = true
	fc.SemanticSearch.Enabled = true
	fc.Cameras = map[string]frigateCamera{}

	for _, c := range cams {
		if !ValidCameraName(c.Name) {
			return nil, fmt.Errorf("invalid camera name %q", c.Name)
		}
		main, sub := c.RTSPMain, c.RTSPSub
		p := profiles.Get(c.ProfileID)
		if p == nil {
			p = profiles.Match(c.Vendor, c.Model)
		}
		if main == "" && p != nil {
			main, sub = p.StreamURLs(c.Host, c.Username, c.Password, c.Channel)
		}
		if main == "" {
			return nil, fmt.Errorf("camera %s: no stream url", c.Name)
		}

		fc.Go2RTC.Streams[c.Name] = []string{main}
		detectStream := c.Name
		if sub != "" {
			detectStream = c.Name + "_sub"
			fc.Go2RTC.Streams[detectStream] = []string{sub}
		}

		var fcam frigateCamera
		restream := func(name string) string { return "rtsp://127.0.0.1:8554/" + name }
		if detectStream == c.Name {
			fcam.FFmpeg.Inputs = []frigateInput{{Path: restream(c.Name), InputArgs: "preset-rtsp-restream", Roles: []string{"record", "detect"}}}
		} else {
			fcam.FFmpeg.Inputs = []frigateInput{
				{Path: restream(c.Name), InputArgs: "preset-rtsp-restream", Roles: []string{"record"}},
				{Path: restream(detectStream), InputArgs: "preset-rtsp-restream", Roles: []string{"detect"}},
			}
		}
		fcam.Detect.Width, fcam.Detect.Height, fcam.Detect.FPS = 640, 480, 5
		if p != nil && p.Recommended.SubWidth > 0 {
			fcam.Detect.Width = p.Recommended.SubWidth
			fcam.Detect.Height = p.Recommended.SubHeight
			fcam.Detect.FPS = p.Recommended.SubFPS
		}
		fcam.Objects.Track = c.Track
		if len(fcam.Objects.Track) == 0 {
			fcam.Objects.Track = defaultTrack
		}
		if c.Audio {
			fcam.Audio = &struct {
				Enabled bool     `yaml:"enabled"`
				Listen  []string `yaml:"listen"`
			}{true, defaultAudio}
			fcam.FFmpeg.Inputs[0].Roles = append(fcam.FFmpeg.Inputs[0].Roles, "audio")
		}
		fc.Cameras[c.Name] = fcam
	}
	return yaml.Marshal(&fc)
}

func detectorFor(kind string) map[string]map[string]string {
	switch kind {
	case "hailo8l", "hailo":
		return map[string]map[string]string{"hailo": {"type": "hailo8l", "device": "PCIe"}}
	case "cpu":
		return map[string]map[string]string{"cpu": {"type": "cpu"}}
	default:
		return map[string]map[string]string{"ov": {"type": "openvino", "device": "AUTO"}}
	}
}
