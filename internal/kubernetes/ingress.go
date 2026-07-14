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

var ingressGVR = schema.GroupVersionResource{
	Group:    "networking.k8s.io",
	Version:  "v1",
	Resource: "ingresses",
}

// ingressAliases reads spec.rules[].host from core Ingress objects.
func (s *Source) ingressAliases(ctx context.Context) ([]model.HostAlias, error) {
	items, err := s.listResources(ctx, ingressGVR)
	if err != nil {
		return nil, err
	}

	var aliases []model.HostAlias
	for _, item := range items {
		if s.resourceIgnored(item.GetName(), item.GetNamespace(), config.ResourceIngress) {
			continue
		}

		rules, _, err := unstructured.NestedSlice(item.Object, "spec", "rules")
		if err != nil {
			log.Printf("skipping ingress %s/%s: %v", item.GetNamespace(), item.GetName(), err)
			continue
		}

		for _, rule := range rules {
			ruleMap, ok := rule.(map[string]any)
			if !ok {
				continue
			}

			host, _, _ := unstructured.NestedString(ruleMap, "host")
			if host == "" {
				continue
			}
			if strings.HasPrefix(host, "*.") {
				log.Println("skipping wildcard ingress host:", host)
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
