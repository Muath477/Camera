// Package compliance runs a site's readiness checks against the obligations
// of its facility category and serves the report to owners and auditors.
package compliance

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Muath477/Camera/services/audit"
	"github.com/Muath477/Camera/services/platform"
)

// Check statuses.
const (
	Pass = "pass"
	Warn = "warn"
	Fail = "fail"
)

// CheckResult is one line of the readiness report.
type CheckResult struct {
	Type     string         `json:"type"`
	TitleAR  string         `json:"title_ar"`
	Status   string         `json:"status"`
	DetailAR string         `json:"detail_ar"`
	Evidence map[string]any `json:"evidence,omitempty"`
}

// Report is a site's readiness at one moment.
type Report struct {
	SiteID    string        `json:"site_id"`
	SiteName  string        `json:"site_name"`
	Category  *Category     `json:"category"`
	Generated time.Time     `json:"generated_at"`
	Score     int           `json:"score"` // 0-100
	Checks    []CheckResult `json:"checks"`
}

// SiteFacts is what the checks read. Gathered in one place so the rules can
// be tested without a database.
type SiteFacts struct {
	SiteName        string
	Category        string
	RetentionNeeded int
	Devices         []DeviceFact
	Cameras         []CameraFact
	SignageAttested *time.Time
	AuditEntries30d int
	Now             time.Time
}

type DeviceFact struct {
	ID            string
	LastSeen      *time.Time
	RetentionDays *float64
	DiskTotal     *int64
}

type CameraFact struct {
	Name   string
	Status string
	Zone   string
}

// Evaluate runs every check over the facts.
func Evaluate(f SiteFacts) []CheckResult {
	cat := CategoryByID(f.Category)
	var out []CheckResult

	// Recording retention on the dedicated disk.
	ret := CheckResult{Type: "retention", TitleAR: "مدة حفظ التسجيلات", Status: Pass}
	if len(f.Devices) == 0 {
		ret.Status, ret.DetailAR = Fail, "لا يوجد جهاز تسجيل مرتبط بالموقع"
	} else {
		min := -1.0
		for _, d := range f.Devices {
			if d.RetentionDays == nil {
				continue
			}
			if min < 0 || *d.RetentionDays < min {
				min = *d.RetentionDays
			}
		}
		switch {
		case min < 0:
			ret.Status, ret.DetailAR = Warn, "لم يُبلّغ الجهاز عن السعة بعد"
		case min < float64(f.RetentionNeeded):
			ret.Status = Fail
			ret.DetailAR = "السعة الحالية تكفي " + strconv.Itoa(int(min)) + " يوماً والمطلوب " + strconv.Itoa(f.RetentionNeeded) + " يوماً"
		default:
			ret.DetailAR = "السعة تكفي " + strconv.Itoa(int(min)) + " يوماً (المطلوب " + strconv.Itoa(f.RetentionNeeded) + ")"
		}
		ret.Evidence = map[string]any{"min_days": min, "required_days": f.RetentionNeeded}
	}
	out = append(out, ret)

	// Devices reporting.
	con := CheckResult{Type: "connectivity", TitleAR: "اتصال أجهزة التسجيل", Status: Pass, DetailAR: "كل الأجهزة متصلة"}
	stale := 0
	for _, d := range f.Devices {
		if d.LastSeen == nil || f.Now.Sub(*d.LastSeen) > 5*time.Minute {
			stale++
		}
	}
	if stale > 0 {
		con.Status, con.DetailAR = Fail, strconv.Itoa(stale)+" جهاز غير متصل الآن"
	}
	out = append(out, con)

	// Cameras running.
	up := CheckResult{Type: "uptime", TitleAR: "تشغيل الكاميرات", Status: Pass}
	online := 0
	var down []string
	for _, c := range f.Cameras {
		if c.Status == "online" {
			online++
		} else {
			down = append(down, c.Name)
		}
	}
	switch {
	case len(f.Cameras) == 0:
		up.Status, up.DetailAR = Fail, "لا توجد كاميرات مضافة"
	case len(down) == 0:
		up.DetailAR = "كل الكاميرات تعمل (" + strconv.Itoa(online) + ")"
	case float64(online)/float64(len(f.Cameras)) >= 0.9:
		up.Status, up.DetailAR = Warn, "كاميرات متوقفة: "+strings.Join(down, "، ")
	default:
		up.Status, up.DetailAR = Fail, "كاميرات متوقفة: "+strings.Join(down, "، ")
	}
	up.Evidence = map[string]any{"online": online, "total": len(f.Cameras)}
	out = append(out, up)

	// Coverage of the zones the category requires.
	cov := CheckResult{Type: "coverage", TitleAR: "تغطية المناطق المطلوبة", Status: Pass}
	if cat != nil {
		have := map[string]bool{}
		for _, c := range f.Cameras {
			have[c.Zone] = true
		}
		var missing []string
		for _, z := range cat.RequiredZones {
			if !have[z] {
				missing = append(missing, ZoneNamesAR[z])
			}
		}
		if len(missing) > 0 {
			cov.Status, cov.DetailAR = Fail, "مناطق بلا كاميرا: "+strings.Join(missing, "، ")
		} else {
			cov.DetailAR = "كل المناطق المطلوبة مغطاة"
		}
	}
	out = append(out, cov)

	// Signage, attested by the owner with a photo.
	sig := CheckResult{Type: "signage", TitleAR: "لوحات التنبيه بوجود كاميرات", Status: Pass}
	switch {
	case f.SignageAttested == nil:
		sig.Status, sig.DetailAR = Warn, "لم يُرفع إثبات وجود اللوحات"
	case f.Now.Sub(*f.SignageAttested) > 365*24*time.Hour:
		sig.Status, sig.DetailAR = Warn, "إثبات اللوحات أقدم من سنة"
	default:
		sig.DetailAR = "مثبتة بتاريخ " + f.SignageAttested.Format("2006-01-02")
	}
	out = append(out, sig)

	// Access log kept by the platform.
	out = append(out, CheckResult{Type: "access_log", TitleAR: "سجل الوصول والمشاهدة", Status: Pass,
		DetailAR: "يُسجَّل تلقائياً، " + strconv.Itoa(f.AuditEntries30d) + " عملية في آخر 30 يوماً"})
	return out
}

// Score weighs failures more than warnings.
func Score(checks []CheckResult) int {
	if len(checks) == 0 {
		return 0
	}
	total := 0.0
	for _, c := range checks {
		switch c.Status {
		case Pass:
			total += 1
		case Warn:
			total += 0.5
		}
	}
	return int(total / float64(len(checks)) * 100)
}

// Handler serves reports, attestations and auditor links.
type Handler struct {
	DB         *platform.DB
	LinkSecret []byte
}

func (h *Handler) Routes(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	mux.Handle("GET /v1/facility-categories", wrap(http.HandlerFunc(h.categories)))
	mux.Handle("GET /v1/sites/{id}/compliance", wrap(http.HandlerFunc(h.report)))
	mux.Handle("POST /v1/sites/{id}/compliance/attest", wrap(http.HandlerFunc(h.attest)))
	mux.Handle("POST /v1/sites/{id}/compliance/share", wrap(http.HandlerFunc(h.share)))
	mux.HandleFunc("GET /public/compliance/{token}", h.public)
}

func (h *Handler) categories(w http.ResponseWriter, r *http.Request) {
	platform.JSON(w, http.StatusOK, map[string]any{"items": Categories(), "zones": ZoneNamesAR})
}

func (h *Handler) report(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	id := r.PathValue("id")
	if !p.Can(platform.PermComplianceView, id) {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	rep, err := h.build(r.Context(), p.TenantID, id, true)
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	platform.JSON(w, http.StatusOK, rep)
}

// Build gathers facts, evaluates and optionally stores the results.
func (h *Handler) build(ctx context.Context, tenantID, siteID string, store bool) (*Report, error) {
	var f SiteFacts
	f.Now = time.Now().UTC()
	err := h.DB.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT name, facility_category, retention_days_required FROM sites WHERE id = $1`, siteID).
			Scan(&f.SiteName, &f.Category, &f.RetentionNeeded); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT id, last_seen_at, retention_days, disk_total FROM devices WHERE site_id = $1`, siteID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var d DeviceFact
			var rd *float32
			if err := rows.Scan(&d.ID, &d.LastSeen, &rd, &d.DiskTotal); err != nil {
				rows.Close()
				return err
			}
			if rd != nil {
				v := float64(*rd)
				d.RetentionDays = &v
			}
			f.Devices = append(f.Devices, d)
		}
		rows.Close()
		rows, err = tx.Query(ctx, `SELECT display_name, status, coalesce(article2_zone,'') FROM cameras WHERE site_id = $1`, siteID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var c CameraFact
			if err := rows.Scan(&c.Name, &c.Status, &c.Zone); err != nil {
				rows.Close()
				return err
			}
			f.Cameras = append(f.Cameras, c)
		}
		rows.Close()
		err = tx.QueryRow(ctx, `SELECT max(checked_at) FROM compliance_checks WHERE site_id = $1 AND check_type = 'signage' AND status = 'pass'`, siteID).
			Scan(&f.SignageAttested)
		if err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE at > now() - interval '30 days'`).Scan(&f.AuditEntries30d); err != nil {
			return err
		}
		if !store {
			return nil
		}
		for _, c := range Evaluate(f) {
			if c.Type == "signage" {
				continue // stored only by attestation
			}
			if _, err := tx.Exec(ctx, `INSERT INTO compliance_checks (tenant_id, site_id, check_type, status, evidence) VALUES ($1, $2, $3, $4, coalesce($5, '{}'::jsonb))`,
				tenantID, siteID, c.Type, c.Status, c.Evidence); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	checks := Evaluate(f)
	return &Report{SiteID: siteID, SiteName: f.SiteName, Category: CategoryByID(f.Category), Generated: f.Now,
		Score: Score(checks), Checks: checks}, nil
}

func (h *Handler) attest(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	id := r.PathValue("id")
	if !p.Can(platform.PermSiteManage, id) {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	var in struct {
		CheckType string         `json:"check_type"`
		Evidence  map[string]any `json:"evidence"`
	}
	if err := platform.Decode(r, &in); err != nil || in.CheckType != "signage" {
		platform.Fail(w, r, platform.ErrBadInput)
		return
	}
	err := h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `INSERT INTO compliance_checks (tenant_id, site_id, check_type, status, evidence) VALUES ($1, $2, 'signage', 'pass', coalesce($3, '{}'::jsonb))`,
			p.TenantID, id, in.Evidence); err != nil {
			return err
		}
		return audit.Append(r.Context(), tx, p.TenantID, audit.FromRequest(r, "compliance.attest", "site", id, in.Evidence))
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type shareClaims struct {
	Tenant string `json:"t"`
	Site   string `json:"s"`
	Exp    int64  `json:"e"`
}

// share issues a read-only link for an outside auditor, valid for 7 days.
func (h *Handler) share(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	id := r.PathValue("id")
	if !p.Can(platform.PermTenantManage, "") {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	exp := time.Now().Add(7 * 24 * time.Hour)
	token := h.sign(shareClaims{p.TenantID, id, exp.Unix()})
	err := h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		return audit.Append(r.Context(), tx, p.TenantID, audit.FromRequest(r, "compliance.share", "site", id,
			map[string]any{"expires_at": exp.UTC().Format(time.RFC3339)}))
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	platform.JSON(w, http.StatusCreated, map[string]any{"token": token, "expires_at": exp.UTC()})
}

func (h *Handler) sign(c shareClaims) string {
	body, _ := json.Marshal(c)
	mac := hmac.New(sha256.New, h.LinkSecret)
	mac.Write(body)
	enc := base64.RawURLEncoding
	return enc.EncodeToString(body) + "." + enc.EncodeToString(mac.Sum(nil))
}

func (h *Handler) parse(token string) (*shareClaims, error) {
	enc := base64.RawURLEncoding
	body64, sig64, ok := strings.Cut(token, ".")
	if !ok {
		return nil, errors.New("malformed")
	}
	body, err1 := enc.DecodeString(body64)
	sig, err2 := enc.DecodeString(sig64)
	if err1 != nil || err2 != nil {
		return nil, errors.New("malformed")
	}
	mac := hmac.New(sha256.New, h.LinkSecret)
	mac.Write(body)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return nil, errors.New("bad signature")
	}
	var c shareClaims
	if err := json.Unmarshal(body, &c); err != nil {
		return nil, err
	}
	if time.Now().Unix() > c.Exp {
		return nil, errors.New("expired")
	}
	return &c, nil
}

func (h *Handler) public(w http.ResponseWriter, r *http.Request) {
	c, err := h.parse(r.PathValue("token"))
	if err != nil {
		platform.Error(w, http.StatusNotFound, "link is invalid or expired")
		return
	}
	rep, err := h.build(r.Context(), c.Tenant, c.Site, false)
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	platform.JSON(w, http.StatusOK, rep)
}
