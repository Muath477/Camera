// Package devices manages edge devices: claiming, health, and commands.
package devices

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Muath477/Camera/contract"
	"github.com/Muath477/Camera/services/audit"
	"github.com/Muath477/Camera/services/platform"
)

type Handler struct {
	DB *platform.DB
	// Cmd sends commands to devices.
	Cmd Commander
	// EnrollSecret signs one-time enrollment tokens the agent exchanges with
	// the certificate authority for its client certificate.
	EnrollSecret []byte
	Log          *slog.Logger
}

func (h *Handler) Routes(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	mux.Handle("GET /v1/devices", wrap(http.HandlerFunc(h.list)))
	mux.Handle("POST /v1/devices/claim", wrap(http.HandlerFunc(h.claim)))
	mux.Handle("GET /v1/devices/{id}", wrap(http.HandlerFunc(h.get)))
	mux.Handle("POST /v1/devices/{id}/discover", wrap(http.HandlerFunc(h.discover)))
}

// Device is the API view of a device.
type Device struct {
	ID            string          `json:"id"`
	SiteID        *string         `json:"site_id"`
	Serial        string          `json:"serial"`
	HWModel       string          `json:"hw_model"`
	FWVersion     string          `json:"fw_version"`
	Channel       string          `json:"channel"`
	Status        string          `json:"status"`
	LastSeenAt    *time.Time      `json:"last_seen_at"`
	DiskTotal     *int64          `json:"disk_total"`
	DiskUsed      *int64          `json:"disk_used"`
	RetentionDays *float32        `json:"retention_days"`
	Health        json.RawMessage `json:"health,omitempty"`
}

const deviceColumns = `id, site_id::text, serial, coalesce(hw_model,''), coalesce(fw_version,''), channel,
	CASE WHEN last_seen_at > now() - interval '2 minutes' THEN 'online' WHEN status = 'pending' THEN 'pending' ELSE 'offline' END,
	last_seen_at, disk_total, disk_used, retention_days, health`

func scanDevice(row pgx.Row) (Device, error) {
	var d Device
	var health []byte
	err := row.Scan(&d.ID, &d.SiteID, &d.Serial, &d.HWModel, &d.FWVersion, &d.Channel, &d.Status,
		&d.LastSeenAt, &d.DiskTotal, &d.DiskUsed, &d.RetentionDays, &health)
	d.Health = health
	return d, err
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	sites, all := p.SitesWith(platform.PermDeviceManage)
	var out []Device
	err := h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `SELECT `+deviceColumns+` FROM devices
			WHERE $1 OR site_id = ANY($2::uuid[]) ORDER BY serial`, all, sites)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			d, err := scanDevice(rows)
			if err != nil {
				return err
			}
			out = append(out, d)
		}
		return rows.Err()
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	platform.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) load(ctx context.Context, tenantID, id string) (Device, error) {
	var d Device
	err := h.DB.InTenant(ctx, tenantID, func(tx pgx.Tx) (err error) {
		d, err = scanDevice(tx.QueryRow(ctx, `SELECT `+deviceColumns+` FROM devices WHERE id = $1`, id))
		return err
	})
	return d, err
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	d, err := h.load(r.Context(), p.TenantID, r.PathValue("id"))
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	if d.SiteID == nil || !p.Can(platform.PermDeviceManage, *d.SiteID) {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	platform.JSON(w, http.StatusOK, d)
}

// HashClaimCode is how claim codes are stored in the inventory.
func HashClaimCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

// claim binds a shipped device (serial + code printed on its QR label) to a site.
func (h *Handler) claim(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	var in struct {
		Serial    string `json:"serial"`
		ClaimCode string `json:"claim_code"`
		SiteID    string `json:"site_id"`
	}
	if err := platform.Decode(r, &in); err != nil || in.Serial == "" || in.ClaimCode == "" || in.SiteID == "" {
		platform.Fail(w, r, platform.ErrBadInput)
		return
	}
	if !p.Can(platform.PermDeviceManage, in.SiteID) {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}

	var hwModel, codeHash string
	var claimedAt *time.Time
	err := h.DB.Pool.QueryRow(r.Context(), `SELECT hw_model, claim_code_hash, claimed_at FROM device_inventory WHERE serial = $1`, in.Serial).
		Scan(&hwModel, &codeHash, &claimedAt)
	if err != nil || claimedAt != nil || subtle.ConstantTimeCompare([]byte(codeHash), []byte(HashClaimCode(in.ClaimCode))) != 1 {
		// One answer for every failure so serials cannot be probed.
		platform.Fail(w, r, platform.Errorf(http.StatusBadRequest, "serial or claim code is wrong"))
		return
	}

	var deviceID string
	err = h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		ctx := r.Context()
		tag, err := tx.Exec(ctx, `UPDATE device_inventory SET claimed_at = now() WHERE serial = $1 AND claimed_at IS NULL`, in.Serial)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return platform.Errorf(http.StatusConflict, "device already claimed")
		}
		if err := tx.QueryRow(ctx, `INSERT INTO devices (tenant_id, site_id, serial, hw_model) VALUES ($1, $2, $3, $4) RETURNING id`,
			p.TenantID, in.SiteID, in.Serial, hwModel).Scan(&deviceID); err != nil {
			return err
		}
		return audit.Append(ctx, tx, p.TenantID, audit.FromRequest(r, "device.claim", "device", deviceID, map[string]any{"serial": in.Serial}))
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	platform.JSON(w, http.StatusCreated, map[string]any{
		"device_id":        deviceID,
		"tenant_id":        p.TenantID,
		"enrollment_token": h.enrollmentToken(p.TenantID, deviceID, in.Serial),
	})
}

// enrollmentToken is valid for one hour and names the identity the
// certificate will carry (CN = device id, OU = tenant id).
func (h *Handler) enrollmentToken(tenantID, deviceID, serial string) string {
	body, _ := json.Marshal(map[string]any{"t": tenantID, "d": deviceID, "s": serial, "e": time.Now().Add(time.Hour).Unix()})
	mac := hmac.New(sha256.New, h.EnrollSecret)
	mac.Write(body)
	enc := base64.RawURLEncoding
	return enc.EncodeToString(body) + "." + enc.EncodeToString(mac.Sum(nil))
}

func (h *Handler) discover(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	d, err := h.load(r.Context(), p.TenantID, r.PathValue("id"))
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	if d.SiteID == nil || !p.Can(platform.PermCameraManage, *d.SiteID) {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	ack, err := h.Cmd.Send(ctx, p.TenantID, d.ID, contract.CmdDiscover, nil)
	if err != nil {
		platform.Fail(w, r, platform.Errorf(http.StatusGatewayTimeout, err.Error()))
		return
	}
	if !ack.OK {
		platform.Fail(w, r, platform.Errorf(http.StatusBadGateway, ack.Error))
		return
	}
	platform.JSON(w, http.StatusOK, ack.Result)
}

// IngestHealth stores a device's health report and camera states.
func (h *Handler) IngestHealth(m Message) {
	var hl contract.Health
	if err := json.Unmarshal(m.Payload, &hl); err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := h.DB.InTenant(ctx, m.TenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE devices SET last_seen_at = now(), status = 'active', fw_version = $2,
			       disk_total = $3, disk_used = $4, retention_days = $5, health = $6
			WHERE id = $1`, m.DeviceID, hl.AgentVersion, int64(hl.DiskTotal), int64(hl.DiskUsed), hl.RetentionDays, m.Payload)
		if err != nil {
			return err
		}
		for _, c := range hl.Cameras {
			status := "offline"
			if c.Online {
				status = "online"
			}
			if _, err := tx.Exec(ctx, `UPDATE cameras SET status = $3 WHERE device_id = $1 AND name = $2`, m.DeviceID, c.Camera, status); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		h.Log.Warn("health ingest", "device", m.DeviceID, "err", err)
	}
}
