package kubernetes

import (
	"fmt"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// newDynamicClient connects to the Kubernetes API using the pod's in-cluster
// service account credentials. Running outside a cluster (no kubeconfig
// fallback) is not supported.
func newDynamicClient() (dynamic.Interface, error) {
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("load in-cluster kubernetes config: %w", err)
	}

	client, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("build kubernetes dynamic client: %w", err)
	}

	return client, nil
}
