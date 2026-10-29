// Command device-sim pretends to be an edge device for local development:
// it reports health, raises random detections and answers commands, so the
// cloud and the web app can be exercised without hardware.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"os/signal"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/Muath477/Camera/contract"
)

func main() {
	broker := flag.String("broker", "tcp://localhost:1883", "MQTT broker")
	tenant := flag.String("tenant", "", "tenant id")
	device := flag.String("device", "", "device id")
	cams := flag.String("cameras", "entrance,cashier", "comma separated camera names")
	every := flag.Duration("every", 20*time.Second, "mean time between detections")
	once := flag.String("once", "", "publish one event with this label and exit")
	flag.Parse()
	if *tenant == "" || *device == "" {
		log.Fatal("-tenant and -device are required")
	}
	cameras := strings.Split(*cams, ",")
	topic := func(kind string) string { return contract.Topic(*tenant, *device, kind) }

	c := mqtt.NewClient(mqtt.NewClientOptions().AddBroker(*broker).SetClientID("sim-" + *device).SetAutoReconnect(true))
	if t := c.Connect(); t.Wait() && t.Error() != nil {
		log.Fatal(t.Error())
	}
	publish := func(kind string, v any) {
		b, _ := json.Marshal(v)
		c.Publish(topic(kind), 1, false, b).Wait()
	}

	event := func(label string) {
		cam := cameras[rand.IntN(len(cameras))]
		kind := contract.EventObject
		if label == "glass" || label == "fire_alarm" || label == "scream" {
			kind = contract.EventAudio
		}
		start := time.Now().UTC()
		end := start.Add(8 * time.Second)
		ev := contract.Event{ID: fmt.Sprintf("%d.%d-sim", start.Unix(), rand.IntN(1e6)), Kind: kind, Camera: cam, Label: label,
			Score: 0.6 + rand.Float64()*0.39, Zones: []string{cam}, StartTime: start, EndTime: &end}
		publish(contract.KindEvents, ev)
		log.Printf("event %s on %s", label, cam)
	}
	if *once != "" {
		event(*once)
		c.Disconnect(250)
		return
	}

	c.Subscribe(topic(contract.KindCmd), 1, func(_ mqtt.Client, m mqtt.Message) {
		var cmd contract.Command
		if json.Unmarshal(m.Payload(), &cmd) != nil {
			return
		}
		ack := contract.CommandAck{ID: cmd.ID, OK: true, Result: map[string]any{}}
		switch cmd.Type {
		case contract.CmdSnapshot:
			ack.Result["sha256"] = "0000000000000000000000000000000000000000000000000000000000000000"
			ack.Result["taken_at"] = time.Now().UTC().Format(time.RFC3339Nano)
		case contract.CmdAddCamera:
			ack.Result["profile_id"] = "hikvision"
		case contract.CmdDiscover:
			ack.Result["cameras"] = []map[string]string{{"host": "192.168.50.31", "vendor": "hikvision", "model": "DS-2CD2143G2-I", "profile_id": "hikvision"}}
		case contract.CmdWebRTCOffer:
			ack.OK, ack.Error = false, "the simulator has no video"
		}
		log.Printf("command %s", cmd.Type)
		b, _ := json.Marshal(ack)
		c.Publish(topic(contract.KindCmdAck), 1, false, b)
	})

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	health := time.NewTicker(30 * time.Second)
	labels := []string{"person", "person", "person", "car", "motorcycle", "glass"}
	sendHealth := func() {
		h := contract.Health{At: time.Now().UTC(), AgentVersion: "sim", CPUTempC: 48 + rand.Float64()*8,
			DiskTotal: 8e12, DiskUsed: 5e12, RetentionDays: 96}
		for _, cam := range cameras {
			h.Cameras = append(h.Cameras, contract.CameraHealth{Camera: cam, Online: true, FPS: 5, BitrateKbps: 2048})
		}
		publish(contract.KindHealth, h)
	}
	sendHealth()
	for {
		wait := time.Duration(rand.ExpFloat64() * float64(*every))
		select {
		case <-stop:
			c.Disconnect(250)
			return
		case <-health.C:
			sendHealth()
		case <-time.After(wait):
			event(labels[rand.IntN(len(labels))])
		}
	}
}
