package kubernetes

import (
	"context"
	"log"

	"github.com/0x464e/traefik-opnsense-sync/internal/config"
	"github.com/0x464e/traefik-opnsense-sync/internal/model"
	"github.com/0x464e/traefik-opnsense-sync/internal/traefik"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var ingressRouteGVR = schema.GroupVersionResource{
	Group:    "traefik.io",
	Version:  "v1alpha1",
	Resource: "ingressroutes",
}

// ingressRouteAliases reads spec.routes[].match from Traefik IngressRoute
// CRDs. The match string is the same rule syntax as a Traefik router's Rule
// field, so it's parsed with traefik.ParseDomains and expanded the same way.
func (s *Source) ingressRouteAliases(ctx context.Context) ([]model.HostAlias, error) {
	items, err := s.listResources(ctx, ingressRouteGVR)
	if err != nil {
		return nil, err
	}

	var aliases []model.HostAlias
	for _, item := range items {
		if s.resourceIgnored(item.GetName(), item.GetNamespace(), config.ResourceIngressRoute) {
			continue
		}

		routes, _, err := unstructured.NestedSlice(item.Object, "spec", "routes")
		if err != nil {
			log.Printf("skipping ingressroute %s/%s: %v", item.GetNamespace(), item.GetName(), err)
			continue
		}

		var matches []string
		for _, route := range routes {
			routeMap, ok := route.(map[string]any)
			if !ok {
				continue
			}

			// non-HTTP routes (TCP/UDP) carry no Host()/HostRegexp() semantics
			kind, _, _ := unstructured.NestedString(routeMap, "kind")
			if kind != "" && kind != "Rule" {
				continue
			}

			match, _, _ := unstructured.NestedString(routeMap, "match")
			if match == "" {
				continue
			}
			matches = append(matches, match)
		}

		routeAliases, err := s.matchesToHostAliases(matches)
		if err != nil {
			log.Printf("skipping ingressroute %s/%s: %v", item.GetNamespace(), item.GetName(), err)
			continue
		}
		aliases = append(aliases, routeAliases...)
	}

	return aliases, nil
}

func (s *Source) matchesToHostAliases(matches []string) ([]model.HostAlias, error) {
	var parsedDomains []traefik.DomainMatch
	for _, match := range matches {
		domains, err := traefik.ParseDomains(match)
		if err != nil {
			return nil, err
		}
		parsedDomains = append(parsedDomains, domains...)
	}

	return traefik.DomainsToHostAliases(parsedDomains, s.regexGenerator, s.descTag), nil
}
