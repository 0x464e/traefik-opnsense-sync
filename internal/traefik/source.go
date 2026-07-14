package traefik

import (
	"context"
	"log"

	"github.com/0x464e/traefik-opnsense-sync/internal/config"
	"github.com/0x464e/traefik-opnsense-sync/internal/exrex"
	"github.com/0x464e/traefik-opnsense-sync/internal/model"
)

// Source produces the desired set of DNS host aliases from Traefik's router
// configuration, applying the configured entrypoint/provider/router filters.
type Source struct {
	client             Client
	regexGenerator     *exrex.Exrex
	includeEntryPoints []string
	ignoreRouters      []string
	includeProviders   []string
	ignoreProviders    []string
	descTag            string
}

func NewSource(cfg *config.Config) *Source {
	return &Source{
		client:             NewClient(cfg.Traefik.BaseURL, cfg.Traefik.VerifyTLS, cfg.Traefik.Username, cfg.Traefik.Password),
		regexGenerator:     exrex.NewExrexRunner(cfg),
		includeEntryPoints: cfg.Traefik.IncludeEntryPoints,
		ignoreRouters:      cfg.Traefik.IgnoreRouters,
		includeProviders:   cfg.Traefik.IncludeProviders,
		ignoreProviders:    cfg.Traefik.IgnoreProviders,
		descTag:            cfg.Sync.DescriptionTag,
	}
}

func (s *Source) DesiredAliases(ctx context.Context) ([]model.HostAlias, error) {
	routers, err := s.client.GetRouters(ctx)
	if err != nil {
		return nil, err
	}

	return s.desiredFromRouters(routers)
}

func (s *Source) desiredFromRouters(routers []Router) ([]model.HostAlias, error) {
	var desired []Router

	for _, router := range routers {
		// filter by entrypoints
		if len(s.includeEntryPoints) > 0 {
			matched := false
			for _, ep := range router.EntryPoints {
				for _, includeEp := range s.includeEntryPoints {
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
		if len(s.includeProviders) > 0 {
			matched := false
			for _, includeProvider := range s.includeProviders {
				if router.Provider == includeProvider {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if len(s.ignoreProviders) > 0 {
			ignored := false
			for _, ignoreProvider := range s.ignoreProviders {
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
		if len(s.ignoreRouters) > 0 {
			ignored := false
			for _, ignoreRouter := range s.ignoreRouters {
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

	return s.routersToHostAliases(desired)
}

func (s *Source) routersToHostAliases(routers []Router) ([]model.HostAlias, error) {
	var parsedDomains []DomainMatch
	for _, router := range routers {
		domains, err := ParseDomains(router.Rule)
		if err != nil {
			return nil, err
		}

		parsedDomains = append(parsedDomains, domains...)
	}

	return DomainsToHostAliases(parsedDomains, s.regexGenerator, s.descTag), nil
}

// DomainsToHostAliases expands regex domain matches via regexGenerator and
// converts every resulting literal FQDN into a model.HostAlias tagged with
// descTag. It's shared with the internal/kubernetes IngressRoute source,
// since a Traefik IngressRoute CRD's "match" field is the same rule syntax
// as router.Rule.
func DomainsToHostAliases(domains []DomainMatch, regexGenerator *exrex.Exrex, descTag string) []model.HostAlias {
	var plainDomains []string
	for _, domain := range domains {
		if domain.Kind == DomainLiteral {
			plainDomains = append(plainDomains, domain.Value)
		} else if domain.Kind == DomainRegex {
			generatedDomains, err := regexGenerator.Generate(domain.Value)
			if err != nil {
				log.Println("failed to generate domains from regex:", domain.Value, "error:", err)
				continue
			}
			plainDomains = append(plainDomains, generatedDomains...)
		}
	}

	var aliases []model.HostAlias
	for _, plainDomain := range plainDomains {
		alias, ok := model.NewHostAliasFromFQDN(plainDomain, descTag)
		if !ok {
			log.Println("skipping invalid domain:", plainDomain)
			continue
		}
		aliases = append(aliases, alias)
	}

	return aliases
}
