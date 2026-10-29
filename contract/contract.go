// Package contract holds the messages exchanged between edge devices and the
// cloud over MQTT. Both sides import it so the wire format has one source.
package contract

import (
	"fmt"
	"strings"
	"time"
)

// Topic kinds published under t/{tenant}/d/{device}/...
const (
	KindEvents = "events"
	KindHealth = "health"
	KindThumbs = "thumbs"
	KindCmd    = "cmd"
	KindCmdAck = "cmd/ack"
)

// Topic builds the MQTT topic for a device.
func Topic(tenantID, deviceID, kind string) string {
	return fmt.Sprintf("t/%s/d/%s/%s", tenantID, deviceID, kind)
}

// ParseTopic splits a device topic into its tenant, device and kind parts.
func ParseTopic(topic string) (tenantID, deviceID, kind string, err error) {
	parts := strings.SplitN(topic, "/", 5)
	if len(parts) < 5 || parts[0] != "t" || parts[2] != "d" {
		return "", "", "", fmt.Errorf("unexpected topic %q", topic)
	}
	return parts[1], parts[3], parts[4], nil
}

// EventKind groups detections by source.
type EventKind string

const (
	EventObject EventKind = "object"
	EventAudio  EventKind = "audio"
	EventSystem EventKind = "system"
)

// Event is one detection or system event raised at the edge.
type Event struct {
	ID          string     `json:"id"`
	Kind        EventKind  `json:"kind"`
	Camera      string     `json:"camera"`
	Label       string     `json:"label"`
	Score       float64    `json:"score"`
	Zones       []string   `json:"zones,omitempty"`
	StartTime   time.Time  `json:"start_time"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	SnapshotKey string     `json:"snapshot_key,omitempty"`
	ClipKey     string     `json:"clip_key,omitempty"`
	Embedding   []float32  `json:"embedding,omitempty"`
}

// CameraHealth is the per-camera part of a health report.
type CameraHealth struct {
	Camera        string  `json:"camera"`
	Online        bool    `json:"online"`
	FPS           float64 `json:"fps"`
	SkippedFPS    float64 `json:"skipped_fps"`
	BitrateKbps   float64 `json:"bitrate_kbps"`
	RetainedHours float64 `json:"retained_hours"`
}

// Health is published by every device on a fixed interval.
type Health struct {
	At            time.Time      `json:"at"`
	AgentVersion  string         `json:"agent_version"`
	FrigateVer    string         `json:"frigate_version,omitempty"`
	CPUTempC      float64        `json:"cpu_temp_c"`
	DiskTotal     uint64         `json:"disk_total"`
	DiskUsed      uint64         `json:"disk_used"`
	RetentionDays float64        `json:"retention_days"`
	QueueDepth    int            `json:"queue_depth"`
	Cameras       []CameraHealth `json:"cameras"`
}

// Thumb is a small rolling preview used by the grid and the map.
type Thumb struct {
	Camera string    `json:"camera"`
	At     time.Time `json:"at"`
	JPEG   []byte    `json:"jpeg"`
}

// Command types sent from the cloud to a device.
const (
	CmdSnapshot       = "snapshot"
	CmdAddCamera      = "add_camera"
	CmdRemoveCamera   = "remove_camera"
	CmdExportClip     = "export_clip"
	CmdPTZ            = "ptz"
	CmdUpdateProfiles = "update_profiles"
	CmdDiscover       = "discover"
	CmdThumbs         = "thumbs"       // start or stop rolling thumbnails
	CmdWebRTCOffer    = "webrtc_offer" // SDP offer in, SDP answer out
)

// Command is a request from the cloud. Args depend on Type.
type Command struct {
	ID   string         `json:"id"`
	Type string         `json:"type"`
	Args map[string]any `json:"args,omitempty"`
}

// CommandAck reports the outcome of a command.
type CommandAck struct {
	ID     string         `json:"id"`
	OK     bool           `json:"ok"`
	Error  string         `json:"error,omitempty"`
	Result map[string]any `json:"result,omitempty"`
}
