package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"traefik-opnsense-sync/internal/app"
	"traefik-opnsense-sync/internal/config"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	tosApp := app.NewApp(&cfg)

	if err := tosApp.Run(ctx); err != nil {
		log.Printf("app exited: %v", err)
	}
}
