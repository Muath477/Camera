package agent

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the agent's local configuration file (/etc/camera/agent.yaml).
type Config struct {
	DeviceID string `yaml:"device_id"`
	TenantID string `yaml:"tenant_id"`

	Cloud struct {
		BrokerURL string `yaml:"broker_url"` // ssl://mqtt.example.sa:8883
		CAFile    string `yaml:"ca_file"`
		CertFile  string `yaml:"cert_file"`
		KeyFile   string `yaml:"key_file"`
		APIURL    string `yaml:"api_url"` // https://device-api.example.sa, same client certificate
	} `yaml:"cloud"`

	Frigate struct {
		URL        string `yaml:"url"`         // http://frigate:5000
		BrokerURL  string `yaml:"broker_url"`  // tcp://mosquitto:1883
		ConfigPath string `yaml:"config_path"` // /config/config.yml
		Detector   string `yaml:"detector"`    // openvino | hailo8l | cpu
	} `yaml:"frigate"`

	Go2RTCURL string `yaml:"go2rtc_url"` // http://go2rtc:1984

	DataDir        string        `yaml:"data_dir"`        // outbox, camera store
	RecordingsPath string        `yaml:"recordings_path"` // dedicated recordings disk
	ProfilesDir    string        `yaml:"profiles_dir"`
	KeyFile        string        `yaml:"key_file"` // local secrets key, sealed by the TPM in production
	RetentionDays  int           `yaml:"retention_days"`
	HealthInterval time.Duration `yaml:"health_interval"`
	ThumbInterval  time.Duration `yaml:"thumb_interval"`
}

// LoadConfig reads and validates the agent configuration.
func LoadConfig(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Config{
		DataDir:        "/var/lib/camera-agent",
		RecordingsPath: "/media/frigate",
		ProfilesDir:    "/etc/camera/profiles",
		KeyFile:        "/var/lib/camera-agent/secret.key",
		RetentionDays:  45,
		HealthInterval: 30 * time.Second,
		ThumbInterval:  15 * time.Second,
	}
	cfg.Frigate.URL = "http://frigate:5000"
	cfg.Frigate.BrokerURL = "tcp://mosquitto:1883"
	cfg.Frigate.ConfigPath = "/config/config.yml"
	cfg.Frigate.Detector = "openvino"
	cfg.Go2RTCURL = "http://go2rtc:1984"

	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.DeviceID == "" || cfg.TenantID == "" {
		return nil, fmt.Errorf("device_id and tenant_id are required")
	}
	if cfg.Cloud.BrokerURL == "" {
		return nil, fmt.Errorf("cloud.broker_url is required")
	}
	return cfg, nil
}
