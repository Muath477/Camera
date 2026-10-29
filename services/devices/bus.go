package devices

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/Muath477/Camera/contract"
)

// Message is one device message received from the broker.
type Message struct {
	TenantID string
	DeviceID string
	Kind     string
	Payload  []byte
}

// Bus is the services' connection to the device broker. It delivers device
// messages to subscribers and sends commands, waiting for their acks.
type Bus struct {
	client mqtt.Client
	log    *slog.Logger

	mu       sync.Mutex
	pending  map[string]chan contract.CommandAck
	handlers map[string][]func(Message)
}

// Commander sends a command to a device and waits for the result.
type Commander interface {
	Send(ctx context.Context, tenantID, deviceID, typ string, args map[string]any) (*contract.CommandAck, error)
}

func NewBus(brokerURL, clientID, user, pass string, log *slog.Logger) *Bus {
	b := &Bus{log: log, pending: map[string]chan contract.CommandAck{}, handlers: map[string][]func(Message){}}
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(clientID).
		SetUsername(user).
		SetPassword(pass).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetOnConnectHandler(func(c mqtt.Client) {
			// A shared subscription spreads device traffic across API replicas.
			c.Subscribe("$share/api/t/+/d/+/#", 1, b.onMessage)
			log.Info("device broker connected")
		})
	b.client = mqtt.NewClient(opts)
	return b
}

func (b *Bus) Connect(timeout time.Duration) error {
	tok := b.client.Connect()
	if !tok.WaitTimeout(timeout) {
		return fmt.Errorf("broker connect timeout")
	}
	return tok.Error()
}

func (b *Bus) Close() { b.client.Disconnect(500) }

// On registers a handler for one message kind (events, health, thumbs).
func (b *Bus) On(kind string, fn func(Message)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[kind] = append(b.handlers[kind], fn)
}

func (b *Bus) onMessage(_ mqtt.Client, m mqtt.Message) {
	tenant, device, kind, err := contract.ParseTopic(m.Topic())
	if err != nil {
		return
	}
	if kind == contract.KindCmdAck {
		var ack contract.CommandAck
		if json.Unmarshal(m.Payload(), &ack) != nil {
			return
		}
		b.mu.Lock()
		ch := b.pending[ack.ID]
		delete(b.pending, ack.ID)
		b.mu.Unlock()
		if ch != nil {
			ch <- ack
		}
		return
	}
	b.mu.Lock()
	hs := b.handlers[kind]
	b.mu.Unlock()
	msg := Message{TenantID: tenant, DeviceID: device, Kind: kind, Payload: m.Payload()}
	for _, h := range hs {
		h(msg)
	}
}

// Send publishes a command and waits for the device's ack or ctx expiry.
func (b *Bus) Send(ctx context.Context, tenantID, deviceID, typ string, args map[string]any) (*contract.CommandAck, error) {
	id := make([]byte, 12)
	_, _ = rand.Read(id)
	cmd := contract.Command{ID: hex.EncodeToString(id), Type: typ, Args: args}
	payload, err := json.Marshal(cmd)
	if err != nil {
		return nil, err
	}
	ch := make(chan contract.CommandAck, 1)
	b.mu.Lock()
	b.pending[cmd.ID] = ch
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		delete(b.pending, cmd.ID)
		b.mu.Unlock()
	}()

	tok := b.client.Publish(contract.Topic(tenantID, deviceID, contract.KindCmd), 1, false, payload)
	if !tok.WaitTimeout(5 * time.Second) {
		return nil, fmt.Errorf("publish timeout")
	}
	if tok.Error() != nil {
		return nil, tok.Error()
	}
	select {
	case ack := <-ch:
		return &ack, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("device did not answer: %w", ctx.Err())
	}
}
