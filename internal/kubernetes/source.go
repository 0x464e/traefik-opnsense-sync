package kubernetes

import (
	"context"
	"fmt"

	"github.com/0x464e/traefik-opnsense-sync/internal/config"
	"github.com/0x464e/traefik-opnsense-sync/internal/exrex"
	"github.com/0x464e/traefik-opnsense-sync/internal/model"
	"k8s.io/client-go/dynamic"
)

// Source produces the desired set of DNS host aliases from Kubernetes
// Ingress, Traefik IngressRoute and/or Gateway API HTTPRoute resources,
// applying the configured namespace/resource filters.
type Source struct {
	client            dynamic.Interface
	regexGenerator    *exrex.Exrex
	resources         []string
	includeNamespaces []string
	ignoreNamespaces  []string
	ignoreResources   []string
	descTag           string
}

func NewSource(cfg *config.Config) (*Source, error) {
	client, err := newDynamicClient()
	if err != nil {
		return nil, err
	}

	return &Source{
		client:            client,
		regexGenerator:    exrex.NewExrexRunner(cfg),
		resources:         cfg.Kubernetes.Resources,
		includeNamespaces: cfg.Kubernetes.IncludeNamespaces,
		ignoreNamespaces:  cfg.Kubernetes.IgnoreNamespaces,
		ignoreResources:   cfg.Kubernetes.IgnoreResources,
		descTag:           cfg.Sync.DescriptionTag,
	}, nil
}

func (s *Source) DesiredAliases(ctx context.Context) ([]model.HostAlias, error) {
	var aliases []model.HostAlias

	for _, resource := range s.resources {
		var (
			resourceAliases []model.HostAlias
			err             error
		)

		switch resource {
		case config.ResourceIngress:
			resourceAliases, err = s.ingressAliases(ctx)
		case config.ResourceIngressRoute:
			resourceAliases, err = s.ingressRouteAliases(ctx)
		case config.ResourceHTTPRoute:
			resourceAliases, err = s.httpRouteAliases(ctx)
		default:
			return nil, fmt.Errorf("unknown kubernetes resource kind: %q", resource)
		}
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", resource, err)
		}

		aliases = append(aliases, resourceAliases...)
	}

	return aliases, nil
}
