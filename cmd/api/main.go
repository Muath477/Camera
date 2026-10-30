// Command api runs the cloud services as one process: tenancy, devices,
// cameras, events, search, media, compliance and audit.
package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/Muath477/Camera/contract"
	"github.com/Muath477/Camera/services/audit"
	"github.com/Muath477/Camera/services/auth"
	"github.com/Muath477/Camera/services/cameras"
	"github.com/Muath477/Camera/services/compliance"
	"github.com/Muath477/Camera/services/db"
	"github.com/Muath477/Camera/services/devices"
	"github.com/Muath477/Camera/services/events"
	"github.com/Muath477/Camera/services/media"
	"github.com/Muath477/Camera/services/notify"
	"github.com/Muath477/Camera/services/platform"
	"github.com/Muath477/Camera/services/search"
	"github.com/Muath477/Camera/services/tenancy"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)
	if err := run(log); err != nil {
		log.Error("api", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if dsn := os.Getenv("MIGRATE_DATABASE_URL"); dsn != "" {
		if err := platform.Migrate(ctx, dsn, db.Migrations()); err != nil {
			return err
		}
		log.Info("migrations applied")
	}
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "migrate":
			return nil
		case "vapid":
			// Prints a fresh Web Push key pair for the deployment's environment file.
			priv, pub, err := notify.GenerateVAPID()
			if err != nil {
				return err
			}
			fmt.Printf("VAPID_PUBLIC_KEY=%s\nVAPID_PRIVATE_KEY=%s\n", pub, priv)
			return nil
		}
	}

	database, err := platform.Connect(ctx, env("DATABASE_URL", "postgres://camera_api:camera_api@localhost:5432/camera"))
	if err != nil {
		return err
	}
	defer database.Close()

	if len(os.Args) > 1 && os.Args[1] == "seed" {
		return seed(ctx, database, log)
	}

	production := os.Getenv("APP_ENV") == "production"
	jwtSecret := env("JWT_SECRET", os.Getenv("DEV_JWT_SECRET"))
	if production && len(jwtSecret) < 32 {
		return errors.New("JWT_SECRET of at least 32 bytes is required in production")
	}
	authn, err := platform.NewAuthenticator(ctx, database, os.Getenv("OIDC_JWKS_URL"), os.Getenv("OIDC_ISSUER"),
		os.Getenv("OIDC_AUDIENCE"), jwtSecret)
	if err != nil {
		return err
	}

	bus := devices.NewBus(env("MQTT_URL", "tcp://localhost:1883"), "api-"+hostname(), os.Getenv("MQTT_USER"), os.Getenv("MQTT_PASSWORD"), log)
	if err := bus.Connect(15 * time.Second); err != nil {
		return err
	}
	defer bus.Close()

	store, err := objectStore()
	if err != nil {
		return err
	}
	signKey, err := signingKey()
	if err != nil {
		return err
	}

	notifier := notifyService(database, log, production)
	dispatcher := &events.Dispatcher{DB: database, Provider: notifier, HTTP: &http.Client{Timeout: 10 * time.Second}, Log: log}
	var embedder search.Embedder
	if u := os.Getenv("EMBEDDER_URL"); u != "" {
		embedder = search.HTTPEmbedder{URL: u, HTTP: &http.Client{Timeout: 10 * time.Second}}
	}

	devicesH := &devices.Handler{DB: database, Cmd: bus, EnrollSecret: []byte(env("ENROLL_SECRET", "dev-enroll-secret")), Log: log}
	camerasH := &cameras.Handler{DB: database, Cmd: bus}
	eventsH := &events.Handler{DB: database, Cmd: bus, Notifier: dispatcher, Log: log}

	bus.On(contract.KindHealth, devicesH.IngestHealth)
	bus.On(contract.KindThumbs, camerasH.IngestThumb)
	bus.On(contract.KindEvents, eventsH.Ingest)

	mux := http.NewServeMux()
	wrap := authn.Middleware
	for _, m := range []interface {
		Routes(*http.ServeMux, func(http.Handler) http.Handler)
	}{
		&tenancy.Handler{DB: database},
		devicesH,
		camerasH,
		eventsH,
		&search.Handler{DB: database, Embedder: embedder},
		&media.Handler{DB: database, Cmd: bus, Store: store, SignKey: signKey, TrustProxyIdentity: os.Getenv("TRUST_PROXY_DEVICE_IDENTITY") == "true"},
		&compliance.Handler{DB: database, LinkSecret: []byte(env("LINK_SECRET", "dev-link-secret"))},
		&audit.Handler{DB: database},
		&notify.Handler{Service: notifier},
	} {
		m.Routes(mux, wrap)
	}
	(&auth.Handler{DB: database, Secret: []byte(jwtSecret), SMS: notifier.SMS, WhatsApp: notifier.WhatsApp, Dev: !production, Log: log}).PublicRoutes(mux)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := database.Pool.Ping(r.Context()); err != nil {
			http.Error(w, "db", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              env("LISTEN_ADDR", ":8080"),
		Handler:           cors(env("CORS_ORIGINS", "http://localhost:3000"), mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shut, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(shut)
	}()
	log.Info("api listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func hostname() string {
	h, _ := os.Hostname()
	return h
}

// objectStore connects to the bucket. S3_PUBLIC_URL, when set, is the address
// devices and browsers reach the same bucket at; presigned links carry it.
func objectStore() (media.Store, error) {
	creds := credentials.NewStaticV4(env("S3_ACCESS_KEY", "camera"), env("S3_SECRET_KEY", "camera-secret"), "")
	region := env("S3_REGION", "me-central2")
	client, err := minio.New(env("S3_ENDPOINT", "localhost:9000"), &minio.Options{Creds: creds, Secure: os.Getenv("S3_SECURE") == "true", Region: region})
	if err != nil {
		return nil, err
	}
	store := media.S3{Client: client, Bucket: env("S3_BUCKET", "camera-media")}
	if public := os.Getenv("S3_PUBLIC_URL"); public != "" {
		u, err := url.Parse(public)
		if err != nil || u.Host == "" {
			return nil, errors.New("S3_PUBLIC_URL must be a URL such as https://media.example.sa")
		}
		if store.Presign, err = minio.New(u.Host, &minio.Options{Creds: creds, Secure: u.Scheme == "https", Region: region}); err != nil {
			return nil, err
		}
	}
	return store, nil
}

// signingKey loads the Ed25519 seed (base64) that signs snapshots. In
// production it comes from the cloud KMS; a missing key is only allowed in
// development, where a random one is generated per run.
func signingKey() (ed25519.PrivateKey, error) {
	seed := os.Getenv("SNAPSHOT_SIGNING_SEED")
	if seed == "" {
		if os.Getenv("APP_ENV") == "production" {
			return nil, errors.New("SNAPSHOT_SIGNING_SEED is required in production")
		}
		_, key, err := ed25519.GenerateKey(nil)
		return key, err
	}
	raw, err := base64.StdEncoding.DecodeString(seed)
	if err != nil || len(raw) != ed25519.SeedSize {
		return nil, errors.New("SNAPSHOT_SIGNING_SEED must be 32 bytes, base64")
	}
	return ed25519.NewKeyFromSeed(raw), nil
}

func cors(origins string, next http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range strings.Split(origins, ",") {
		allowed[strings.TrimSpace(o)] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if o := r.Header.Get("Origin"); allowed[o] {
			w.Header().Set("Access-Control-Allow-Origin", o)
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// notifyService wires the message gateways from the environment. Anything
// unset stays off: in development codes are logged and returned instead.
func notifyService(database *platform.DB, log *slog.Logger, production bool) *notify.Service {
	client := &http.Client{Timeout: 15 * time.Second}
	s := &notify.Service{DB: database, Log: log, AppURL: env("APP_URL", "http://localhost:3000")}
	switch env("SMS_PROVIDER", "") {
	case "http":
		s.SMS = notify.HTTPSMS{URL: os.Getenv("SMS_HTTP_URL"), Token: os.Getenv("SMS_HTTP_TOKEN"), From: env("SMS_FROM", "Camera"), HTTP: client}
	case "console":
		s.SMS = notify.Console{Log: log, Channel: "sms"}
	}
	if token := os.Getenv("WHATSAPP_TOKEN"); token != "" {
		s.WhatsApp = notify.WhatsApp{Token: token, PhoneID: os.Getenv("WHATSAPP_PHONE_ID"), Template: os.Getenv("WHATSAPP_TEMPLATE"), Lang: env("WHATSAPP_LANG", "ar"), HTTP: client}
	}
	pub, priv := os.Getenv("VAPID_PUBLIC_KEY"), os.Getenv("VAPID_PRIVATE_KEY")
	if pub == "" || priv == "" {
		if production {
			log.Warn("push disabled: VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY are not set")
		} else if p, k, err := notify.GenerateVAPID(); err == nil {
			priv, pub = p, k
			log.Info("push: generated a development VAPID key pair; set VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY to keep subscriptions across restarts")
		}
	}
	if pub != "" && priv != "" {
		s.Push = &notify.Push{PublicKey: pub, PrivateKey: priv, Subject: env("VAPID_SUBJECT", "mailto:ops@camera.sa"), HTTP: client}
	}
	return s
}
