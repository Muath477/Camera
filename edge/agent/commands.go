package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Muath477/Camera/contract"
)

// handleCommand runs one command from the cloud and returns its ack.
func (a *Agent) handleCommand(ctx context.Context, cmd contract.Command) contract.CommandAck {
	ack := contract.CommandAck{ID: cmd.ID}
	result, err := a.runCommand(ctx, cmd)
	if err != nil {
		ack.Error = err.Error()
		return ack
	}
	ack.OK = true
	ack.Result = result
	return ack
}

func (a *Agent) runCommand(ctx context.Context, cmd contract.Command) (map[string]any, error) {
	args := argReader(cmd.Args)
	switch cmd.Type {
	case contract.CmdSnapshot:
		camera := args.str("camera")
		img, err := a.frigate.LatestJPEG(ctx, camera, 0)
		if err != nil {
			return nil, err
		}
		if err := putObject(ctx, args.str("upload_url"), "image/jpeg", img); err != nil {
			return nil, err
		}
		sum := sha256.Sum256(img)
		return map[string]any{"sha256": hex.EncodeToString(sum[:]), "bytes": len(img), "taken_at": time.Now().UTC()}, nil

	case contract.CmdExportClip:
		start, err := time.Parse(time.RFC3339, args.str("start"))
		if err != nil {
			return nil, fmt.Errorf("start: %w", err)
		}
		end, err := time.Parse(time.RFC3339, args.str("end"))
		if err != nil {
			return nil, fmt.Errorf("end: %w", err)
		}
		if end.Sub(start) > 30*time.Minute || !end.After(start) {
			return nil, fmt.Errorf("export window must be between 0 and 30 minutes")
		}
		out, err := a.frigate.Export(ctx, args.str("camera"), start, end)
		if err != nil {
			return nil, err
		}
		var res map[string]any
		_ = json.Unmarshal(out, &res)
		return res, nil

	case contract.CmdAddCamera:
		var c Camera
		if err := remarshal(cmd.Args, &c); err != nil {
			return nil, err
		}
		if !ValidCameraName(c.Name) {
			return nil, fmt.Errorf("invalid camera name %q", c.Name)
		}
		if c.ProfileID == "" {
			if p := a.profiles.Match(c.Vendor, c.Model); p != nil {
				c.ProfileID = p.ID
			}
		}
		if err := a.store.Put(c); err != nil {
			return nil, err
		}
		return map[string]any{"profile_id": c.ProfileID}, a.applyFrigateConfig(ctx)

	case contract.CmdRemoveCamera:
		if err := a.store.Delete(args.str("camera")); err != nil {
			return nil, err
		}
		return nil, a.applyFrigateConfig(ctx)

	case contract.CmdPTZ:
		camera, action := args.str("camera"), args.str("action")
		if p := a.profileFor(camera); p != nil && p.HasQuirk("no_relative_move") && action == "move_relative" {
			return nil, fmt.Errorf("camera does not support relative moves")
		}
		// Frigate takes PTZ commands on MQTT: MOVE_LEFT, ZOOM_IN, STOP, preset_<name>.
		tok := a.local.Publish("frigate/"+camera+"/ptz", 0, false, action)
		tok.WaitTimeout(5 * time.Second)
		return nil, tok.Error()

	case contract.CmdUpdateProfiles:
		raw := args.str("yaml")
		if raw == "" {
			return nil, fmt.Errorf("yaml is required")
		}
		tmp := a.cfg.ProfilesDir + "/cloud.yaml.tmp"
		if err := os.WriteFile(tmp, []byte(raw), 0o644); err != nil {
			return nil, err
		}
		if err := os.Rename(tmp, a.cfg.ProfilesDir+"/cloud.yaml"); err != nil {
			return nil, err
		}
		ps, err := LoadProfiles(a.cfg.ProfilesDir)
		if err != nil {
			return nil, err
		}
		a.profiles = ps
		return nil, nil

	case contract.CmdDiscover:
		dctx, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
		found, err := Discover(dctx, args.str("interface_addr"), a.profiles)
		if err != nil {
			return nil, err
		}
		return map[string]any{"cameras": found}, nil

	case contract.CmdThumbs:
		minutes := args.num("minutes")
		if minutes <= 0 || minutes > 60 {
			minutes = 5
		}
		a.thumbsUntil.Store(time.Now().Add(time.Duration(minutes) * time.Minute).Unix())
		return nil, nil

	case contract.CmdRetainEvent:
		keep, _ := cmd.Args["keep"].(bool)
		return nil, a.frigate.Retain(ctx, args.str("event_id"), keep)

	case contract.CmdWebRTCOffer:
		answer, err := a.webrtcAnswer(ctx, args.str("camera"), args.str("sdp"))
		if err != nil {
			return nil, err
		}
		return map[string]any{"sdp": answer}, nil
	}
	return nil, fmt.Errorf("unknown command %q", cmd.Type)
}

// applyFrigateConfig regenerates Frigate's config from the camera store and
// restarts Frigate so it takes effect.
func (a *Agent) applyFrigateConfig(ctx context.Context) error {
	out, err := FrigateConfig(a.cfg, a.store.List(), a.profiles)
	if err != nil {
		return err
	}
	tmp := a.cfg.Frigate.ConfigPath + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, a.cfg.Frigate.ConfigPath); err != nil {
		return err
	}
	return a.frigate.Restart(ctx)
}

// webrtcAnswer forwards a browser's SDP offer to the local go2rtc and
// returns its answer. Media then flows browser <-> device directly, or via
// TURN; the cloud only carries these two small messages.
func (a *Agent) webrtcAnswer(ctx context.Context, camera, offer string) (string, error) {
	if !ValidCameraName(camera) || offer == "" {
		return "", fmt.Errorf("camera and sdp are required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.Go2RTCURL+"/api/webrtc?src="+camera, bytes.NewBufferString(offer))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/sdp")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("go2rtc: %s", resp.Status)
	}
	return string(body), nil
}

func (a *Agent) profileFor(camera string) *Profile {
	for _, c := range a.store.List() {
		if c.Name == camera {
			return a.profiles.Get(c.ProfileID)
		}
	}
	return nil
}

// putObject uploads bytes to a presigned object storage URL.
func putObject(ctx context.Context, url, contentType string, body []byte) error {
	if url == "" {
		return fmt.Errorf("upload_url is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("upload: %s", resp.Status)
	}
	return nil
}

type argReader map[string]any

func (r argReader) str(k string) string {
	s, _ := r[k].(string)
	return s
}

func (r argReader) num(k string) int {
	f, _ := r[k].(float64)
	return int(f)
}

func remarshal(in any, out any) error {
	raw, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}
