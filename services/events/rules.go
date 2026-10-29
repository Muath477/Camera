package events

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Muath477/Camera/services/audit"
	"github.com/Muath477/Camera/services/platform"
)

// Conditions decide when a rule fires.
type Conditions struct {
	Labels    []string `json:"labels,omitempty"`     // empty = any label
	Zones     []string `json:"zones,omitempty"`      // empty = anywhere
	CameraIDs []string `json:"camera_ids,omitempty"` // empty = every camera of the site
	MinScore  float32  `json:"min_score,omitempty"`
	// Schedule limits the rule to local hours, e.g. after closing: from 23 to 6.
	Schedule *struct {
		FromHour int    `json:"from_hour"`
		ToHour   int    `json:"to_hour"`
		Timezone string `json:"timezone"`
	} `json:"schedule,omitempty"`
}

// Rule is an alert rule.
type Rule struct {
	ID         string     `json:"id"`
	SiteID     *string    `json:"site_id"`
	Name       string     `json:"name"`
	Conditions Conditions `json:"conditions"`
	Severity   string     `json:"severity"`
	Channels   []string   `json:"channels"`
	CooldownS  int        `json:"cooldown_s"`
	Enabled    bool       `json:"enabled"`
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// Matches reports whether ev triggers the rule at time now.
func (r Rule) Matches(ev Event, now time.Time) bool {
	c := r.Conditions
	if !r.Enabled {
		return false
	}
	if r.SiteID != nil && *r.SiteID != ev.SiteID {
		return false
	}
	if len(c.Labels) > 0 && !contains(c.Labels, ev.Label) {
		return false
	}
	if len(c.CameraIDs) > 0 && (ev.CameraID == nil || !contains(c.CameraIDs, *ev.CameraID)) {
		return false
	}
	if len(c.Zones) > 0 {
		hit := false
		for _, z := range ev.Zones {
			if contains(c.Zones, z) {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	if c.MinScore > 0 && (ev.Score == nil || *ev.Score < c.MinScore) {
		return false
	}
	if s := c.Schedule; s != nil {
		loc, err := time.LoadLocation(s.Timezone)
		if err != nil {
			loc = time.FixedZone("AST", 3*3600)
		}
		h := now.In(loc).Hour()
		in := h >= s.FromHour && h < s.ToHour
		if s.FromHour > s.ToHour { // wraps midnight
			in = h >= s.FromHour || h < s.ToHour
		}
		if !in {
			return false
		}
	}
	return true
}

func loadRules(ctx context.Context, tx pgx.Tx, siteID string) ([]Rule, error) {
	rows, err := tx.Query(ctx, `SELECT id, site_id::text, name, conditions, severity, channels, cooldown_s, enabled
		FROM alert_rules WHERE enabled AND (site_id IS NULL OR site_id = $1)`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Rule
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.ID, &r.SiteID, &r.Name, &r.Conditions, &r.Severity, &r.Channels, &r.CooldownS, &r.Enabled); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (h *Handler) listRules(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	sites, all := p.SitesWith(platform.PermAlertManage)
	var out []Rule
	err := h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `SELECT id, site_id::text, name, conditions, severity, channels, cooldown_s, enabled
			FROM alert_rules WHERE $1 OR site_id = ANY($2::uuid[]) ORDER BY name`, all, sites)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var x Rule
			if err := rows.Scan(&x.ID, &x.SiteID, &x.Name, &x.Conditions, &x.Severity, &x.Channels, &x.CooldownS, &x.Enabled); err != nil {
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

func (h *Handler) createRule(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	var in Rule
	if err := platform.Decode(r, &in); err != nil || in.Name == "" {
		platform.Fail(w, r, platform.ErrBadInput)
		return
	}
	site := ""
	if in.SiteID != nil {
		site = *in.SiteID
	}
	if !p.Can(platform.PermAlertManage, site) {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	if in.Severity == "" {
		in.Severity = "warning"
	}
	if len(in.Channels) == 0 {
		in.Channels = []string{"push"}
	}
	if in.CooldownS <= 0 {
		in.CooldownS = 300
	}
	var id string
	err := h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		err := tx.QueryRow(r.Context(), `INSERT INTO alert_rules (tenant_id, site_id, name, conditions, severity, channels, cooldown_s)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
			p.TenantID, in.SiteID, in.Name, in.Conditions, in.Severity, in.Channels, in.CooldownS).Scan(&id)
		if err != nil {
			return err
		}
		return audit.Append(r.Context(), tx, p.TenantID, audit.FromRequest(r, "alert_rule.create", "alert_rule", id, nil))
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	platform.JSON(w, http.StatusCreated, map[string]string{"id": id})
}
