// Package auth signs people in with their phone number: a one-time code by
// SMS or WhatsApp, then a short-lived token the other modules accept.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Muath477/Camera/services/audit"
	"github.com/Muath477/Camera/services/notify"
	"github.com/Muath477/Camera/services/platform"
)

const (
	codeTTL     = 5 * time.Minute
	maxAttempts = 5
	maxPer10Min = 3
	tokenTTL    = 12 * time.Hour
)

type Handler struct {
	DB       *platform.DB
	Secret   []byte // HS256 key shared with platform.Authenticator
	SMS      notify.TextSender
	WhatsApp notify.TextSender
	// Dev returns the code in the response when no real sender is set, so
	// the flow can be tried without a gateway.
	Dev bool
	Log *slog.Logger
}

// PublicRoutes mounts the endpoints that run before the caller has a token.
func (h *Handler) PublicRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/auth/otp/request", h.request)
	mux.HandleFunc("POST /v1/auth/otp/verify", h.verify)
	mux.HandleFunc("POST /v1/auth/register", h.register)
}

var saudiMobile = regexp.MustCompile(`^\+9665[0-9]{8}$`)

// NormalizePhone turns the ways people write a Saudi mobile number
// (05xxxxxxxx, 5xxxxxxxx, 9665..., +9665..., with spaces or dashes) into
// E.164. Other countries must already be in E.164.
func NormalizePhone(s string) (string, error) {
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= '0' && r <= '9', r == '+':
			return r
		case r >= '٠' && r <= '٩': // Arabic-Indic digits
			return '0' + (r - '٠')
		}
		return -1
	}, s)
	switch {
	case strings.HasPrefix(s, "00"):
		s = "+" + s[2:]
	case strings.HasPrefix(s, "05") && len(s) == 10:
		s = "+966" + s[1:]
	case strings.HasPrefix(s, "5") && len(s) == 9:
		s = "+966" + s
	case strings.HasPrefix(s, "966"):
		s = "+" + s
	}
	if strings.HasPrefix(s, "+966") && !saudiMobile.MatchString(s) {
		return "", errors.New("invalid saudi mobile number")
	}
	if !strings.HasPrefix(s, "+") || len(s) < 9 || len(s) > 16 {
		return "", errors.New("invalid phone number")
	}
	return s, nil
}

func hashCode(phone, code string) string {
	sum := sha256.Sum256([]byte(phone + ":" + code))
	return hex.EncodeToString(sum[:])
}

func newCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func (h *Handler) request(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone   string `json:"phone"`
		Channel string `json:"channel"` // sms | whatsapp
	}
	if err := platform.Decode(r, &in); err != nil {
		platform.Fail(w, r, platform.ErrBadInput)
		return
	}
	phone, err := NormalizePhone(in.Phone)
	if err != nil {
		platform.Fail(w, r, platform.Errorf(http.StatusBadRequest, "رقم الجوال غير صحيح"))
		return
	}
	ctx := r.Context()
	var recent int
	if err := h.DB.Pool.QueryRow(ctx, `SELECT count(*) FROM otp_codes WHERE phone = $1 AND created_at > now() - interval '10 minutes'`, phone).Scan(&recent); err != nil {
		platform.Fail(w, r, err)
		return
	}
	if recent >= maxPer10Min {
		platform.Fail(w, r, platform.Errorf(http.StatusTooManyRequests, "طلبات كثيرة، حاول بعد عشر دقائق"))
		return
	}
	code, err := newCode()
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	if _, err := h.DB.Pool.Exec(ctx, `INSERT INTO otp_codes (phone, code_hash, expires_at) VALUES ($1, $2, now() + $3::interval)`,
		phone, hashCode(phone, code), codeTTL.String()); err != nil {
		platform.Fail(w, r, err)
		return
	}
	text := "رمز الدخول إلى Camera: " + code + "\nصالح لخمس دقائق. لا تشاركه مع أحد."
	sender := h.SMS
	if in.Channel == "whatsapp" && h.WhatsApp != nil {
		sender = h.WhatsApp
	}
	out := map[string]any{"phone": phone, "expires_in": int(codeTTL.Seconds())}
	if sender == nil {
		if !h.Dev {
			platform.Fail(w, r, platform.Errorf(http.StatusServiceUnavailable, "إرسال الرسائل غير مهيأ"))
			return
		}
		h.Log.Info("otp code (no sender configured)", "phone", phone, "code", code)
		out["dev_code"] = code
	} else if err := sender.Send(ctx, phone, text); err != nil {
		h.Log.Warn("otp send", "phone", phone, "err", err)
		platform.Fail(w, r, platform.Errorf(http.StatusBadGateway, "تعذّر إرسال الرمز، حاول مرة أخرى"))
		return
	}
	platform.JSON(w, http.StatusOK, out)
}

// consume checks a code against the newest unused one for the phone and
// burns it. Wrong guesses count towards the attempt limit.
func (h *Handler) consume(ctx context.Context, phone, code string) error {
	if len(code) != 6 {
		return errBadCode
	}
	return pgx.BeginFunc(ctx, h.DB.Pool, func(tx pgx.Tx) error {
		var id int64
		var hash string
		var attempts int
		err := tx.QueryRow(ctx, `SELECT id, code_hash, attempts FROM otp_codes
			WHERE phone = $1 AND used_at IS NULL AND expires_at > now() ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, phone).
			Scan(&id, &hash, &attempts)
		if err != nil {
			return errBadCode
		}
		if attempts >= maxAttempts {
			return errBadCode
		}
		if subtle.ConstantTimeCompare([]byte(hash), []byte(hashCode(phone, code))) != 1 {
			_, _ = tx.Exec(ctx, `UPDATE otp_codes SET attempts = attempts + 1 WHERE id = $1`, id)
			return errBadCode
		}
		_, err = tx.Exec(ctx, `UPDATE otp_codes SET used_at = now() WHERE id = $1`, id)
		return err
	})
}

var errBadCode = platform.Errorf(http.StatusUnauthorized, "الرمز غير صحيح أو منتهي")

type account struct {
	TenantID   string `json:"tenant_id"`
	TenantName string `json:"tenant_name"`
	Token      string `json:"token,omitempty"`
	UserID     string `json:"-"`
	Sub        string `json:"-"`
}

func (h *Handler) accounts(ctx context.Context, phone string) ([]account, error) {
	rows, err := h.DB.Pool.Query(ctx, `SELECT tenant_id, user_id FROM phone_logins WHERE phone = $1`, phone)
	if err != nil {
		return nil, err
	}
	var out []account
	for rows.Next() {
		var a account
		if err := rows.Scan(&a.TenantID, &a.UserID); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, a)
	}
	rows.Close()
	for i := range out {
		a := &out[i]
		err := h.DB.InTenant(ctx, a.TenantID, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT u.keycloak_sub, t.name FROM users u JOIN tenants t ON t.id = u.tenant_id WHERE u.id = $1`, a.UserID).
				Scan(&a.Sub, &a.TenantName)
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Token signs a session for one user in one tenant.
func (h *Handler) Token(sub, tenantID string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": platform.LocalIssuer, "sub": sub, "tenant_id": tenantID,
		"iat": now.Unix(), "exp": now.Add(tokenTTL).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(h.Secret)
}

func (h *Handler) verify(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone    string `json:"phone"`
		Code     string `json:"code"`
		TenantID string `json:"tenant_id"`
	}
	if err := platform.Decode(r, &in); err != nil {
		platform.Fail(w, r, platform.ErrBadInput)
		return
	}
	phone, err := NormalizePhone(in.Phone)
	if err != nil {
		platform.Fail(w, r, platform.Errorf(http.StatusBadRequest, "رقم الجوال غير صحيح"))
		return
	}
	if err := h.consume(r.Context(), phone, in.Code); err != nil {
		platform.Fail(w, r, err)
		return
	}
	accts, err := h.accounts(r.Context(), phone)
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	if len(accts) == 0 {
		platform.JSON(w, http.StatusNotFound, map[string]any{"error": "no_account", "phone": phone})
		return
	}
	// One code signs the person into every account their number holds; the
	// app keeps the one they pick.
	chosen := 0
	for i := range accts {
		a := &accts[i]
		if a.Token, err = h.Token(a.Sub, a.TenantID); err != nil {
			platform.Fail(w, r, err)
			return
		}
		if a.TenantID == in.TenantID {
			chosen = i
		}
		_ = h.DB.InTenant(r.Context(), a.TenantID, func(tx pgx.Tx) error {
			return audit.Append(r.Context(), tx, a.TenantID, audit.Entry{ActorType: "user", ActorID: a.UserID,
				Action: "auth.login", TargetType: "user", TargetID: a.UserID, IP: audit.ClientIP(r), DeviceInfo: r.UserAgent()})
		})
	}
	platform.JSON(w, http.StatusOK, map[string]any{"token": accts[chosen].Token, "tenant_id": accts[chosen].TenantID, "accounts": accts})
}

// register creates a tenant with the caller as its owner. The phone must
// have just been verified and must not belong to an account yet.
func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone      string `json:"phone"`
		Code       string `json:"code"`
		Name       string `json:"name"`
		TenantName string `json:"tenant_name"`
		CRNumber   string `json:"cr_number"`
	}
	if err := platform.Decode(r, &in); err != nil || strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.TenantName) == "" {
		platform.Fail(w, r, platform.ErrBadInput)
		return
	}
	phone, err := NormalizePhone(in.Phone)
	if err != nil {
		platform.Fail(w, r, platform.Errorf(http.StatusBadRequest, "رقم الجوال غير صحيح"))
		return
	}
	ctx := r.Context()
	if err := h.consume(ctx, phone, in.Code); err != nil {
		platform.Fail(w, r, err)
		return
	}
	var existing int
	if err := h.DB.Pool.QueryRow(ctx, `SELECT count(*) FROM phone_logins WHERE phone = $1`, phone).Scan(&existing); err != nil {
		platform.Fail(w, r, err)
		return
	}
	if existing > 0 {
		platform.Fail(w, r, platform.Errorf(http.StatusConflict, "هذا الرقم مسجّل من قبل، سجّل الدخول"))
		return
	}
	var tenantID string
	if err := h.DB.Pool.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&tenantID); err != nil {
		platform.Fail(w, r, err)
		return
	}
	sub := "phone:" + phone + ":" + tenantID
	var userID string
	err = h.DB.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO tenants (id, name, cr_number) VALUES ($1, $2, nullif($3,''))`, tenantID, strings.TrimSpace(in.TenantName), strings.TrimSpace(in.CRNumber)); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `INSERT INTO users (tenant_id, keycloak_sub, name, phone) VALUES ($1, $2, $3, $4) RETURNING id`,
			tenantID, sub, strings.TrimSpace(in.Name), phone).Scan(&userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO memberships (tenant_id, user_id, role) VALUES ($1, $2, 'owner')`, tenantID, userID); err != nil {
			return err
		}
		return audit.Append(ctx, tx, tenantID, audit.Entry{ActorType: "user", ActorID: userID, Action: "tenant.register",
			TargetType: "tenant", TargetID: tenantID, IP: audit.ClientIP(r), DeviceInfo: r.UserAgent()})
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	token, err := h.Token(sub, tenantID)
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	platform.JSON(w, http.StatusCreated, map[string]any{"token": token, "tenant_id": tenantID})
}
