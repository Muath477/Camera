// Package agent is the management agent that runs on every edge device. It
// owns the device's link to the cloud: it turns Frigate events into platform
// events, reports health, runs commands and keeps Frigate's config in sync.
package agent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/Muath477/Camera/contract"
)

const Version = "0.1.0"

type Agent struct {
	cfg      *Config
	log      *slog.Logger
	profiles *Profiles
	store    *Store
	outbox   *Outbox
	frigate  *Frigate
	uploader *Uploader

	cloud mqtt.Client
	local mqtt.Client

	thumbsUntil atomic.Int64
	wake        chan struct{}
}

func New(cfg *Config, log *slog.Logger) (*Agent, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return nil, err
	}
	profiles, err := LoadProfiles(cfg.ProfilesDir)
	if err != nil {
		return nil, fmt.Errorf("profiles: %w", err)
	}
	store, err := OpenStore(filepath.Join(cfg.DataDir, "cameras.enc"), cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("camera store: %w", err)
	}
	outbox, err := OpenOutbox(filepath.Join(cfg.DataDir, "outbox.db"))
	if err != nil {
		return nil, fmt.Errorf("outbox: %w", err)
	}
	tlsCfg, err := cloudTLS(cfg)
	if err != nil {
		return nil, err
	}
	return &Agent{
		cfg:      cfg,
		log:      log,
		profiles: profiles,
		store:    store,
		outbox:   outbox,
		frigate:  NewFrigate(cfg.Frigate.URL),
		uploader: NewUploader(cfg.Cloud.APIURL, tlsCfg),
		wake:     make(chan struct{}, 1),
	}, nil
}

// cloudTLS builds the mTLS config used for MQTT and the device API. The
// client certificate carries device_id and tenant_id and is renewed by ACME.
func cloudTLS(cfg *Config) (*tls.Config, error) {
	tc := &tls.Config{MinVersion: tls.VersionTLS13}
	if cfg.Cloud.CAFile != "" {
		pem, err := os.ReadFile(cfg.Cloud.CAFile)
		if err != nil {
			return nil, err
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("no certificates in %s", cfg.Cloud.CAFile)
		}
		tc.RootCAs = pool
	}
	if cfg.Cloud.CertFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.Cloud.CertFile, cfg.Cloud.KeyFile)
		if err != nil {
			return nil, err
		}
		tc.Certificates = []tls.Certificate{cert}
	}
	return tc, nil
}

func (a *Agent) topic(kind string) string {
	return contract.Topic(a.cfg.TenantID, a.cfg.DeviceID, kind)
}

// Run blocks until ctx is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	defer a.outbox.Close()

	tlsCfg, err := cloudTLS(a.cfg)
	if err != nil {
		return err
	}
	cloudOpts := mqtt.NewClientOptions().
		AddBroker(a.cfg.Cloud.BrokerURL).
		SetClientID(a.cfg.DeviceID).
		SetTLSConfig(tlsCfg).
		SetCleanSession(false).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetMaxReconnectInterval(time.Minute).
		SetKeepAlive(30 * time.Second).
		SetOnConnectHandler(func(c mqtt.Client) {
			a.log.Info("cloud connected")
			c.Subscribe(a.topic(contract.KindCmd), 1, a.onCommand(ctx))
			a.kick()
		}).
		SetConnectionLostHandler(func(_ mqtt.Client, err error) {
			a.log.Warn("cloud connection lost", "err", err)
		})
	a.cloud = mqtt.NewClient(cloudOpts)
	a.cloud.Connect()

	localOpts := mqtt.NewClientOptions().
		AddBroker(a.cfg.Frigate.BrokerURL).
		SetClientID(a.cfg.DeviceID + "-local").
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetOnConnectHandler(func(c mqtt.Client) {
			c.Subscribe("frigate/events", 1, a.onFrigateEvent(ctx))
		})
	a.local = mqtt.NewClient(localOpts)
	a.local.Connect()

	health := time.NewTicker(a.cfg.HealthInterval)
	thumbs := time.NewTicker(a.cfg.ThumbInterval)
	drain := time.NewTicker(10 * time.Second)
	defer health.Stop()
	defer thumbs.Stop()
	defer drain.Stop()

	a.publishHealth(ctx)
	for {
		select {
		case <-ctx.Done():
			a.cloud.Disconnect(500)
			a.local.Disconnect(500)
			return nil
		case <-health.C:
			a.publishHealth(ctx)
		case <-thumbs.C:
			if time.Now().Unix() < a.thumbsUntil.Load() {
				a.publishThumbs(ctx)
			}
		case <-drain.C:
			a.drainOutbox()
		case <-a.wake:
			a.drainOutbox()
		}
	}
}

func (a *Agent) kick() {
	select {
	case a.wake <- struct{}{}:
	default:
	}
}

// enqueue stores a message durably, then tries to send it right away.
func (a *Agent) enqueue(kind string, v any) {
	payload, err := json.Marshal(v)
	if err != nil {
		a.log.Error("marshal", "kind", kind, "err", err)
		return
	}
	if err := a.outbox.Push(a.topic(kind), payload); err != nil {
		a.log.Error("outbox push", "err", err)
		return
	}
	a.kick()
}

func (a *Agent) drainOutbox() {
	if !a.cloud.IsConnectionOpen() {
		return
	}
	for {
		n, err := a.outbox.Drain(100, func(topic string, payload []byte) error {
			tok := a.cloud.Publish(topic, 1, false, payload)
			if !tok.WaitTimeout(10 * time.Second) {
				return fmt.Errorf("publish timeout")
			}
			return tok.Error()
		})
		if err != nil {
			a.log.Warn("outbox drain stopped", "sent", n, "err", err)
			return
		}
		if n < 100 {
			return
		}
	}
}

func (a *Agent) onFrigateEvent(ctx context.Context) mqtt.MessageHandler {
	return func(_ mqtt.Client, m mqtt.Message) {
		ev, wantSnapshot, err := MapFrigateEvent(m.Payload())
		if err != nil {
			a.log.Warn("bad frigate event", "err", err)
			return
		}
		if ev == nil {
			return
		}
		if wantSnapshot && a.uploader.Enabled() {
			if img, err := a.frigate.EventSnapshot(ctx, ev.ID); err == nil {
				key, err := a.uploader.Upload(ctx, "snapshot", ev.Camera, "image/jpeg", img)
				if err != nil {
					a.log.Warn("snapshot upload failed", "event", ev.ID, "err", err)
				} else {
					ev.SnapshotKey = key
				}
			}
		}
		a.enqueue(contract.KindEvents, ev)
	}
}

func (a *Agent) onCommand(ctx context.Context) mqtt.MessageHandler {
	return func(_ mqtt.Client, m mqtt.Message) {
		var cmd contract.Command
		if err := json.Unmarshal(m.Payload(), &cmd); err != nil {
			a.log.Warn("bad command", "err", err)
			return
		}
		go func() {
			cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			ack := a.handleCommand(cctx, cmd)
			a.log.Info("command", "id", cmd.ID, "type", cmd.Type, "ok", ack.OK, "err", ack.Error)
			payload, _ := json.Marshal(ack)
			// Acks are not queued: a stale answer is worse than none.
			a.cloud.Publish(a.topic(contract.KindCmdAck), 1, false, payload)
		}()
	}
}

func (a *Agent) publishHealth(ctx context.Context) {
	h := contract.Health{
		At:           time.Now().UTC(),
		AgentVersion: Version,
		CPUTempC:     CPUTemp(),
		QueueDepth:   a.outbox.Len(),
	}
	if total, used, err := DiskUsage(a.cfg.RecordingsPath); err == nil {
		h.DiskTotal, h.DiskUsed = total, used
	}

	online := map[string]bool{}
	byName := map[string]*contract.CameraHealth{}
	for _, c := range a.store.List() {
		byName[c.Name] = &contract.CameraHealth{Camera: c.Name}
	}
	if st, err := a.frigate.Stats(ctx); err == nil {
		h.FrigateVer = st.Service.Version
		for name, cs := range st.Cameras {
			ch, ok := byName[name]
			if !ok {
				ch = &contract.CameraHealth{Camera: name}
				byName[name] = ch
			}
			ch.FPS, ch.SkippedFPS = cs.CameraFPS, cs.SkippedFPS
			online[name] = cs.CameraFPS > 0
		}
	} else {
		a.log.Warn("frigate stats", "err", err)
	}

	var rates []float64
	if usage, err := a.frigate.RecordingUsage(ctx); err == nil {
		for name, u := range usage {
			if ch, ok := byName[name]; ok && u.Bandwidth > 0 {
				ch.BitrateKbps = u.Bandwidth * (1 << 20) * 8 / 3600 / 1000
				ch.RetainedHours = u.Usage / u.Bandwidth
				rates = append(rates, u.Bandwidth)
			}
		}
	}
	h.RetentionDays = RetentionDays(h.DiskTotal, rates)

	for _, ch := range byName {
		ch.Online = online[ch.Camera]
		h.Cameras = append(h.Cameras, *ch)
	}
	if h.RetentionDays > 0 && h.RetentionDays < float64(a.cfg.RetentionDays) {
		a.enqueue(contract.KindEvents, contract.Event{
			ID:        fmt.Sprintf("retention-%d", time.Now().Unix()/3600),
			Kind:      contract.EventSystem,
			Label:     "retention_short",
			Score:     h.RetentionDays,
			StartTime: h.At,
		})
	}

	// Health is a snapshot of now; when offline the next one replaces it.
	if a.cloud != nil && a.cloud.IsConnectionOpen() {
		payload, _ := json.Marshal(h)
		a.cloud.Publish(a.topic(contract.KindHealth), 0, false, payload)
	}
}

func (a *Agent) publishThumbs(ctx context.Context) {
	if !a.cloud.IsConnectionOpen() {
		return
	}
	for _, c := range a.store.List() {
		img, err := a.frigate.LatestJPEG(ctx, c.Name, 180)
		if err != nil {
			continue
		}
		payload, _ := json.Marshal(contract.Thumb{Camera: c.Name, At: time.Now().UTC(), JPEG: img})
		a.cloud.Publish(a.topic(contract.KindThumbs), 0, false, payload)
	}
}
