// Package search finds events by description or by similarity across every
// site of a tenant, using the image embeddings the devices compute.
package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"

	"github.com/Muath477/Camera/services/events"
	"github.com/Muath477/Camera/services/platform"
)

// Embedder turns a text query into the same 512-dimension space as the
// device image embeddings (the text tower of the same CLIP model).
type Embedder interface {
	EmbedText(ctx context.Context, text string) ([]float32, error)
}

// HTTPEmbedder calls an in-Kingdom embedding service: POST {"text": "..."}
// returning {"embedding": [...]}.
type HTTPEmbedder struct {
	URL  string
	HTTP *http.Client
}

func (e HTTPEmbedder) EmbedText(ctx context.Context, text string) ([]float32, error) {
	body, _ := json.Marshal(map[string]string{"text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedder: %s", resp.Status)
	}
	var out struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Embedding) != 512 {
		return nil, fmt.Errorf("embedder returned %d dimensions", len(out.Embedding))
	}
	return out.Embedding, nil
}

type Handler struct {
	DB       *platform.DB
	Embedder Embedder // nil disables text search
}

func (h *Handler) Routes(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	mux.Handle("GET /v1/search", wrap(http.HandlerFunc(h.byText)))
	mux.Handle("GET /v1/events/{device}/{edge}/similar", wrap(http.HandlerFunc(h.similar)))
}

type hit struct {
	events.Event
	Distance float64 `json:"distance"`
}

func (h *Handler) query(ctx context.Context, tenantID string, f events.Filter, vec pgvector.Vector) ([]hit, error) {
	where, args := f.Where()
	args = append(args, vec)
	var out []hit
	err := h.DB.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT e.time, e.device_id, e.edge_id, e.site_id, e.camera_id, c.display_name, e.kind, e.label,
			       e.score, e.zones, e.end_time, e.snapshot_key, e.severity, e.acked_at, e.embedding <=> $10
			FROM events e LEFT JOIN cameras c ON c.id = e.camera_id
			WHERE e.embedding IS NOT NULL AND `+where+fmt.Sprintf(` ORDER BY e.embedding <=> $10 LIMIT %d`, f.Limit), args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var x hit
			if err := rows.Scan(&x.Time, &x.DeviceID, &x.EdgeID, &x.SiteID, &x.CameraID, &x.CameraName, &x.Kind, &x.Label,
				&x.Score, &x.Zones, &x.EndTime, &x.SnapshotKey, &x.Severity, &x.AckedAt, &x.Distance); err != nil {
				return err
			}
			out = append(out, x)
		}
		return rows.Err()
	})
	return out, err
}

// byText: GET /v1/search?q=رجل يحمل صندوق&site_id=...
func (h *Handler) byText(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	q := r.URL.Query().Get("q")
	if q == "" {
		platform.Fail(w, r, platform.Errorf(http.StatusBadRequest, "q is required"))
		return
	}
	if h.Embedder == nil {
		platform.Fail(w, r, platform.Errorf(http.StatusNotImplemented, "text search is not enabled"))
		return
	}
	f, err := events.ParseFilter(r, p)
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	if r.URL.Query().Get("limit") == "" {
		f.Limit = 24
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	vec, err := h.Embedder.EmbedText(ctx, q)
	if err != nil {
		platform.Fail(w, r, platform.Errorf(http.StatusBadGateway, "embedding failed"))
		return
	}
	out, err := h.query(r.Context(), p.TenantID, f, pgvector.NewVector(vec))
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	platform.JSON(w, http.StatusOK, map[string]any{"items": out})
}

// similar finds events that look like a given one.
func (h *Handler) similar(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	f, err := events.ParseFilter(r, p)
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	if r.URL.Query().Get("limit") == "" {
		f.Limit = 24
	}
	if r.URL.Query().Get("from") == "" {
		f.From = f.To.AddDate(0, 0, -30)
	}
	var vec pgvector.Vector
	err = h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(r.Context(), `SELECT embedding FROM events WHERE device_id = $1 AND edge_id = $2 AND embedding IS NOT NULL LIMIT 1`,
			r.PathValue("device"), r.PathValue("edge")).Scan(&vec)
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	out, err := h.query(r.Context(), p.TenantID, f, vec)
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	platform.JSON(w, http.StatusOK, map[string]any{"items": out})
}
