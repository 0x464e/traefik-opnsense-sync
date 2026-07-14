package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/0x464e/traefik-opnsense-sync/internal/app"
	"github.com/0x464e/traefik-opnsense-sync/internal/config"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
	builtBy = "unspecified"
)

func main() {
	args := os.Args[1:]

	// for flags, only printing version is supported
	if len(args) == 1 && isVersionFlag(args[0]) {
		fmt.Printf(
			"trafik-opnsense-sync\n"+
				"version:  %s\n"+
				"commit:   %s\n"+
				"date:     %s\n"+
				"built by: %s\n", version, commit, date, builtBy)
		return
	}
	if len(args) > 0 {
		_, _ = fmt.Fprintf(os.Stderr, "error: unrecognized argument(s): %v\n", args)
		_, _ = fmt.Fprintf(os.Stderr, "only version printing argument is supported (-v, --version, or version)\n")
		os.Exit(2)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	tosApp, err := app.NewApp(&cfg)
	if err != nil {
		log.Fatalf("init app: %v", err)
	}

	if err := tosApp.Run(ctx); err != nil {
		log.Printf("app exited: %v", err)
	}
}

func isVersionFlag(flag string) bool {
	switch flag {
	case "-v", "--version", "version":
		return true
	default:
		return false
	}
}
