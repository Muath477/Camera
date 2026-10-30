// Package media handles snapshots, clip exports and device uploads. Bytes go
// straight between devices, object storage and browsers through presigned
// URLs; the services only sign and record.
package media

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"

	"github.com/Muath477/Camera/contract"
	"github.com/Muath477/Camera/services/audit"
	"github.com/Muath477/Camera/services/cameras"
	"github.com/Muath477/Camera/services/devices"
	"github.com/Muath477/Camera/services/platform"
)

// Store is the slice of object storage the module needs.
type Store interface {
	PresignPut(ctx context.Context, key string, ttl time.Duration) (string, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// S3 implements Store on any S3-compatible bucket inside the Kingdom.
// Presign, when set, is a client pointed at the public address of the same
// bucket: devices and browsers follow presigned links from outside the
// network, so the signature has to carry the host they will use.
type S3 struct {
	Client  *minio.Client
	Presign *minio.Client
	Bucket  string
}

func (s S3) signer() *minio.Client {
	if s.Presign != nil {
		return s.Presign
	}
	return s.Client
}

func (s S3) PresignPut(ctx context.Context, key string, ttl time.Duration) (string, error) {
	u, err := s.signer().PresignedPutObject(ctx, s.Bucket, key, ttl)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (s S3) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	u, err := s.signer().PresignedGetObject(ctx, s.Bucket, key, ttl, url.Values{})
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

type Handler struct {
	DB      *platform.DB
	Cmd     devices.Commander
	Store   Store
	SignKey ed25519.PrivateKey
	// TrustProxyIdentity accepts the device's client certificate from the
	// X-Client-Cert header (DER, base64) that the mTLS-terminating gateway
	// sets after verifying it. Only enable behind that gateway, and only when
	// it overwrites the header on every request.
	TrustProxyIdentity bool
}

func (h *Handler) Routes(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	mux.Handle("POST /v1/cameras/{id}/snapshots", wrap(http.HandlerFunc(h.snapshot)))
	mux.Handle("GET /v1/snapshots", wrap(http.HandlerFunc(h.listSnapshots)))
	mux.Handle("GET /v1/snapshots/{id}/verify", wrap(http.HandlerFunc(h.verify)))
	mux.Handle("POST /v1/cameras/{id}/exports", wrap(http.HandlerFunc(h.export)))
	mux.Handle("GET /v1/media", wrap(http.HandlerFunc(h.mediaURL)))
	mux.HandleFunc("POST /device/v1/uploads", h.deviceUpload)
}

// SignedFacts is what a snapshot signature covers.
type SignedFacts struct {
	Tenant      string    `json:"tenant"`
	Site        string    `json:"site"`
	Camera      string    `json:"camera"`
	TakenAt     time.Time `json:"taken_at"`
	SHA256      string    `json:"sha256"`
	RequestedBy string    `json:"requested_by"`
}

func (f SignedFacts) bytes() []byte {
	b, _ := json.Marshal(f)
	return b
}

// Sign returns the base64 Ed25519 signature of the facts.
func Sign(key ed25519.PrivateKey, f SignedFacts) string {
	return base64.StdEncoding.EncodeToString(ed25519.Sign(key, f.bytes()))
}

// VerifySignature checks a signature with the platform's public key.
func VerifySignature(pub ed25519.PublicKey, f SignedFacts, sig string) bool {
	raw, err := base64.StdEncoding.DecodeString(sig)
	return err == nil && ed25519.Verify(pub, f.bytes(), raw)
}

// snapshot takes a full-resolution frame on the device, stores it and signs
// its hash, producing evidence that can be handed to an authority.
func (h *Handler) snapshot(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	cam, err := cameras.Load(r.Context(), h.DB, p.TenantID, r.PathValue("id"))
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	if !p.Can(platform.PermSnapshotCreate, cam.SiteID) {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	key := fmt.Sprintf("%s/snapshots/%s/%s/%s.jpg", p.TenantID, cam.SiteID, cam.ID, randID())
	put, err := h.Store.PresignPut(r.Context(), key, 2*time.Minute)
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	ack, err := h.Cmd.Send(ctx, p.TenantID, cam.DeviceID, contract.CmdSnapshot, map[string]any{"camera": cam.Name, "upload_url": put})
	if err != nil || !ack.OK {
		platform.Fail(w, r, platform.Errorf(http.StatusBadGateway, "device could not take the snapshot"))
		return
	}
	sum, _ := ack.Result["sha256"].(string)
	takenAt, _ := time.Parse(time.RFC3339Nano, fmt.Sprint(ack.Result["taken_at"]))
	if takenAt.IsZero() {
		takenAt = time.Now().UTC()
	}
	// Postgres keeps microseconds; sign what will be stored.
	facts := SignedFacts{Tenant: p.TenantID, Site: cam.SiteID, Camera: cam.ID, TakenAt: takenAt.UTC().Truncate(time.Microsecond), SHA256: sum, RequestedBy: p.UserID}
	sig := Sign(h.SignKey, facts)

	var id string
	err = h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		err := tx.QueryRow(r.Context(), `INSERT INTO snapshots (tenant_id, camera_id, taken_at, requested_by, s3_key, sha256, signature, source, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'live', 'signed') RETURNING id`,
			p.TenantID, cam.ID, facts.TakenAt, p.UserID, key, sum, sig).Scan(&id)
		if err != nil {
			return err
		}
		return audit.Append(r.Context(), tx, p.TenantID, audit.FromRequest(r, "snapshot.create", "snapshot", id,
			map[string]any{"camera": cam.ID, "sha256": sum}))
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	get, _ := h.Store.PresignGet(r.Context(), key, 10*time.Minute)
	platform.JSON(w, http.StatusCreated, map[string]any{"id": id, "sha256": sum, "signature": sig, "taken_at": facts.TakenAt, "url": get})
}

type snapshotRow struct {
	ID          string    `json:"id"`
	CameraID    string    `json:"camera_id"`
	CameraName  string    `json:"camera_name"`
	SiteID      string    `json:"site_id"`
	TakenAt     time.Time `json:"taken_at"`
	RequestedBy *string   `json:"requested_by"`
	SHA256      *string   `json:"sha256"`
	Signature   *string   `json:"signature"`
	Source      string    `json:"source"`
	key         string
}

func (h *Handler) loadSnapshot(ctx context.Context, tenantID, id string) (snapshotRow, error) {
	var s snapshotRow
	err := h.DB.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT s.id, s.camera_id, c.display_name, c.site_id, s.taken_at, s.requested_by::text, s.sha256, s.signature, s.source, s.s3_key
			FROM snapshots s JOIN cameras c ON c.id = s.camera_id WHERE s.id = $1`, id).
			Scan(&s.ID, &s.CameraID, &s.CameraName, &s.SiteID, &s.TakenAt, &s.RequestedBy, &s.SHA256, &s.Signature, &s.Source, &s.key)
	})
	return s, err
}

func (h *Handler) listSnapshots(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	sites, all := p.SitesWith(platform.PermSnapshotCreate)
	site := r.URL.Query().Get("site_id")
	var out []snapshotRow
	err := h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `SELECT s.id, s.camera_id, c.display_name, c.site_id, s.taken_at, s.requested_by::text, s.sha256, s.signature, s.source, s.s3_key
			FROM snapshots s JOIN cameras c ON c.id = s.camera_id
			WHERE ($1 OR c.site_id = ANY($2::uuid[])) AND ($3 = '' OR c.site_id::text = $3)
			ORDER BY s.taken_at DESC LIMIT 100`, all, sites, site)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var s snapshotRow
			if err := rows.Scan(&s.ID, &s.CameraID, &s.CameraName, &s.SiteID, &s.TakenAt, &s.RequestedBy, &s.SHA256, &s.Signature, &s.Source, &s.key); err != nil {
				return err
			}
			out = append(out, s)
		}
		return rows.Err()
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	platform.JSON(w, http.StatusOK, map[string]any{"items": out})
}

// verify recomputes the signature check for a stored snapshot.
func (h *Handler) verify(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	s, err := h.loadSnapshot(r.Context(), p.TenantID, r.PathValue("id"))
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	if !p.Can(platform.PermSnapshotCreate, s.SiteID) && !p.Can(platform.PermComplianceView, s.SiteID) {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	ok := false
	if s.SHA256 != nil && s.Signature != nil && s.RequestedBy != nil {
		facts := SignedFacts{Tenant: p.TenantID, Site: s.SiteID, Camera: s.CameraID, TakenAt: s.TakenAt.UTC(), SHA256: *s.SHA256, RequestedBy: *s.RequestedBy}
		ok = VerifySignature(h.SignKey.Public().(ed25519.PublicKey), facts, *s.Signature)
	}
	platform.JSON(w, http.StatusOK, map[string]any{"valid": ok, "snapshot": s,
		"public_key": base64.StdEncoding.EncodeToString(h.SignKey.Public().(ed25519.PublicKey))})
}

// export cuts a clip from the device's local recording. Every export is
// audited because the regulation forbids moving recordings without permission.
func (h *Handler) export(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	cam, err := cameras.Load(r.Context(), h.DB, p.TenantID, r.PathValue("id"))
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	if !p.Can(platform.PermExport, cam.SiteID) {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	var in struct {
		Start  time.Time `json:"start"`
		End    time.Time `json:"end"`
		Reason string    `json:"reason"`
	}
	if err := platform.Decode(r, &in); err != nil || in.Reason == "" || !in.End.After(in.Start) {
		platform.Fail(w, r, platform.Errorf(http.StatusBadRequest, "start, end and reason are required"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	ack, err := h.Cmd.Send(ctx, p.TenantID, cam.DeviceID, contract.CmdExportClip, map[string]any{
		"camera": cam.Name, "start": in.Start.UTC().Format(time.RFC3339), "end": in.End.UTC().Format(time.RFC3339)})
	if err != nil || !ack.OK {
		msg := "device could not export"
		if ack != nil && ack.Error != "" {
			msg = ack.Error
		}
		platform.Fail(w, r, platform.Errorf(http.StatusBadGateway, msg))
		return
	}
	_ = h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		return audit.Append(r.Context(), tx, p.TenantID, audit.FromRequest(r, "recording.export", "camera", cam.ID,
			map[string]any{"start": in.Start, "end": in.End, "reason": in.Reason}))
	})
	platform.JSON(w, http.StatusAccepted, ack.Result)
}

// mediaURL returns a short-lived download link for a key the caller's tenant owns.
func (h *Handler) mediaURL(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	key := r.URL.Query().Get("key")
	parts := strings.SplitN(key, "/", 4)
	if len(parts) < 4 || parts[0] != p.TenantID || !p.Can(platform.PermEventsView, parts[2]) {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	u, err := h.Store.PresignGet(r.Context(), key, 5*time.Minute)
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	http.Redirect(w, r, u, http.StatusFound)
}

// DeviceIdentity reads the device and tenant from the client certificate
// (CN = device id, OU = tenant id), either from the TLS connection or, behind
// the gateway, from the certificate it forwards.
func (h *Handler) DeviceIdentity(r *http.Request) (deviceID, tenantID string, ok bool) {
	var cert *x509.Certificate
	switch {
	case r.TLS != nil && len(r.TLS.PeerCertificates) > 0:
		cert = r.TLS.PeerCertificates[0]
	case h.TrustProxyIdentity:
		der, err := base64.StdEncoding.DecodeString(r.Header.Get("X-Client-Cert"))
		if err != nil || len(der) == 0 {
			return "", "", false
		}
		if cert, err = x509.ParseCertificate(der); err != nil {
			return "", "", false
		}
	}
	if cert == nil || cert.Subject.CommonName == "" || len(cert.Subject.OrganizationalUnit) == 0 {
		return "", "", false
	}
	return cert.Subject.CommonName, cert.Subject.OrganizationalUnit[0], true
}

// deviceUpload gives a device a presigned URL for an event snapshot or clip.
func (h *Handler) deviceUpload(w http.ResponseWriter, r *http.Request) {
	deviceID, tenantID, ok := h.DeviceIdentity(r)
	if !ok {
		platform.Error(w, http.StatusUnauthorized, "client certificate required")
		return
	}
	var in struct {
		Kind        string `json:"kind"`
		Camera      string `json:"camera"`
		ContentType string `json:"content_type"`
		Bytes       int    `json:"bytes"`
	}
	if err := platform.Decode(r, &in); err != nil || (in.Kind != "snapshot" && in.Kind != "clip") || in.Bytes > 200<<20 {
		platform.Fail(w, r, platform.ErrBadInput)
		return
	}
	var siteID string
	err := h.DB.InTenant(r.Context(), tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(r.Context(), `SELECT site_id FROM devices WHERE id = $1 AND site_id IS NOT NULL`, deviceID).Scan(&siteID)
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	ext := ".jpg"
	if in.Kind == "clip" {
		ext = ".mp4"
	}
	key := fmt.Sprintf("%s/events/%s/%s/%s/%s%s", tenantID, siteID, deviceID, time.Now().UTC().Format("2006/01/02"), randID(), ext)
	u, err := h.Store.PresignPut(r.Context(), key, 5*time.Minute)
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	platform.JSON(w, http.StatusOK, map[string]string{"key": key, "url": u})
}

func randID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
