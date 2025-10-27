package app

import (
	"context"
	"log"
	"time"

	"github.com/0x464e/traefik-opnsense-sync/internal/config"
	"github.com/0x464e/traefik-opnsense-sync/internal/syncer"
)

type App struct {
	cfg      *config.Config
	runner   *syncer.Runner
	interval time.Duration
}

func NewApp(cfg *config.Config) *App {
	return &App{
		cfg:      cfg,
		runner:   syncer.NewRunner(cfg),
		interval: cfg.Reconcile.Interval,
	}
}

func (a *App) Run(ctx context.Context) error {
	log.Println("running initial sync...")

	// initial sync
	if err := a.syncOnce(ctx); err != nil {
		if a.cfg.DryRun {
			return err
		}
		log.Printf("sync error: %v", err)
	}

	// dry-run: do not start ticker
	if a.cfg.DryRun {
		log.Println("dry-run enabled; executed a single sync and exiting")
		return nil
	}
	log.Println("initial sync complete")
	log.Printf("starting sync loop interval=%s", a.interval)
	log.Println("no more output will be shown until changes are detected")

	t := time.NewTicker(a.interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("shutdown signal received, stopping app")
			return nil
		case <-t.C:
			if err := a.syncOnce(ctx); err != nil {
				log.Printf("sync error: %v", err)
			}
		}
	}
}

func (a *App) syncOnce(ctx context.Context) error {
	return a.runner.Sync(ctx)
}
