package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/Muath477/Camera/contract"
)

// Frigate is a small client for the local Frigate HTTP API. Frigate is not
// exposed on the network; only the agent talks to it.
type Frigate struct {
	base string
	http *http.Client
}

func NewFrigate(base string) *Frigate {
	return &Frigate{base: base, http: &http.Client{Timeout: 15 * time.Second}}
}

func (f *Frigate) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.base+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("frigate %s: %s", path, resp.Status)
	}
	return body, nil
}

func (f *Frigate) post(ctx context.Context, path string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.base+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("frigate %s: %s", path, resp.Status)
	}
	return out, nil
}

// LatestJPEG returns the current frame of a camera at full resolution.
func (f *Frigate) LatestJPEG(ctx context.Context, camera string, height int) ([]byte, error) {
	path := "/api/" + camera + "/latest.jpg"
	if height > 0 {
		path += fmt.Sprintf("?h=%d", height)
	}
	return f.get(ctx, path)
}

// EventSnapshot returns the best snapshot of a tracked object.
func (f *Frigate) EventSnapshot(ctx context.Context, eventID string) ([]byte, error) {
	return f.get(ctx, "/api/events/"+eventID+"/snapshot.jpg?bbox=1&timestamp=1")
}

// Export asks Frigate to cut an MP4 from the local recordings.
func (f *Frigate) Export(ctx context.Context, camera string, start, end time.Time) ([]byte, error) {
	path := fmt.Sprintf("/api/export/%s/start/%d/end/%d", camera, start.Unix(), end.Unix())
	return f.post(ctx, path, []byte(`{"playback":"realtime"}`))
}

// Restart reloads Frigate after its configuration file changed.
func (f *Frigate) Restart(ctx context.Context) error {
	_, err := f.post(ctx, "/api/restart", nil)
	return err
}

// FrigateStats is the subset of /api/stats the agent reports.
type FrigateStats struct {
	Cameras map[string]struct {
		CameraFPS  float64 `json:"camera_fps"`
		ProcessFPS float64 `json:"process_fps"`
		SkippedFPS float64 `json:"skipped_fps"`
	} `json:"cameras"`
	Service struct {
		Version string `json:"version"`
		Storage map[string]struct {
			Total float64 `json:"total"` // MB
			Used  float64 `json:"used"`
		} `json:"storage"`
	} `json:"service"`
}

func (f *Frigate) Stats(ctx context.Context) (*FrigateStats, error) {
	raw, err := f.get(ctx, "/api/stats")
	if err != nil {
		return nil, err
	}
	var s FrigateStats
	return &s, json.Unmarshal(raw, &s)
}

// RecordingUsage is the recorded size and measured bandwidth per camera,
// the real bitrate behind the retention estimate.
type RecordingUsage map[string]struct {
	Usage     float64 `json:"usage"`     // MB
	Bandwidth float64 `json:"bandwidth"` // MB per hour
}

func (f *Frigate) RecordingUsage(ctx context.Context) (RecordingUsage, error) {
	raw, err := f.get(ctx, "/api/recordings/storage")
	if err != nil {
		return nil, err
	}
	var u RecordingUsage
	return u, json.Unmarshal(raw, &u)
}

// frigateEvent is the message Frigate publishes on frigate/events.
type frigateEvent struct {
	Type   string `json:"type"` // new | update | end
	Before struct {
		ID string `json:"id"`
	} `json:"before"`
	After struct {
		ID            string   `json:"id"`
		Camera        string   `json:"camera"`
		Label         string   `json:"label"`
		SubLabel      any      `json:"sub_label"`
		TopScore      float64  `json:"top_score"`
		Score         float64  `json:"score"`
		StartTime     float64  `json:"start_time"`
		EndTime       *float64 `json:"end_time"`
		EnteredZones  []string `json:"entered_zones"`
		HasSnapshot   bool     `json:"has_snapshot"`
		HasClip       bool     `json:"has_clip"`
		FalsePositive bool     `json:"false_positive"`
	} `json:"after"`
}

// audioLabels are the labels Frigate's audio detector raises.
var audioLabels = map[string]bool{
	"scream": true, "yell": true, "glass": true, "smoke_detector": true,
	"fire_alarm": true, "siren": true, "gunshot": true, "speech": true, "bark": true,
}

// MapFrigateEvent turns a Frigate event message into a platform event.
// Only "new" and "end" are forwarded: updates would flood the uplink, and
// the cloud only needs to know an object appeared and when it left.
func MapFrigateEvent(payload []byte) (*contract.Event, bool, error) {
	var fe frigateEvent
	if err := json.Unmarshal(payload, &fe); err != nil {
		return nil, false, err
	}
	if fe.Type != "new" && fe.Type != "end" {
		return nil, false, nil
	}
	a := fe.After
	if a.FalsePositive {
		return nil, false, nil
	}
	ev := &contract.Event{
		ID:        a.ID,
		Kind:      contract.EventObject,
		Camera:    a.Camera,
		Label:     a.Label,
		Score:     math.Max(a.TopScore, a.Score),
		Zones:     a.EnteredZones,
		StartTime: unixFloat(a.StartTime),
	}
	if audioLabels[a.Label] {
		ev.Kind = contract.EventAudio
	}
	if a.EndTime != nil {
		t := unixFloat(*a.EndTime)
		ev.EndTime = &t
	}
	return ev, a.HasSnapshot && fe.Type == "end", nil
}

func unixFloat(f float64) time.Time {
	sec, frac := math.Modf(f)
	return time.Unix(int64(sec), int64(frac*1e9)).UTC()
}
