package syncer

import (
	"log"
	"sort"
	"strings"
	"traefik-opnsense-sync/internal/config"
	"traefik-opnsense-sync/internal/exrex"
	"traefik-opnsense-sync/internal/model"
	"traefik-opnsense-sync/internal/traefik"
)

type Engine struct {
	regexGenerator     *exrex.Exrex
	includeEntryPoints []string
	ignoreRouters      []string
	includeProviders   []string
	ignoreProviders    []string
	descTag            string
}

func newEngine(cfg *config.Config) *Engine {
	return &Engine{
		regexGenerator:     exrex.NewExrexRunner(cfg),
		includeEntryPoints: cfg.Traefik.IncludeEntryPoints,
		ignoreRouters:      cfg.Traefik.IgnoreRouters,
		includeProviders:   cfg.Traefik.IncludeProviders,
		ignoreProviders:    cfg.Traefik.IgnoreProviders,
		descTag:            cfg.Reconcile.DescriptionTag,
	}
}

func (e *Engine) computePlan(routers []traefik.Router, aliases []model.HostAlias) (*model.Plan, error) {
	desiredAliases, err := e.desiredFromTraefik(routers)
	if err != nil {
		return nil, err
	}

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

func (e *Engine) desiredFromTraefik(routers []traefik.Router) ([]model.HostAlias, error) {
	var desired []traefik.Router

	for _, router := range routers {
		// filter by entrypoints
		if len(e.includeEntryPoints) > 0 {
			matched := false
			for _, ep := range router.EntryPoints {
				for _, includeEp := range e.includeEntryPoints {
					if ep == includeEp {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if !matched {
				continue
			}
		}

		// filter by providers
		if len(e.includeProviders) > 0 {
			matched := false
			for _, includeProvider := range e.includeProviders {
				if router.Provider == includeProvider {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if len(e.ignoreProviders) > 0 {
			ignored := false
			for _, ignoreProvider := range e.ignoreProviders {
				if router.Provider == ignoreProvider {
					ignored = true
					break
				}
			}
			if ignored {
				continue
			}
		}

		// filter by router name
		if len(e.ignoreRouters) > 0 {
			ignored := false
			for _, ignoreRouter := range e.ignoreRouters {
				if router.Name == ignoreRouter {
					ignored = true
					break
				}
			}
			if ignored {
				continue
			}
		}

		desired = append(desired, router)
	}

	desiredAliases, err := e.routersToHostAliases(desired)
	if err != nil {
		return nil, err
	}

	return desiredAliases, nil
}

func (e *Engine) routersToHostAliases(routers []traefik.Router) ([]model.HostAlias, error) {
	var aliases []model.HostAlias

	var parsedDomains []traefik.DomainMatch
	for _, router := range routers {
		domains, err := traefik.ParseDomains(router.Rule)
		if err != nil {
			return nil, err
		}

		parsedDomains = append(parsedDomains, domains...)
	}

	var plainDomains []string
	for _, domain := range parsedDomains {
		if domain.Kind == traefik.DomainLiteral {
			plainDomains = append(plainDomains, domain.Value)
		} else if domain.Kind == traefik.DomainRegex {
			generatedDomains, err := e.regexGenerator.Generate(domain.Value)
			if err != nil {
				log.Println("failed to generate domains from regex:", domain.Value, "error:", err)
				continue
			}
			plainDomains = append(plainDomains, generatedDomains...)
		}
	}

	for _, domain := range plainDomains {
		hostname, domain, found := strings.Cut(domain, ".")
		if !found || hostname == "" || domain == "" {
			log.Println("skipping invalid domain:", domain)
			continue
		}
		aliases = append(aliases, model.HostAlias{
			Hostname:    hostname,
			Domain:      domain,
			Description: e.descTag,
		})
	}

	return aliases, nil
}
