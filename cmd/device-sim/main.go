// Command device-sim pretends to be an edge device for local development:
// it reports health, raises random detections and answers commands, so the
// cloud and the web app can be exercised without hardware.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"os/signal"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/Muath477/Camera/contract"
	"github.com/Muath477/Camera/edge/agent"
)

func main() {
	broker := flag.String("broker", "tcp://localhost:1883", "MQTT broker")
	tenant := flag.String("tenant", "", "tenant id")
	device := flag.String("device", "", "device id")
	cams := flag.String("cameras", "entrance,cashier", "comma separated camera names")
	every := flag.Duration("every", 20*time.Second, "mean time between detections")
	once := flag.String("once", "", "publish one event with this label and exit")
	embedderURL := flag.String("embedder", "", "image embedder (ai/embedder) for search by description, e.g. http://embedder:8000/embed-image")
	dump := flag.String("dump-scenes", "", "write -n sample frames per label to this folder (raw/<label>/*.jpg) and exit")
	n := flag.Int("n", 40, "frames per label for -dump-scenes")
	flag.Parse()
	if *dump != "" {
		if err := dumpScenes(*dump, *n); err != nil {
			log.Fatal(err)
		}
		return
	}
	embedder := agent.NewEmbedder(*embedderURL)
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
		// Like the real agent: embed the event's picture so it can be found
		// by description. A third of the people carry a box.
		if embedder != nil && kind == contract.EventObject {
			what := label
			if label == "person" && rand.IntN(3) == 0 {
				what = "box"
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			if vec, err := embedder.EmbedImage(ctx, scene(what, slices.Index(cameras, cam))); err != nil {
				log.Printf("embedding failed: %v", err)
			} else {
				ev.Embedding = vec
			}
			cancel()
		}
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
				publish(contract.KindThumbs, contract.Thumb{Camera: cam, At: time.Now().UTC(), JPEG: scene([]string{"person", "car", "person", "motorcycle"}[(i+frame/3)%4], i)})
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
			// One camera on the device's camera port (ONVIF) and, when the app
			// passed the home network, one behind the router found by the scan.
			cams := []map[string]any{{"host": "192.168.50.31", "vendor": "hikvision", "model": "DS-2CD2143G2-I", "profile_id": "hikvision", "method": "onvif", "rtsp_port": 554}}
			nets := []string{"192.168.50.0/24"}
			if s, _ := cmd.Args["subnet"].(string); s != "" {
				octets := strings.Split(strings.SplitN(s, "/", 2)[0], ".")
				if len(octets) == 4 {
					octets[3] = "64"
				}
				cams = append(cams, map[string]any{"host": strings.Join(octets, "."), "vendor": "tp-link", "profile_id": "tapo", "method": "scan", "rtsp_port": 554, "ports": []int{554, 2020}})
				nets = append(nets, s)
			}
			ack.Result["cameras"] = cams
			ack.Result["subnets"] = nets
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
