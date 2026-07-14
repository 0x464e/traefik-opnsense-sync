package syncer

import (
	"sort"
	"strings"

	"github.com/0x464e/traefik-opnsense-sync/internal/config"
	"github.com/0x464e/traefik-opnsense-sync/internal/model"
)

// Engine reconciles a source-agnostic set of desired DNS host aliases
// against the aliases currently present in OPNsense, producing a create/delete
// Plan. It only considers OPNsense aliases tagged with descTag as owned by us.
type Engine struct {
	descTag string
}

func newEngine(cfg *config.Config) *Engine {
	return &Engine{
		descTag: cfg.Sync.DescriptionTag,
	}
}

func (e *Engine) computePlan(desiredAliases []model.HostAlias, aliases []model.HostAlias) (*model.Plan, error) {
	currentAliases, err := e.currentFromOPNsense(aliases)
	if err != nil {
		return nil, err
	}

	desired := make(map[string]model.HostAlias, len(desiredAliases))
	for _, d := range desiredAliases {
		desired[d.Key()] = d
	}

	current := make(map[string]model.HostAlias, len(currentAliases))
	for _, c := range currentAliases {
		current[c.Key()] = c
	}

	var operations []model.Operation

	// determine creates
	for key, d := range desired {
		if _, exists := current[key]; !exists {
			operations = append(operations, model.Operation{
				Kind:  model.OpCreate,
				Alias: d,
			})
		}
	}

	// determine deletes
	for key, c := range current {
		if _, exists := desired[key]; !exists {
			operations = append(operations, model.Operation{
				Kind:  model.OpDelete,
				Alias: c,
			})
		}
	}

	sort.Slice(operations, func(i, j int) bool {
		// delete before create
		if operations[i].Kind != operations[j].Kind {
			return operations[i].Kind == model.OpDelete
		}

		// alphabetical by fqdn
		ai := strings.ToLower(operations[i].Alias.Hostname + "." + operations[i].Alias.Domain)
		aj := strings.ToLower(operations[j].Alias.Hostname + "." + operations[j].Alias.Domain)
		if ai == aj {
			return false
		}
		return ai < aj
	})

	return &model.Plan{
		Operations: operations,
	}, nil
}

func (e *Engine) currentFromOPNsense(aliases []model.HostAlias) ([]model.HostAlias, error) {
	var current []model.HostAlias

	for _, alias := range aliases {
		if alias.Description != e.descTag {
			continue
		}
		current = append(current, alias)
	}

	return current, nil
}
