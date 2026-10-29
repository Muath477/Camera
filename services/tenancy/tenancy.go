// Package tenancy manages tenants, their sites, users and memberships.
package tenancy

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Muath477/Camera/services/audit"
	"github.com/Muath477/Camera/services/compliance"
	"github.com/Muath477/Camera/services/platform"
)

type Handler struct {
	DB *platform.DB
}

func (h *Handler) Routes(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	mux.Handle("POST /v1/tenants", wrap(http.HandlerFunc(h.createTenant)))
	mux.Handle("GET /v1/me", wrap(http.HandlerFunc(h.me)))
	mux.Handle("GET /v1/sites", wrap(http.HandlerFunc(h.listSites)))
	mux.Handle("POST /v1/sites", wrap(http.HandlerFunc(h.createSite)))
	mux.Handle("GET /v1/sites/{id}", wrap(http.HandlerFunc(h.getSite)))
	mux.Handle("PATCH /v1/sites/{id}", wrap(http.HandlerFunc(h.updateSite)))
	mux.Handle("GET /v1/members", wrap(http.HandlerFunc(h.listMembers)))
	mux.Handle("POST /v1/members", wrap(http.HandlerFunc(h.addMember)))
}

// Site is a branch or building of a tenant.
type Site struct {
	ID                    string    `json:"id"`
	Name                  string    `json:"name"`
	FacilityCategory      string    `json:"facility_category"`
	Address               string    `json:"address,omitempty"`
	Lat                   *float64  `json:"lat,omitempty"`
	Lng                   *float64  `json:"lng,omitempty"`
	Timezone              string    `json:"timezone"`
	RetentionDaysRequired int       `json:"retention_days_required"`
	Cameras               int       `json:"cameras"`
	CamerasOnline         int       `json:"cameras_online"`
	CreatedAt             time.Time `json:"created_at"`
}

const siteColumns = `
	s.id, s.name, s.facility_category, coalesce(s.address,''),
	ST_Y(s.geom::geometry), ST_X(s.geom::geometry), s.timezone, s.retention_days_required,
	(SELECT count(*) FROM cameras c WHERE c.site_id = s.id),
	(SELECT count(*) FROM cameras c WHERE c.site_id = s.id AND c.status = 'online'),
	s.created_at`

func scanSite(row pgx.Row) (Site, error) {
	var s Site
	err := row.Scan(&s.ID, &s.Name, &s.FacilityCategory, &s.Address, &s.Lat, &s.Lng, &s.Timezone,
		&s.RetentionDaysRequired, &s.Cameras, &s.CamerasOnline, &s.CreatedAt)
	return s, err
}

// createTenant registers a new customer with its first owner. Platform staff only.
func (h *Handler) createTenant(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	if !p.PlatformAdmin {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	var in struct {
		Name      string `json:"name"`
		CRNumber  string `json:"cr_number"`
		Plan      string `json:"plan"`
		OwnerSub  string `json:"owner_sub"`
		OwnerName string `json:"owner_name"`
		Phone     string `json:"phone"`
		Email     string `json:"email"`
	}
	if err := platform.Decode(r, &in); err != nil || in.Name == "" || in.OwnerSub == "" {
		platform.Fail(w, r, platform.ErrBadInput)
		return
	}
	if in.Plan == "" {
		in.Plan = "trial"
	}
	var tenantID string
	if err := h.DB.Pool.QueryRow(r.Context(), `SELECT gen_random_uuid()::text`).Scan(&tenantID); err != nil {
		platform.Fail(w, r, err)
		return
	}
	err := h.DB.InTenant(r.Context(), tenantID, func(tx pgx.Tx) error {
		ctx := r.Context()
		if _, err := tx.Exec(ctx, `INSERT INTO tenants (id, name, cr_number, plan) VALUES ($1, $2, nullif($3,''), $4)`,
			tenantID, in.Name, in.CRNumber, in.Plan); err != nil {
			return err
		}
		var userID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO users (tenant_id, keycloak_sub, name, phone, email) VALUES ($1, $2, $3, nullif($4,''), nullif($5,''))
			RETURNING id`, tenantID, in.OwnerSub, in.OwnerName, in.Phone, in.Email).Scan(&userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO memberships (tenant_id, user_id, role) VALUES ($1, $2, 'owner')`, tenantID, userID); err != nil {
			return err
		}
		return audit.Append(ctx, tx, tenantID, audit.Entry{ActorType: "user", ActorID: p.Subject, Action: "tenant.create",
			TargetType: "tenant", TargetID: tenantID})
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	platform.JSON(w, http.StatusCreated, map[string]string{"id": tenantID})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	var name, tenantName string
	err := h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(r.Context(), `SELECT u.name, t.name FROM users u JOIN tenants t ON t.id = u.tenant_id WHERE u.id = $1`, p.UserID).
			Scan(&name, &tenantName)
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	platform.JSON(w, http.StatusOK, map[string]any{
		"user_id": p.UserID, "name": name, "tenant_id": p.TenantID, "tenant_name": tenantName, "roles": p.Roles,
	})
}

func (h *Handler) listSites(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	sites, all := p.SitesWith(platform.PermLiveView)
	if !all && len(sites) == 0 {
		sites, all = p.SitesWith(platform.PermComplianceView)
	}
	var out []Site
	err := h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `SELECT `+siteColumns+` FROM sites s
			WHERE $1 OR s.id = ANY($2::uuid[]) ORDER BY s.name`, all, sites)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			s, err := scanSite(rows)
			if err != nil {
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

func (h *Handler) getSite(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	id := r.PathValue("id")
	if !p.Can(platform.PermLiveView, id) && !p.Can(platform.PermComplianceView, id) {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	var s Site
	err := h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) (err error) {
		s, err = scanSite(tx.QueryRow(r.Context(), `SELECT `+siteColumns+` FROM sites s WHERE s.id = $1`, id))
		return err
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	platform.JSON(w, http.StatusOK, s)
}

type siteInput struct {
	Name             string   `json:"name"`
	FacilityCategory string   `json:"facility_category"`
	Address          string   `json:"address"`
	Lat              *float64 `json:"lat"`
	Lng              *float64 `json:"lng"`
	Timezone         string   `json:"timezone"`
}

func (in *siteInput) validate() error {
	if in.Name == "" {
		return platform.Errorf(http.StatusBadRequest, "name is required")
	}
	if compliance.CategoryByID(in.FacilityCategory) == nil {
		return platform.Errorf(http.StatusBadRequest, "unknown facility_category")
	}
	if (in.Lat == nil) != (in.Lng == nil) {
		return platform.Errorf(http.StatusBadRequest, "lat and lng go together")
	}
	if in.Timezone == "" {
		in.Timezone = "Asia/Riyadh"
	}
	return nil
}

func (h *Handler) createSite(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	if !p.Can(platform.PermTenantManage, "") {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	var in siteInput
	if err := platform.Decode(r, &in); err != nil {
		platform.Fail(w, r, platform.ErrBadInput)
		return
	}
	if err := in.validate(); err != nil {
		platform.Fail(w, r, err)
		return
	}
	cat := compliance.CategoryByID(in.FacilityCategory)
	var id string
	err := h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		err := tx.QueryRow(r.Context(), `
			INSERT INTO sites (tenant_id, name, facility_category, address, geom, timezone, retention_days_required)
			VALUES ($1, $2, $3, nullif($4,''), `+pointSQL+`, $7, $8) RETURNING id`,
			p.TenantID, in.Name, in.FacilityCategory, in.Address, in.Lng, in.Lat, in.Timezone, cat.RetentionDays).Scan(&id)
		if err != nil {
			return err
		}
		return audit.Append(r.Context(), tx, p.TenantID, audit.FromRequest(r, "site.create", "site", id, nil))
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	platform.JSON(w, http.StatusCreated, map[string]string{"id": id})
}

// pointSQL builds a geography point from $5 (lng) and $6 (lat), or NULL.
const pointSQL = `CASE WHEN $5::float8 IS NULL THEN NULL ELSE ST_SetSRID(ST_MakePoint($5::float8, $6::float8), 4326)::geography END`

func (h *Handler) updateSite(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	id := r.PathValue("id")
	if !p.Can(platform.PermSiteManage, id) {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	var in siteInput
	if err := platform.Decode(r, &in); err != nil {
		platform.Fail(w, r, platform.ErrBadInput)
		return
	}
	if err := in.validate(); err != nil {
		platform.Fail(w, r, err)
		return
	}
	cat := compliance.CategoryByID(in.FacilityCategory)
	err := h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(r.Context(), `
			UPDATE sites SET name = $2, facility_category = $3, address = nullif($4,''), geom = `+pointSQL+`,
			       timezone = $7, retention_days_required = $8
			WHERE id = $1`, id, in.Name, in.FacilityCategory, in.Address, in.Lng, in.Lat, in.Timezone, cat.RetentionDays)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return audit.Append(r.Context(), tx, p.TenantID, audit.FromRequest(r, "site.update", "site", id, nil))
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type member struct {
	UserID    string     `json:"user_id"`
	Name      string     `json:"name"`
	Phone     string     `json:"phone,omitempty"`
	Email     string     `json:"email,omitempty"`
	SiteID    *string    `json:"site_id,omitempty"`
	Role      string     `json:"role"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

func (h *Handler) listMembers(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	if !p.Can(platform.PermTenantManage, "") {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	var out []member
	err := h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `
			SELECT u.id, u.name, coalesce(u.phone,''), coalesce(u.email,''), m.site_id::text, m.role, m.expires_at
			FROM memberships m JOIN users u ON u.id = m.user_id ORDER BY u.name`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var m member
			if err := rows.Scan(&m.UserID, &m.Name, &m.Phone, &m.Email, &m.SiteID, &m.Role, &m.ExpiresAt); err != nil {
				return err
			}
			out = append(out, m)
		}
		return rows.Err()
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	platform.JSON(w, http.StatusOK, map[string]any{"items": out})
}

// addMember grants a role to a user, creating the user record on first grant.
// Integrators and auditors must carry an expiry.
func (h *Handler) addMember(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	if !p.Can(platform.PermTenantManage, "") {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	var in struct {
		KeycloakSub string     `json:"keycloak_sub"`
		Name        string     `json:"name"`
		Phone       string     `json:"phone"`
		Email       string     `json:"email"`
		SiteID      *string    `json:"site_id"`
		Role        string     `json:"role"`
		ExpiresAt   *time.Time `json:"expires_at"`
	}
	if err := platform.Decode(r, &in); err != nil || in.KeycloakSub == "" || in.Name == "" {
		platform.Fail(w, r, platform.ErrBadInput)
		return
	}
	if _, ok := map[string]bool{"owner": true, "site_manager": true, "operator": true, "viewer": true, "auditor": true, "integrator": true}[in.Role]; !ok {
		platform.Fail(w, r, platform.Errorf(http.StatusBadRequest, "unknown role"))
		return
	}
	if (in.Role == "integrator" || in.Role == "auditor") && in.ExpiresAt == nil {
		platform.Fail(w, r, platform.Errorf(http.StatusBadRequest, "this role needs expires_at"))
		return
	}
	var userID string
	err := h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		ctx := r.Context()
		err := tx.QueryRow(ctx, `
			INSERT INTO users (tenant_id, keycloak_sub, name, phone, email) VALUES ($1, $2, $3, nullif($4,''), nullif($5,''))
			ON CONFLICT (keycloak_sub) DO UPDATE SET name = excluded.name RETURNING id`,
			p.TenantID, in.KeycloakSub, in.Name, in.Phone, in.Email).Scan(&userID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO memberships (tenant_id, user_id, site_id, role, granted_by, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6)`, p.TenantID, userID, in.SiteID, in.Role, p.UserID, in.ExpiresAt)
		if err != nil {
			return err
		}
		return audit.Append(ctx, tx, p.TenantID, audit.FromRequest(r, "member.grant", "user", userID,
			map[string]any{"role": in.Role, "site_id": in.SiteID}))
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	platform.JSON(w, http.StatusCreated, map[string]string{"user_id": userID})
}
