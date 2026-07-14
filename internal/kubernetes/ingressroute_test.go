package kubernetes

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type routeSpec struct {
	kind  string
	match string
}

func newIngressRoute(namespace, name string, routes []routeSpec) *unstructured.Unstructured {
	items := make([]any, 0, len(routes))
	for _, r := range routes {
		route := map[string]any{"match": r.match}
		if r.kind != "" {
			route["kind"] = r.kind
		}
		items = append(items, route)
	}
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "traefik.io/v1alpha1",
			"kind":       "IngressRoute",
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]any{
				"routes": items,
			},
		},
	}
}

func TestIngressRouteAliases(t *testing.T) {
	objs := []runtime.Object{
		newIngressRoute("default", "app", []routeSpec{
			{kind: "Rule", match: "Host(`app.example.com`) && !Host(`excluded.example.com`)"},
			{kind: "TCP", match: "HostSNI(`should-be-skipped.example.com`)"},
		}),
		newIngressRoute("default", "bare", []routeSpec{
			{match: "Host(`bare.example.com`)"},
		}),
	}
	client := newFakeDynamicClient(t, ingressRouteGVR, "IngressRouteList", objs...)

	s := &Source{client: client, descTag: "tag"}

	aliases, err := s.ingressRouteAliases(context.Background())
	if err != nil {
		t.Fatalf("ingressRouteAliases returned error: %v", err)
	}

	want := map[string]bool{
		"app.example.com":  true,
		"bare.example.com": true,
	}
	if len(aliases) != len(want) {
		t.Fatalf("got %d aliases, want %d: %+v", len(aliases), len(want), aliases)
	}
	for _, a := range aliases {
		key := a.Hostname + "." + a.Domain
		if !want[key] {
			t.Errorf("unexpected alias: %s", key)
		}
	}
}
