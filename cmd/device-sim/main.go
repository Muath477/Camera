// Command device-sim pretends to be an edge device for local development:
// it reports health, raises random detections and answers commands, so the
// cloud and the web app can be exercised without hardware.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"math/rand/v2"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
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

	// Rolling thumbnails, sent while the cloud has asked for them recently.
	var thumbsUntil atomic.Int64
	go func() {
		for frame := 0; ; frame++ {
			time.Sleep(5 * time.Second)
			if time.Now().UnixNano() > thumbsUntil.Load() {
				continue
			}
			for i, cam := range cameras {
				publish(contract.KindThumbs, contract.Thumb{Camera: cam, At: time.Now().UTC(), JPEG: fakeFrame(i, frame)})
			}
		}
	}()

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
		case contract.CmdThumbs:
			minutes, _ := cmd.Args["minutes"].(float64)
			thumbsUntil.Store(time.Now().Add(time.Duration(max(minutes, 1) * float64(time.Minute))).UnixNano())
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

// fakeFrame draws a small stand-in camera picture: a wall and a floor in a
// tint that differs per camera, and a figure that walks across over time.
func fakeFrame(cam, frame int) []byte {
	const w, h = 320, 180
	tints := []color.RGBA{{70, 96, 120, 255}, {112, 86, 70, 255}, {72, 110, 90, 255}, {96, 80, 118, 255}}
	wall := tints[cam%len(tints)]
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := wall
			if y > h*2/3 {
				c = color.RGBA{wall.R / 2, wall.G / 2, wall.B / 2, 255}
			}
			// A little sensor noise so it reads as video, not a flat fill.
			n := uint8(rand.IntN(10))
			img.Set(x, y, color.RGBA{c.R + n, c.G + n, c.B + n, 255})
		}
	}
	px := (frame*14 + cam*70) % (w + 40) - 20
	for y := h/3 + 10; y < h*2/3+20; y++ {
		for x := px; x < px+16; x++ {
			if x >= 0 && x < w {
				img.Set(x, y, color.RGBA{30, 30, 36, 255})
			}
		}
	}
	var b bytes.Buffer
	_ = jpeg.Encode(&b, img, &jpeg.Options{Quality: 70})
	return b.Bytes()
}
