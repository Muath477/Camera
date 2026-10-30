// Package notify delivers alerts and sign-in codes outside the app: SMS,
// WhatsApp and push. Vendors are behind small interfaces so a gateway can
// change without touching the rules that decide who gets what.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// TextSender delivers one short message to one phone number in E.164 form.
type TextSender interface {
	Send(ctx context.Context, to, text string) error
}

// Console logs messages instead of sending them (development and tests).
type Console struct {
	Log     *slog.Logger
	Channel string
}

func (c Console) Send(_ context.Context, to, text string) error {
	c.Log.Info("message", "channel", c.Channel, "to", to, "text", text)
	return nil
}

// HTTPSMS posts {"to","from","text"} with a bearer token. Local gateways
// (Unifonic, Msegat, Taqnyat and the like) each take a slightly different
// body; adapting is a matter of changing this one request.
type HTTPSMS struct {
	URL   string
	Token string
	From  string
	HTTP  *http.Client
}

func (s HTTPSMS) Send(ctx context.Context, to, text string) error {
	body, _ := json.Marshal(map[string]string{"to": to, "from": s.From, "text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.Token != "" {
		req.Header.Set("Authorization", "Bearer "+s.Token)
	}
	return do(s.HTTP, req, "sms")
}

// WhatsApp sends through the WhatsApp Business Cloud API. Free-form text only
// reaches people who wrote to the business in the last 24 hours; everything
// else must be an approved template, so alerts and codes use one when set.
type WhatsApp struct {
	Token    string
	PhoneID  string
	Template string // approved template with one {{1}} body parameter
	Lang     string // template language code, default "ar"
	HTTP     *http.Client
}

func (w WhatsApp) Send(ctx context.Context, to, text string) error {
	to = strings.TrimPrefix(to, "+")
	var payload map[string]any
	if w.Template != "" {
		lang := w.Lang
		if lang == "" {
			lang = "ar"
		}
		payload = map[string]any{
			"messaging_product": "whatsapp", "to": to, "type": "template",
			"template": map[string]any{
				"name": w.Template, "language": map[string]string{"code": lang},
				"components": []map[string]any{{"type": "body", "parameters": []map[string]string{{"type": "text", "text": text}}}},
			},
		}
	} else {
		payload = map[string]any{"messaging_product": "whatsapp", "to": to, "type": "text", "text": map[string]string{"body": text}}
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://graph.facebook.com/v21.0/"+w.PhoneID+"/messages", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+w.Token)
	return do(w.HTTP, req, "whatsapp")
}

func do(client *http.Client, req *http.Request, name string) error {
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s: %s: %s", name, resp.Status, strings.TrimSpace(string(b)))
	}
	return nil
}
