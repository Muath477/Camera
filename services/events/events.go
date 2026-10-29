// Package events ingests detections from devices, stores them, evaluates
// alert rules and streams them to clients.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"

	"github.com/Muath477/Camera/contract"
	"github.com/Muath477/Camera/services/audit"
	"github.com/Muath477/Camera/services/devices"
	"github.com/Muath477/Camera/services/platform"
)

// Event is the API view of a stored event.
type Event struct {
	Time        time.Time  `json:"time"`
	DeviceID    string     `json:"device_id"`
	EdgeID      string     `json:"edge_id"`
	SiteID      string     `json:"site_id"`
	CameraID    *string    `json:"camera_id"`
	CameraName  *string    `json:"camera_name"`
	Kind        string     `json:"kind"`
	Label       string     `json:"label"`
	Score       *float32   `json:"score"`
	Zones       []string   `json:"zones"`
	EndTime     *time.Time `json:"end_time"`
	SnapshotKey *string    `json:"snapshot_key,omitempty"`
	Severity    string     `json:"severity"`
	AckedAt     *time.Time `json:"acked_at,omitempty"`
}

// Notifier delivers alerts for a matched rule.
type Notifier interface {
	Notify(ctx context.Context, tenantID string, rule Rule, ev Event)
}

type Handler struct {
	DB       *platform.DB
	Cmd      devices.Commander
	Notifier Notifier
	Log      *slog.Logger

	hub      hub
	cooldown sync.Map // rule id + camera -> last fire time
}

func (h *Handler) Routes(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	mux.Handle("GET /v1/events", wrap(http.HandlerFunc(h.list)))
	mux.Handle("GET /v1/events/stream", wrap(http.HandlerFunc(h.stream)))
	mux.Handle("POST /v1/events/{device}/{edge}/ack", wrap(http.HandlerFunc(h.ack)))
	mux.Handle("POST /v1/events/{device}/{edge}/retain", wrap(http.HandlerFunc(h.retain)))
	mux.Handle("GET /v1/events/summary", wrap(http.HandlerFunc(h.summary)))
	mux.Handle("GET /v1/alert-rules", wrap(http.HandlerFunc(h.listRules)))
	mux.Handle("POST /v1/alert-rules", wrap(http.HandlerFunc(h.createRule)))
}

// Ingest stores an event from a device and fans it out. Messages may repeat
// (QoS 1 and the device outbox); the primary key makes that harmless, and
// the "end" message completes the row written by "new".
func (h *Handler) Ingest(m devices.Message) {
	var in contract.Event
	if err := json.Unmarshal(m.Payload, &in); err != nil || in.ID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var ev Event
	var inserted bool
	var rules []Rule
	err := h.DB.InTenant(ctx, m.TenantID, func(tx pgx.Tx) error {
		var emb any
		if len(in.Embedding) == 512 {
			emb = pgvector.NewVector(in.Embedding)
		}
		err := tx.QueryRow(ctx, `
			WITH dev AS (SELECT id, site_id FROM devices WHERE id = $1),
			     cam AS (SELECT id, display_name FROM cameras WHERE device_id = $1 AND name = $3)
			INSERT INTO events (time, tenant_id, site_id, device_id, camera_id, edge_id, kind, label, score, zones,
			                    end_time, snapshot_key, clip_key, embedding, severity)
			SELECT $4, $2, dev.site_id, dev.id, (SELECT id FROM cam), $5, $6, $7, $8, $9, $10, nullif($11,''), nullif($12,''), $13, $14
			FROM dev WHERE dev.site_id IS NOT NULL
			ON CONFLICT (tenant_id, device_id, edge_id, time) DO UPDATE SET
			  end_time = coalesce(excluded.end_time, events.end_time),
			  score = greatest(excluded.score, events.score),
			  zones = coalesce(excluded.zones, events.zones),
			  snapshot_key = coalesce(excluded.snapshot_key, events.snapshot_key),
			  clip_key = coalesce(excluded.clip_key, events.clip_key),
			  embedding = coalesce(excluded.embedding, events.embedding)
			RETURNING time, device_id, edge_id, site_id, camera_id, (SELECT display_name FROM cam), kind, label, score,
			          zones, end_time, snapshot_key, severity, acked_at, (xmax = 0)`,
			m.DeviceID, m.TenantID, in.Camera, in.StartTime, in.ID, string(in.Kind), in.Label, in.Score, in.Zones,
			in.EndTime, in.SnapshotKey, in.ClipKey, emb, severityFor(in)).
			Scan(&ev.Time, &ev.DeviceID, &ev.EdgeID, &ev.SiteID, &ev.CameraID, &ev.CameraName, &ev.Kind, &ev.Label,
				&ev.Score, &ev.Zones, &ev.EndTime, &ev.SnapshotKey, &ev.Severity, &ev.AckedAt, &inserted)
		if err != nil {
			return err
		}
		rules, err = loadRules(ctx, tx, ev.SiteID)
		return err
	})
	if err != nil {
		h.Log.Warn("event ingest", "device", m.DeviceID, "event", in.ID, "err", err)
		return
	}
	h.hub.publish(m.TenantID, ev)
	if !inserted {
		return
	}
	for _, rule := range rules {
		if !rule.Matches(ev, time.Now()) || !h.cool(rule, ev) {
			continue
		}
		if h.Notifier != nil {
			go h.Notifier.Notify(context.Background(), m.TenantID, rule, ev)
		}
	}
}

func severityFor(e contract.Event) string {
	switch e.Label {
	case "fire_alarm", "smoke_detector", "gunshot", "scream", "glass":
		return "critical"
	case "retention_short", "camera_offline", "tamper":
		return "warning"
	}
	return "info"
}

func (h *Handler) cool(rule Rule, ev Event) bool {
	key := rule.ID
	if ev.CameraID != nil {
		key += "/" + *ev.CameraID
	}
	now := time.Now()
	if last, ok := h.cooldown.Load(key); ok && now.Sub(last.(time.Time)) < time.Duration(rule.CooldownS)*time.Second {
		return false
	}
	h.cooldown.Store(key, now)
	return true
}

const eventColumns = `e.time, e.device_id, e.edge_id, e.site_id, e.camera_id, c.display_name, e.kind, e.label, e.score,
	e.zones, e.end_time, e.snapshot_key, e.severity, e.acked_at`

func scanEvent(row pgx.Row) (Event, error) {
	var ev Event
	err := row.Scan(&ev.Time, &ev.DeviceID, &ev.EdgeID, &ev.SiteID, &ev.CameraID, &ev.CameraName, &ev.Kind, &ev.Label,
		&ev.Score, &ev.Zones, &ev.EndTime, &ev.SnapshotKey, &ev.Severity, &ev.AckedAt)
	return ev, err
}

// Filter narrows an event query.
type Filter struct {
	Sites    []string
	AllSites bool
	SiteID   string
	CameraID string
	Label    string
	Kind     string
	From     time.Time
	To       time.Time
	Before   time.Time
	Limit    int
}

// ParseFilter reads the common query parameters.
func ParseFilter(r *http.Request, p *platform.Principal) (Filter, error) {
	q := r.URL.Query()
	f := Filter{SiteID: q.Get("site_id"), CameraID: q.Get("camera_id"), Label: q.Get("label"), Kind: q.Get("kind"), Limit: 50}
	f.Sites, f.AllSites = p.SitesWith(platform.PermEventsView)
	var err error
	parse := func(name string, dst *time.Time) {
		if v := q.Get(name); v != "" && err == nil {
			*dst, err = time.Parse(time.RFC3339, v)
		}
	}
	parse("from", &f.From)
	parse("to", &f.To)
	parse("before", &f.Before)
	if err != nil {
		return f, platform.Errorf(http.StatusBadRequest, "times must be RFC 3339")
	}
	if n, _ := strconv.Atoi(q.Get("limit")); n > 0 && n <= 500 {
		f.Limit = n
	}
	if f.To.IsZero() {
		f.To = time.Now().Add(time.Minute)
	}
	if f.From.IsZero() {
		f.From = f.To.Add(-7 * 24 * time.Hour)
	}
	if f.Before.IsZero() {
		f.Before = f.To
	}
	return f, nil
}

// Where renders the filter as a WHERE clause starting at $1.
func (f Filter) Where() (string, []any) {
	return `($1 OR e.site_id = ANY($2::uuid[]))
		AND ($3 = '' OR e.site_id::text = $3) AND ($4 = '' OR e.camera_id::text = $4)
		AND ($5 = '' OR e.label = $5) AND ($6 = '' OR e.kind = $6)
		AND e.time >= $7 AND e.time < $8 AND e.time < $9`,
		[]any{f.AllSites, f.Sites, f.SiteID, f.CameraID, f.Label, f.Kind, f.From, f.To, f.Before}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	f, err := ParseFilter(r, p)
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	where, args := f.Where()
	var out []Event
	err = h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `SELECT `+eventColumns+` FROM events e LEFT JOIN cameras c ON c.id = e.camera_id
			WHERE `+where+fmt.Sprintf(` ORDER BY e.time DESC LIMIT %d`, f.Limit), args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			ev, err := scanEvent(rows)
			if err != nil {
				return err
			}
			out = append(out, ev)
		}
		return rows.Err()
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	platform.JSON(w, http.StatusOK, map[string]any{"items": out})
}

// summary counts events per label and per hour, for the dashboard and the
// colored markers on the playback timeline.
func (h *Handler) summary(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	f, err := ParseFilter(r, p)
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	where, args := f.Where()
	type bucket struct {
		Hour  time.Time `json:"hour"`
		Label string    `json:"label"`
		Count int       `json:"count"`
	}
	var out []bucket
	err = h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `SELECT date_trunc('hour', e.time), e.label, count(*) FROM events e
			WHERE `+where+` GROUP BY 1, 2 ORDER BY 1`, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var b bucket
			if err := rows.Scan(&b.Hour, &b.Label, &b.Count); err != nil {
				return err
			}
			out = append(out, b)
		}
		return rows.Err()
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	platform.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) ack(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	device, edge := r.PathValue("device"), r.PathValue("edge")
	err := h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		var siteID string
		if err := tx.QueryRow(r.Context(), `SELECT site_id FROM devices WHERE id = $1`, device).Scan(&siteID); err != nil {
			return err
		}
		if !p.Can(platform.PermAlertAck, siteID) {
			return platform.ErrForbidden
		}
		tag, err := tx.Exec(r.Context(), `UPDATE events SET acked_by = $3, acked_at = now()
			WHERE device_id = $1 AND edge_id = $2 AND acked_at IS NULL`, device, edge, p.UserID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return audit.Append(r.Context(), tx, p.TenantID, audit.FromRequest(r, "event.ack", "event", device+"/"+edge, nil))
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// retain locks an incident's recording on the device past the normal
// retention period, as the regulation requires for reported incidents.
func (h *Handler) retain(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	device, edge := r.PathValue("device"), r.PathValue("edge")
	var in struct {
		Keep   bool   `json:"keep"`
		Reason string `json:"reason"`
	}
	if err := platform.Decode(r, &in); err != nil || in.Reason == "" {
		platform.Fail(w, r, platform.Errorf(http.StatusBadRequest, "keep and reason are required"))
		return
	}
	var siteID string
	err := h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(r.Context(), `SELECT site_id FROM devices WHERE id = $1`, device).Scan(&siteID)
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	if !p.Can(platform.PermExport, siteID) {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	ack, err := h.Cmd.Send(ctx, p.TenantID, device, contract.CmdRetainEvent, map[string]any{"event_id": edge, "keep": in.Keep})
	if err != nil || !ack.OK {
		platform.Fail(w, r, platform.Errorf(http.StatusBadGateway, "device could not change retention"))
		return
	}
	_ = h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		return audit.Append(r.Context(), tx, p.TenantID, audit.FromRequest(r, "event.retain", "event", device+"/"+edge,
			map[string]any{"keep": in.Keep, "reason": in.Reason}))
	})
	w.WriteHeader(http.StatusNoContent)
}

// stream sends new events as server-sent events. SSE passes through proxies
// that break websockets and reconnects on its own.
func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	flusher, ok := w.(http.Flusher)
	if !ok {
		platform.Error(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := h.hub.subscribe(p.TenantID)
	defer h.hub.unsubscribe(p.TenantID, ch)
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case ev := <-ch:
			if !p.Can(platform.PermEventsView, ev.SiteID) {
				continue
			}
			body, _ := json.Marshal(ev)
			fmt.Fprintf(w, "event: event\ndata: %s\n\n", body)
			flusher.Flush()
		}
	}
}

// hub fans events out to the open streams of a tenant. With several API
// replicas each one receives its share from the broker, so in production
// this is backed by Redis pub/sub.
type hub struct {
	mu   sync.Mutex
	subs map[string]map[chan Event]struct{}
}

func (h *hub) subscribe(tenant string) chan Event {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs == nil {
		h.subs = map[string]map[chan Event]struct{}{}
	}
	if h.subs[tenant] == nil {
		h.subs[tenant] = map[chan Event]struct{}{}
	}
	ch := make(chan Event, 32)
	h.subs[tenant][ch] = struct{}{}
	return ch
}

func (h *hub) unsubscribe(tenant string, ch chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subs[tenant], ch)
}

func (h *hub) publish(tenant string, ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[tenant] {
		select {
		case ch <- ev:
		default: // a slow client drops events rather than blocking ingest
		}
	}
}
