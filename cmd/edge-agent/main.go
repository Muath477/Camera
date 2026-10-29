// Command edge-agent runs on each site's edge device next to Frigate and go2rtc.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Muath477/Camera/edge/agent"
)

func main() {
	configPath := flag.String("config", "/etc/camera/agent.yaml", "agent configuration file")
	renderOnly := flag.Bool("render-frigate", false, "print the generated Frigate config and exit")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

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
