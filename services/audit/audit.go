// Package audit keeps the append-only, hash-chained record of every
// sensitive action: who watched which camera, who exported what, who changed
// a setting. Each row's hash covers the previous row's hash, so removing or
// editing a row breaks the chain.
package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Muath477/Camera/services/platform"
)

const genesis = "0000000000000000000000000000000000000000000000000000000000000000"

// Entry is one audited action.
type Entry struct {
	ActorType  string         `json:"actor_type"` // user | device | system
	ActorID    string         `json:"actor_id"`
	Action     string         `json:"action"`
	TargetType string         `json:"target_type,omitempty"`
	TargetID   string         `json:"target_id,omitempty"`
	IP         string         `json:"ip,omitempty"`
	DeviceInfo string         `json:"device_info,omitempty"`
	Details    map[string]any `json:"details,omitempty"`
}

// FromRequest fills the actor from the caller and request.
func FromRequest(r *http.Request, action, targetType, targetID string, details map[string]any) Entry {
	p := platform.PrincipalFrom(r.Context())
	e := Entry{ActorType: "user", Action: action, TargetType: targetType, TargetID: targetID,
		DeviceInfo: r.UserAgent(), Details: details}
	if p != nil {
		e.ActorID = p.UserID
	}
	if ip := ClientIP(r); ip != "" {
		e.IP = ip
	}
	return e
}

// ClientIP is the caller's address, honouring the proxy header.
func ClientIP(r *http.Request) string {
	if f := r.Header.Get("X-Forwarded-For"); f != "" {
		for i := 0; i < len(f); i++ {
			if f[i] == ',' {
				return f[:i]
			}
		}
		return f
	}
	host := r.RemoteAddr
	for i := len(host) - 1; i >= 0; i-- {
		if host[i] == ':' {
			return host[:i]
		}
	}
	return host
}

func hashEntry(prev string, tenantID string, at time.Time, e Entry) string {
	body, _ := json.Marshal(struct {
		Prev   string `json:"prev"`
		Tenant string `json:"tenant"`
		At     string `json:"at"`
		Entry
	}{prev, tenantID, at.UTC().Format(time.RFC3339Nano), e})
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// Append writes an entry inside the caller's tenant transaction. Writers are
// serialised per tenant so the chain has no forks.
func Append(ctx context.Context, tx pgx.Tx, tenantID string, e Entry) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('audit:' || $1))`, tenantID); err != nil {
		return err
	}
	prev := genesis
	err := tx.QueryRow(ctx, `SELECT hash FROM audit_log WHERE tenant_id = $1 ORDER BY id DESC LIMIT 1`, tenantID).Scan(&prev)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}
	at := time.Now().UTC().Truncate(time.Microsecond)
	hash := hashEntry(prev, tenantID, at, e)
	var ip any
	if e.IP != "" {
		ip = e.IP
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_log (tenant_id, actor_type, actor_id, action, target_type, target_id, ip, device_info, details, at, prev_hash, hash)
		VALUES ($1, $2, $3, $4, nullif($5,''), nullif($6,''), $7, nullif($8,''), $9, $10, $11, $12)`,
		tenantID, e.ActorType, e.ActorID, e.Action, e.TargetType, e.TargetID, ip, e.DeviceInfo, e.Details, at, prev, hash)
	return err
}

// Row is a stored entry.
type Row struct {
	ID       int64     `json:"id"`
	At       time.Time `json:"at"`
	PrevHash string    `json:"prev_hash"`
	Hash     string    `json:"hash"`
	Entry
}

// Verify walks a tenant's chain and returns the id of the first broken row,
// or 0 when the chain is intact.
func Verify(ctx context.Context, tx pgx.Tx, tenantID string) (checked int, brokenAt int64, err error) {
	rows, err := tx.Query(ctx, `
		SELECT id, at, prev_hash, hash, actor_type, actor_id, action, coalesce(target_type,''), coalesce(target_id,''),
		       coalesce(host(ip),''), coalesce(device_info,''), details
		FROM audit_log WHERE tenant_id = $1 ORDER BY id`, tenantID)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	prev := genesis
	for rows.Next() {
		var r Row
		if err := rows.Scan(&r.ID, &r.At, &r.PrevHash, &r.Hash, &r.ActorType, &r.ActorID, &r.Action,
			&r.TargetType, &r.TargetID, &r.IP, &r.DeviceInfo, &r.Details); err != nil {
			return checked, 0, err
		}
		checked++
		if r.PrevHash != prev || hashEntry(prev, tenantID, r.At, r.Entry) != r.Hash {
			return checked, r.ID, nil
		}
		prev = r.Hash
	}
	return checked, 0, rows.Err()
}

// Handler serves the audit trail to owners and auditors.
type Handler struct {
	DB *platform.DB
}

func (h *Handler) Routes(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	mux.Handle("GET /v1/audit", wrap(http.HandlerFunc(h.list)))
	mux.Handle("GET /v1/audit/verify", wrap(http.HandlerFunc(h.verify)))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	if !p.Can(platform.PermAuditView, "") {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	if before <= 0 {
		before = 1 << 62
	}
	var out []Row
	err := h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `
			SELECT id, at, prev_hash, hash, actor_type, actor_id, action, coalesce(target_type,''), coalesce(target_id,''),
			       coalesce(host(ip),''), coalesce(device_info,''), details
			FROM audit_log WHERE id < $1 ORDER BY id DESC LIMIT 100`, before)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var x Row
			if err := rows.Scan(&x.ID, &x.At, &x.PrevHash, &x.Hash, &x.ActorType, &x.ActorID, &x.Action,
				&x.TargetType, &x.TargetID, &x.IP, &x.DeviceInfo, &x.Details); err != nil {
				return err
			}
			out = append(out, x)
		}
		return rows.Err()
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	platform.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) verify(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	if !p.Can(platform.PermAuditView, "") {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	var checked int
	var broken int64
	err := h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) (err error) {
		checked, broken, err = Verify(r.Context(), tx, p.TenantID)
		return err
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	platform.JSON(w, http.StatusOK, map[string]any{"checked": checked, "intact": broken == 0, "broken_at": broken})
}
