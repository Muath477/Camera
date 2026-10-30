package events

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Muath477/Camera/services/platform"
)

// Dispatcher sends alerts to the rule's channels. Webhooks are delivered
// here; push, SMS and WhatsApp go through Provider so the vendor can change
// without touching the rules.
type Dispatcher struct {
	DB       *platform.DB
	Provider Provider
	HTTP     *http.Client
	Log      *slog.Logger
}

// Provider sends a short Arabic message on one channel to the tenant's
// users who want that channel at that severity.
type Provider interface {
	Send(ctx context.Context, tenantID, severity, channel, title, body string) error
}

// LogProvider records messages instead of sending them (development).
type LogProvider struct{ Log *slog.Logger }

func (p LogProvider) Send(_ context.Context, tenantID, severity, channel, title, body string) error {
	p.Log.Info("notification", "tenant", tenantID, "severity", severity, "channel", channel, "title", title, "body", body)
	return nil
}

var labelAR = map[string]string{
	"person": "شخص", "car": "سيارة", "motorcycle": "دراجة نارية", "bicycle": "دراجة",
	"glass": "كسر زجاج", "scream": "صراخ", "yell": "صياح", "fire_alarm": "إنذار حريق",
	"smoke_detector": "كاشف دخان", "siren": "صفارة", "gunshot": "طلق ناري",
	"retention_short": "مدة الحفظ أقل من المطلوب", "camera_offline": "كاميرا متوقفة",
}

// LabelAR returns the Arabic name of a label.
func LabelAR(l string) string {
	if v, ok := labelAR[l]; ok {
		return v
	}
	return l
}

func (d *Dispatcher) Notify(ctx context.Context, tenantID string, rule Rule, ev Event) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cam := ""
	if ev.CameraName != nil {
		cam = *ev.CameraName
	}
	title := rule.Name
	body := LabelAR(ev.Label) + " — " + cam + " — " + ev.Time.In(time.FixedZone("AST", 3*3600)).Format("15:04:05")
	for _, ch := range rule.Channels {
		if ch == "webhook" {
			d.webhooks(ctx, tenantID, rule, ev)
			continue
		}
		if err := d.Provider.Send(ctx, tenantID, rule.Severity, ch, title, body); err != nil {
			d.Log.Warn("notify", "channel", ch, "err", err)
		}
	}
}

// webhooks posts the event to each active endpoint, signed with
// HMAC-SHA256 over "timestamp.body", retrying with backoff.
func (d *Dispatcher) webhooks(ctx context.Context, tenantID string, rule Rule, ev Event) {
	type hook struct{ id, url, secret string }
	var hooks []hook
	err := d.DB.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, url, secret FROM webhooks WHERE status = 'active' AND 'event.created' = ANY(events)`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var h hook
			if err := rows.Scan(&h.id, &h.url, &h.secret); err != nil {
				return err
			}
			hooks = append(hooks, h)
		}
		return rows.Err()
	})
	if err != nil {
		d.Log.Warn("load webhooks", "err", err)
		return
	}
	body, _ := json.Marshal(map[string]any{"type": "event.created", "rule": rule.Name, "severity": rule.Severity, "event": ev})
	for _, h := range hooks {
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		mac := hmac.New(sha256.New, []byte(h.secret))
		mac.Write([]byte(ts + "."))
		mac.Write(body)
		sig := hex.EncodeToString(mac.Sum(nil))
		for attempt, wait := 0, time.Second; attempt < 4; attempt, wait = attempt+1, wait*4 {
			req, _ := http.NewRequestWithContext(ctx, http.MethodPost, h.url, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Camera-Timestamp", ts)
			req.Header.Set("X-Camera-Signature", "sha256="+sig)
			resp, err := d.HTTP.Do(req)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode < 300 {
					break
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}
	}
}
