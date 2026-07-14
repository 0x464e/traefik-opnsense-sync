package kubernetes

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func newHTTPRoute(namespace, name string, hostnames []string) *unstructured.Unstructured {
	names := make([]any, 0, len(hostnames))
	for _, h := range hostnames {
		names = append(names, h)
	}
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "gateway.networking.k8s.io/v1",
			"kind":       "HTTPRoute",
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]any{
				"hostnames": names,
			},
		},
	}
}

func TestHTTPRouteAliases(t *testing.T) {
	objs := []runtime.Object{
		newHTTPRoute("default", "app", []string{"app.example.com", "*.wild.example.com"}),
		newHTTPRoute("default", "invalid", []string{"nodothost"}),
	}
	client := newFakeDynamicClient(t, httpRouteGVR, "HTTPRouteList", objs...)

	s := &Source{client: client, descTag: "tag"}

	aliases, err := s.httpRouteAliases(context.Background())
	if err != nil {
		t.Fatalf("httpRouteAliases returned error: %v", err)
	}

	if len(aliases) != 1 || aliases[0].Hostname != "app" || aliases[0].Domain != "example.com" {
		t.Fatalf("got %+v, want only app.example.com", aliases)
	}
	if aliases[0].Description != "tag" {
		t.Fatalf("description = %q, want %q", aliases[0].Description, "tag")
	}
}
