package kubernetes

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// listResources lists every object of the given kind cluster-wide, then
// filters out anything excluded by the configured namespace filters. Objects
// ignored by name (kubernetes.ignore_resources) are the caller's
// responsibility, since that check needs the resource kind string.
func (s *Source) listResources(ctx context.Context, gvr schema.GroupVersionResource) ([]unstructured.Unstructured, error) {
	list, err := s.client.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var items []unstructured.Unstructured
	for _, item := range list.Items {
		if !s.namespaceAllowed(item.GetNamespace()) {
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

func (s *Source) namespaceAllowed(namespace string) bool {
	if len(s.includeNamespaces) > 0 {
		for _, ns := range s.includeNamespaces {
			if ns == namespace {
				return true
			}
		}
		return false
	}

	if len(s.ignoreNamespaces) > 0 {
		for _, ns := range s.ignoreNamespaces {
			if ns == namespace {
				return false
			}
		}
	}

	return true
}

func (s *Source) resourceIgnored(name, namespace, kind string) bool {
	target := name + "." + namespace + "@" + kind
	for _, ignored := range s.ignoreResources {
		if ignored == target {
			return true
		}
	}
	return false
}
