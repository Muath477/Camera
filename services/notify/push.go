package notify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// Push sends Web Push notifications signed with the platform's VAPID key.
type Push struct {
	PublicKey  string
	PrivateKey string
	Subject    string // mailto: or https: address the push service may contact
	HTTP       *http.Client
}

// Subscription is what the browser hands back from pushManager.subscribe.
type Subscription struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// Message is the payload the service worker shows.
type Message struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url,omitempty"`
	Tag   string `json:"tag,omitempty"`
	// Critical notifications ask the OS to bypass silent mode where allowed.
	Critical bool `json:"critical,omitempty"`
}

// ErrGone reports a subscription the push service no longer accepts.
var ErrGone = errors.New("subscription is gone")

func (p *Push) Send(ctx context.Context, sub Subscription, msg Message) error {
	if p == nil || p.PrivateKey == "" {
		return errors.New("push is not configured")
	}
	payload, _ := json.Marshal(msg)
	urgency := webpush.UrgencyNormal
	if msg.Critical {
		urgency = webpush.UrgencyHigh
	}
	resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{P256dh: sub.Keys.P256dh, Auth: sub.Keys.Auth},
	}, &webpush.Options{
		Subscriber:      p.Subject,
		VAPIDPublicKey:  p.PublicKey,
		VAPIDPrivateKey: p.PrivateKey,
		TTL:             600,
		Urgency:         urgency,
		HTTPClient:      p.HTTP,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return ErrGone
	case resp.StatusCode >= 300:
		return errors.New("push service: " + resp.Status)
	}
	return nil
}

// GenerateVAPID makes a key pair for development; production keeps one in
// its secrets so subscriptions survive restarts.
func GenerateVAPID() (private, public string, err error) {
	return webpush.GenerateVAPIDKeys()
}
