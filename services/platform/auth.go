package platform

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
)

// Principal is the authenticated caller.
type Principal struct {
	Subject       string // Keycloak sub
	UserID        string // users.id
	TenantID      string
	PlatformAdmin bool
	// Roles maps a site id to the caller's role there. The "" key holds a
	// tenant-wide role that applies to every site.
	Roles map[string]string
}

type ctxKey struct{}

func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

func PrincipalFrom(ctx context.Context) *Principal {
	p, _ := ctx.Value(ctxKey{}).(*Principal)
	return p
}

// claims are the token fields the platform relies on. tenant_id comes from a
// Keycloak user attribute mapper.
type claims struct {
	jwt.RegisteredClaims
	TenantID    string `json:"tenant_id"`
	RealmAccess struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

// LocalIssuer marks tokens the platform signs itself after a phone sign-in.
const LocalIssuer = "camera"

// Authenticator verifies bearer tokens and loads the caller's memberships.
// Two kinds are accepted: RS256 tokens from the identity provider's JWKS
// (checked against issuer and audience) and HS256 tokens the platform signs
// with secret after a phone sign-in.
type Authenticator struct {
	db       *DB
	jwks     jwt.Keyfunc
	secret   []byte
	issuer   string
	audience string
}

func NewAuthenticator(ctx context.Context, db *DB, jwksURL, issuer, audience, secret string) (*Authenticator, error) {
	a := &Authenticator{db: db, issuer: issuer, audience: audience, secret: []byte(secret)}
	if jwksURL != "" {
		k, err := keyfunc.NewDefaultCtx(ctx, []string{jwksURL})
		if err != nil {
			return nil, fmt.Errorf("jwks: %w", err)
		}
		a.jwks = k.Keyfunc
	}
	if a.jwks == nil && secret == "" {
		return nil, errors.New("either a JWKS url or a signing secret is required")
	}
	return a, nil
}

func (a *Authenticator) keyfunc(t *jwt.Token) (any, error) {
	switch t.Method.(type) {
	case *jwt.SigningMethodHMAC:
		if len(a.secret) == 0 || t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return a.secret, nil
	case *jwt.SigningMethodRSA:
		if a.jwks == nil {
			return nil, errors.New("unexpected signing method")
		}
		return a.jwks(t)
	}
	return nil, errors.New("unexpected signing method")
}

// Middleware rejects requests without a valid token.
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			// EventSource cannot set headers; the event stream takes the token as a query parameter.
			raw = r.URL.Query().Get("access_token")
		}
		if raw == "" {
			Error(w, http.StatusUnauthorized, "missing token")
			return
		}
		p, err := a.Verify(r.Context(), raw)
		if err != nil {
			Error(w, http.StatusUnauthorized, "invalid token")
			return
		}
		next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), p)))
	})
}

// Verify checks the token and resolves the caller's roles from memberships.
func (a *Authenticator) Verify(ctx context.Context, raw string) (*Principal, error) {
	opts := []jwt.ParserOption{jwt.WithExpirationRequired(), jwt.WithLeeway(30 * time.Second)}
	var c claims
	tok, err := jwt.ParseWithClaims(raw, &c, a.keyfunc, opts...)
	if err != nil {
		return nil, err
	}
	if _, local := tok.Method.(*jwt.SigningMethodHMAC); local {
		// Platform-signed tokens name the local issuer; a development token
		// without an issuer is accepted too.
		if c.Issuer != "" && c.Issuer != LocalIssuer {
			return nil, errors.New("unexpected issuer")
		}
	} else {
		if a.issuer != "" && c.Issuer != a.issuer {
			return nil, errors.New("unexpected issuer")
		}
		if a.audience != "" {
			ok := false
			for _, aud := range c.Audience {
				ok = ok || aud == a.audience
			}
			if !ok {
				return nil, errors.New("unexpected audience")
			}
		}
	}
	p := &Principal{Subject: c.Subject, TenantID: c.TenantID, Roles: map[string]string{}}
	for _, r := range c.RealmAccess.Roles {
		if r == "platform_admin" {
			p.PlatformAdmin = true
		}
	}
	if p.TenantID == "" {
		if p.PlatformAdmin {
			return p, nil
		}
		return nil, errors.New("token has no tenant")
	}
	err = a.db.InTenant(ctx, p.TenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE keycloak_sub = $1`, p.Subject).Scan(&p.UserID); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			SELECT coalesce(site_id::text, ''), role FROM memberships
			WHERE user_id = $1 AND (expires_at IS NULL OR expires_at > now())`, p.UserID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var site, role string
			if err := rows.Scan(&site, &role); err != nil {
				return err
			}
			p.Roles[site] = role
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("load memberships: %w", err)
	}
	return p, nil
}
