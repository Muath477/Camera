package notify

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Muath477/Camera/services/platform"
)

// Handler exposes the caller's own delivery settings.
type Handler struct {
	Service *Service
}

func (h *Handler) Routes(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	mux.Handle("GET /v1/me/notifications", wrap(http.HandlerFunc(h.get)))
	mux.Handle("PUT /v1/me/notifications", wrap(http.HandlerFunc(h.put)))
	mux.Handle("POST /v1/me/notifications/test", wrap(http.HandlerFunc(h.test)))
	mux.Handle("POST /v1/me/push-subscriptions", wrap(http.HandlerFunc(h.subscribe)))
	mux.Handle("DELETE /v1/me/push-subscriptions", wrap(http.HandlerFunc(h.unsubscribe)))
}

type prefs struct {
	CriticalChannels []string `json:"critical_channels"`
	WarningChannels  []string `json:"warning_channels"`
	QuietFrom        *string  `json:"quiet_from"` // "22:00"
	QuietTo          *string  `json:"quiet_to"`
}

var channels = map[string]bool{"push": true, "sms": true, "whatsapp": true}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	out := prefs{CriticalChannels: []string{"push", "sms"}, WarningChannels: []string{"push"}}
	var subs int
	var phone *string
	err := h.Service.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		var from, to *time.Time
		err := tx.QueryRow(r.Context(), `SELECT critical_channels, warning_channels, quiet_from, quiet_to FROM notification_prefs WHERE user_id = $1`, p.UserID).
			Scan(&out.CriticalChannels, &out.WarningChannels, &from, &to)
		if err != nil && err != pgx.ErrNoRows {
			return err
		}
		out.QuietFrom, out.QuietTo = clock(from), clock(to)
		if err := tx.QueryRow(r.Context(), `SELECT count(*) FROM push_subscriptions WHERE user_id = $1`, p.UserID).Scan(&subs); err != nil {
			return err
		}
		return tx.QueryRow(r.Context(), `SELECT phone FROM users WHERE id = $1`, p.UserID).Scan(&phone)
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	var vapid string
	if h.Service.Push != nil {
		vapid = h.Service.Push.PublicKey
	}
	platform.JSON(w, http.StatusOK, map[string]any{
		"prefs": out, "push_subscriptions": subs, "phone": phone, "vapid_public_key": vapid,
		"sms_enabled": h.Service.SMS != nil, "whatsapp_enabled": h.Service.WhatsApp != nil,
	})
}

func clock(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("15:04")
	return &s
}

func (h *Handler) put(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	var in prefs
	if err := platform.Decode(r, &in); err != nil {
		platform.Fail(w, r, platform.ErrBadInput)
		return
	}
	for _, c := range append(append([]string{}, in.CriticalChannels...), in.WarningChannels...) {
		if !channels[c] {
			platform.Fail(w, r, platform.Errorf(http.StatusBadRequest, "unknown channel "+c))
			return
		}
	}
	if (in.QuietFrom == nil) != (in.QuietTo == nil) {
		platform.Fail(w, r, platform.Errorf(http.StatusBadRequest, "quiet hours need both ends"))
		return
	}
	if in.CriticalChannels == nil {
		in.CriticalChannels = []string{}
	}
	if in.WarningChannels == nil {
		in.WarningChannels = []string{}
	}
	err := h.Service.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(r.Context(), `
			INSERT INTO notification_prefs (tenant_id, user_id, critical_channels, warning_channels, quiet_from, quiet_to)
			VALUES ($1, $2, $3, $4, $5::time, $6::time)
			ON CONFLICT (user_id) DO UPDATE SET critical_channels = excluded.critical_channels,
			  warning_channels = excluded.warning_channels, quiet_from = excluded.quiet_from,
			  quiet_to = excluded.quiet_to, updated_at = now()`,
			p.TenantID, p.UserID, in.CriticalChannels, in.WarningChannels, in.QuietFrom, in.QuietTo)
		return err
	})
	if err != nil {
		platform.Fail(w, r, platform.ErrBadInput)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) subscribe(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	var in Subscription
	if err := platform.Decode(r, &in); err != nil || in.Endpoint == "" || in.Keys.P256dh == "" || in.Keys.Auth == "" {
		platform.Fail(w, r, platform.ErrBadInput)
		return
	}
	err := h.Service.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(r.Context(), `
			INSERT INTO push_subscriptions (tenant_id, user_id, endpoint, p256dh, auth, user_agent)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (endpoint) DO UPDATE SET user_id = excluded.user_id, p256dh = excluded.p256dh, auth = excluded.auth`,
			p.TenantID, p.UserID, in.Endpoint, in.Keys.P256dh, in.Keys.Auth, r.UserAgent())
		return err
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) unsubscribe(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	var in struct {
		Endpoint string `json:"endpoint"`
	}
	if err := platform.Decode(r, &in); err != nil || in.Endpoint == "" {
		platform.Fail(w, r, platform.ErrBadInput)
		return
	}
	err := h.Service.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(r.Context(), `DELETE FROM push_subscriptions WHERE endpoint = $1 AND user_id = $2`, in.Endpoint, p.UserID)
		return err
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// test sends a push to the caller's own devices so they can see it works.
func (h *Handler) test(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	err := h.Service.push(r.Context(), p.TenantID, p.UserID, Message{
		Title: "تنبيه تجريبي", Body: "الإشعارات تعمل على هذا الجهاز.", URL: h.Service.AppURL + "/settings", Tag: "test",
	})
	if err != nil {
		platform.Fail(w, r, platform.Errorf(http.StatusBadGateway, err.Error()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
