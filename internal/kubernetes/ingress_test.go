package kubernetes

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func newIngress(namespace, name string, hosts []string) *unstructured.Unstructured {
	rules := make([]any, 0, len(hosts))
	for _, host := range hosts {
		rules = append(rules, map[string]any{"host": host})
	}
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "networking.k8s.io/v1",
			"kind":       "Ingress",
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]any{
				"rules": rules,
			},
		},
	}
}

func newFakeDynamicClient(t *testing.T, gvr schema.GroupVersionResource, listKind string, objs ...runtime.Object) dynamic.Interface {
	t.Helper()
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{gvr: listKind},
		objs...,
	)
}

func TestIngressAliases(t *testing.T) {
	objs := []runtime.Object{
		newIngress("default", "app", []string{"app.example.com", "*.wild.example.com"}),
		newIngress("kube-system", "sys", []string{"sys.example.com"}),
		newIngress("default", "invalid", []string{"nodothost"}),
	}
	client := newFakeDynamicClient(t, ingressGVR, "IngressList", objs...)

	s := &Source{client: client, descTag: "tag"}

	aliases, err := s.ingressAliases(context.Background())
	if err != nil {
		t.Fatalf("ingressAliases returned error: %v", err)
	}

	want := map[string]bool{
		"app.example.com": true,
		"sys.example.com": true,
	}
	if len(aliases) != len(want) {
		t.Fatalf("got %d aliases, want %d: %+v", len(aliases), len(want), aliases)
	}
	for _, a := range aliases {
		key := a.Hostname + "." + a.Domain
		if !want[key] {
			t.Errorf("unexpected alias: %s", key)
		}
		if a.Description != "tag" {
			t.Errorf("alias %s: description = %q, want %q", key, a.Description, "tag")
		}
	}
}

func TestIngressAliasesNamespaceFilter(t *testing.T) {
	objs := []runtime.Object{
		newIngress("default", "app", []string{"app.example.com"}),
		newIngress("kube-system", "sys", []string{"sys.example.com"}),
	}
	client := newFakeDynamicClient(t, ingressGVR, "IngressList", objs...)

	s := &Source{client: client, descTag: "tag", includeNamespaces: []string{"default"}}

	aliases, err := s.ingressAliases(context.Background())
	if err != nil {
		t.Fatalf("ingressAliases returned error: %v", err)
	}
	if len(aliases) != 1 || aliases[0].Hostname != "app" {
		t.Fatalf("got %+v, want only app.example.com", aliases)
	}
}

func TestIngressAliasesIgnoreResources(t *testing.T) {
	objs := []runtime.Object{
		newIngress("default", "app", []string{"app.example.com"}),
	}
	client := newFakeDynamicClient(t, ingressGVR, "IngressList", objs...)

	s := &Source{client: client, descTag: "tag", ignoreResources: []string{"app.default@ingress"}}

	aliases, err := s.ingressAliases(context.Background())
	if err != nil {
		t.Fatalf("ingressAliases returned error: %v", err)
	}
	if len(aliases) != 0 {
		t.Fatalf("got %+v, want no aliases", aliases)
	}
}

func TestNamespaceAllowed(t *testing.T) {
	tests := []struct {
		name              string
		includeNamespaces []string
		ignoreNamespaces  []string
		namespace         string
		want              bool
	}{
		{name: "no filters", namespace: "default", want: true},
		{name: "include match", includeNamespaces: []string{"default"}, namespace: "default", want: true},
		{name: "include no match", includeNamespaces: []string{"default"}, namespace: "other", want: false},
		{name: "ignore match", ignoreNamespaces: []string{"kube-system"}, namespace: "kube-system", want: false},
		{name: "ignore no match", ignoreNamespaces: []string{"kube-system"}, namespace: "default", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Source{includeNamespaces: tt.includeNamespaces, ignoreNamespaces: tt.ignoreNamespaces}
			if got := s.namespaceAllowed(tt.namespace); got != tt.want {
				t.Errorf("namespaceAllowed(%q) = %v, want %v", tt.namespace, got, tt.want)
			}
		})
	}
}

func TestResourceIgnored(t *testing.T) {
	s := &Source{ignoreResources: []string{"app.default@ingress"}}

	if !s.resourceIgnored("app", "default", "ingress") {
		t.Error("expected app.default@ingress to be ignored")
	}
	if s.resourceIgnored("app", "other", "ingress") {
		t.Error("did not expect app.other@ingress to be ignored")
	}
	if s.resourceIgnored("app", "default", "httproute") {
		t.Error("did not expect app.default@httproute to be ignored")
	}
}
