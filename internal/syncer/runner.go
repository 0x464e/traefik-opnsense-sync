package syncer

import (
	"context"
	"errors"
	"log"

	"github.com/0x464e/traefik-opnsense-sync/internal/config"
	"github.com/0x464e/traefik-opnsense-sync/internal/model"
	"github.com/0x464e/traefik-opnsense-sync/internal/opnsense"
	"github.com/0x464e/traefik-opnsense-sync/internal/traefik"
)

type Runner struct {
	engine       *Engine
	traefik      traefik.Client
	opnsense     opnsense.Client
	hostOverride string
	dryRun       bool
}

func NewRunner(config *config.Config) *Runner {
	return &Runner{
		engine:       newEngine(config),
		traefik:      traefik.NewClient(config.Traefik.BaseURL, config.Traefik.VerifyTLS, config.Traefik.Username, config.Traefik.Password),
		opnsense:     opnsense.NewClient(config.OPNsense.BaseURL, config.OPNsense.VerifyTLS, config.OPNsense.APIKey, config.OPNsense.APISecret),
		hostOverride: config.OPNsense.HostOverride,
		dryRun:       config.DryRun,
	}
}

func (r *Runner) Sync(ctx context.Context) error {
	hostOverrideUUID, found, err := r.opnsense.FindHostOverrideUUID(ctx, r.hostOverride)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("host override '" + r.hostOverride + "' not found from OPNsense Unbound\nSee docs for setup instructions")
	}

	currentHostAliases, err := r.opnsense.GetHostAliases(ctx, hostOverrideUUID)
	if err != nil {
		return err
	}

	routers, err := r.traefik.GetRouters(ctx)
	if err != nil {
		return err
	}

	plan, err := r.engine.computePlan(routers, currentHostAliases)
	if err != nil {
		return err
	}

	return r.executePlan(ctx, plan, hostOverrideUUID)
}

func (r *Runner) executePlan(ctx context.Context, plan *model.Plan, hostOverrideUUID string) error {
	if r.dryRun {
		for _, op := range plan.Operations {
			log.Printf("[Dry Run] %s alias: %s", op.Kind.String(), op.Alias.Key())
		}
		return nil
	}

	var createCount, deleteCount int
	var errs []error

	for _, op := range plan.Operations {
		switch op.Kind {
		case model.OpCreate:
			_, err := r.opnsense.AddHostAlias(ctx, op.Alias, hostOverrideUUID)
			if err != nil {
				errs = append(errs, err)
				log.Printf("Error creating alias %s: %v", op.Alias.Key(), err)
			} else {
				createCount++
				log.Printf("Created alias: %s", op.Alias.Key())
			}
		case model.OpDelete:
			err := r.opnsense.DeleteHostAlias(ctx, op.Alias)
			if err != nil {
				errs = append(errs, err)
				log.Printf("Error deleting alias %s: %v", op.Alias.Key(), err)
			} else {
				deleteCount++
				log.Printf("Deleted alias: %s", op.Alias.Key())
			}
		}
	}

	if createCount > 0 || deleteCount > 0 {
		err := r.opnsense.ReconfigureUnbound(ctx)
		if err != nil {
			errs = append(errs, err)
			log.Printf("Error applying changes: %v", err)
		} else {
			log.Printf("Applied changes to OPNsense Unbound: %d created, %d deleted", createCount, deleteCount)
		}
	}

	if len(errs) > 0 {
		return errors.New("one or more errors occurred during sync")
	}

	return nil
}
