package notify

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Muath477/Camera/services/platform"
)

// Service routes one alert to the tenant's people according to their
// preferences. It is the events module's Provider.
type Service struct {
	DB       *platform.DB
	SMS      TextSender
	WhatsApp TextSender
	Push     *Push
	Log      *slog.Logger
	// AppURL is where push notifications open, e.g. https://app.camera.sa.
	AppURL string
}

var riyadh = time.FixedZone("AST", 3*3600)

type recipient struct {
	userID             string
	phone              *string
	critical           []string
	warning            []string
	quietFrom, quietTo *time.Time
}

func (r recipient) wants(severity, channel string) bool {
	list := r.warning
	if severity == "critical" {
		list = r.critical
	}
	for _, c := range list {
		if c == channel {
			return true
		}
	}
	return false
}

// quiet reports whether now falls inside the user's quiet hours, which may
// cross midnight (22:00 to 07:00).
func (r recipient) quiet(now time.Time) bool {
	if r.quietFrom == nil || r.quietTo == nil {
		return false
	}
	minutes := func(t time.Time) int { return t.Hour()*60 + t.Minute() }
	n, from, to := minutes(now.In(riyadh)), minutes(*r.quietFrom), minutes(*r.quietTo)
	if from == to {
		return false
	}
	if from < to {
		return n >= from && n < to
	}
	return n >= from || n < to
}

// Send delivers title/body on one channel to everyone in the tenant who
// holds an alerting role and asked for that channel at that severity.
// Informational events never leave the app.
func (s *Service) Send(ctx context.Context, tenantID, severity, channel, title, body string) error {
	if severity == "info" {
		return nil
	}
	recipients, err := s.recipients(ctx, tenantID)
	if err != nil {
		return err
	}
	now := time.Now()
	var firstErr error
	for _, r := range recipients {
		if !r.wants(severity, channel) || (severity != "critical" && r.quiet(now)) {
			continue
		}
		var err error
		switch channel {
		case "push":
			err = s.push(ctx, tenantID, r.userID, Message{Title: title, Body: body, URL: s.AppURL + "/events?severity=" + severity, Tag: "alert", Critical: severity == "critical"})
		case "sms":
			if r.phone != nil && s.SMS != nil {
				err = s.SMS.Send(ctx, *r.phone, title+": "+body)
			}
		case "whatsapp":
			if r.phone != nil && s.WhatsApp != nil {
				err = s.WhatsApp.Send(ctx, *r.phone, title+": "+body)
			}
		}
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *Service) recipients(ctx context.Context, tenantID string) ([]recipient, error) {
	var out []recipient
	err := s.DB.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT u.id, u.phone, coalesce(p.critical_channels, '{push,sms}'), coalesce(p.warning_channels, '{push}'),
			       p.quiet_from, p.quiet_to
			FROM users u LEFT JOIN notification_prefs p ON p.user_id = u.id
			WHERE EXISTS (SELECT 1 FROM memberships m WHERE m.user_id = u.id
			              AND m.role IN ('owner','site_manager','operator')
			              AND (m.expires_at IS NULL OR m.expires_at > now()))`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var r recipient
			if err := rows.Scan(&r.userID, &r.phone, &r.critical, &r.warning, &r.quietFrom, &r.quietTo); err != nil {
				return err
			}
			out = append(out, r)
		}
		return rows.Err()
	})
	return out, err
}

// push sends to every subscription of one user and forgets the ones the
// push service has dropped.
func (s *Service) push(ctx context.Context, tenantID, userID string, msg Message) error {
	if s.Push == nil {
		return nil
	}
	var subs []Subscription
	err := s.DB.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT endpoint, p256dh, auth FROM push_subscriptions WHERE user_id = $1`, userID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var sub Subscription
			if err := rows.Scan(&sub.Endpoint, &sub.Keys.P256dh, &sub.Keys.Auth); err != nil {
				return err
			}
			subs = append(subs, sub)
		}
		return rows.Err()
	})
	if err != nil {
		return err
	}
	var firstErr error
	for _, sub := range subs {
		err := s.Push.Send(ctx, sub, msg)
		switch {
		case errors.Is(err, ErrGone):
			_ = s.DB.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `DELETE FROM push_subscriptions WHERE endpoint = $1`, sub.Endpoint)
				return err
			})
		case err != nil:
			s.Log.Warn("push", "user", userID, "err", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}
