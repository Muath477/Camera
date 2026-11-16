// Command edge-agent runs on each site's edge device next to Frigate and go2rtc.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Muath477/Camera/edge/agent"
)

func main() {
	configPath := flag.String("config", "/etc/camera/agent.yaml", "agent configuration file")
	renderOnly := flag.Bool("render-frigate", false, "print the generated Frigate config and exit")
	scan := flag.Bool("scan", false, "find cameras on this machine's networks (and -subnet), print them as JSON and exit")
	subnet := flag.String("subnet", "", "extra network for -scan, e.g. 192.168.1.0/24")
	profilesDir := flag.String("profiles", "/etc/camera/profiles", "vendor profiles for -scan")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if *scan {
		if err := scanOnly(*subnet, *profilesDir); err != nil {
			log.Error("scan", "err", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := agent.LoadConfig(*configPath)
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	if *renderOnly {
		if err := renderFrigate(cfg); err != nil {
			log.Error("render", "err", err)
			os.Exit(1)
		}
		return
	}

	a, err := agent.New(cfg, log)
	if err != nil {
		log.Error("start", "err", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	log.Info("agent starting", "device", cfg.DeviceID, "version", agent.Version)
	if err := a.Run(ctx); err != nil {
		log.Error("run", "err", err)
		os.Exit(1)
	}
}

// scanOnly runs discovery without a config or a cloud connection, so an
// installer can check from a laptop which cameras the site's network has.
func scanOnly(subnet, profilesDir string) error {
	extra, err := agent.ParseSubnets([]string{subnet})
	if err != nil {
		return err
	}
	profiles, _ := agent.LoadProfiles(profilesDir) // optional: vendors still show without it
	subnets := append(agent.LocalSubnets(), extra...)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	found, err := agent.DiscoverAll(ctx, "", subnets, profiles)
	if err != nil {
		return err
	}
	nets := make([]string, len(subnets))
	for i, s := range subnets {
		nets[i] = s.String()
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]any{"subnets": nets, "cameras": found})
}

func renderFrigate(cfg *agent.Config) error {
	profiles, err := agent.LoadProfiles(cfg.ProfilesDir)
	if err != nil {
		return err
	}
	store, err := agent.OpenStore(cfg.DataDir+"/cameras.enc", cfg.KeyFile)
	if err != nil {
		return err
	}
	out, err := agent.FrigateConfig(cfg, store.List(), profiles)
	if err != nil {
		return err
	}
	fmt.Print(string(out))
	return nil
}
