package kubernetes

import (
	"context"
	"log"
	"strings"

	"github.com/0x464e/traefik-opnsense-sync/internal/config"
	"github.com/0x464e/traefik-opnsense-sync/internal/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var httpRouteGVR = schema.GroupVersionResource{
	Group:    "gateway.networking.k8s.io",
	Version:  "v1",
	Resource: "httproutes",
}

// httpRouteAliases reads spec.hostnames from Gateway API HTTPRoute objects.
func (s *Source) httpRouteAliases(ctx context.Context) ([]model.HostAlias, error) {
	items, err := s.listResources(ctx, httpRouteGVR)
	if err != nil {
		return nil, err
	}

	var aliases []model.HostAlias
	for _, item := range items {
		if s.resourceIgnored(item.GetName(), item.GetNamespace(), config.ResourceHTTPRoute) {
			continue
		}

		hostnames, _, err := unstructured.NestedStringSlice(item.Object, "spec", "hostnames")
		if err != nil {
			log.Printf("skipping httproute %s/%s: %v", item.GetNamespace(), item.GetName(), err)
			continue
		}

		for _, host := range hostnames {
			if strings.HasPrefix(host, "*.") {
				log.Println("skipping wildcard httproute hostname:", host)
				continue
			}

			alias, ok := model.NewHostAliasFromFQDN(host, s.descTag)
			if !ok {
				log.Println("skipping invalid domain:", host)
				continue
			}
			aliases = append(aliases, alias)
		}
	}

	return aliases, nil
}
