package config

import "testing"

func baseValidConfig() *Config {
	return &Config{
		Traefik:  traefikCfg{BaseURL: "http://traefik.example.com"},
		OPNsense: opnSenseCfg{BaseURL: "https://opnsense.example.com", APIKey: "key", APISecret: "secret", HostOverride: "proxy.example.com"},
		Regex:    regexCfg{MaxGenerated: 5},
		Sync:     syncCfg{Source: SourceTraefik, Interval: 30, DescriptionTag: "tag"},
	}
}

func TestValidateSource(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{
			name:    "valid traefik source",
			mutate:  func(c *Config) {},
			wantErr: false,
		},
		{
			name: "missing traefik base_url",
			mutate: func(c *Config) {
				c.Traefik.BaseURL = ""
			},
			wantErr: true,
		},
		{
			name: "unknown source",
			mutate: func(c *Config) {
				c.Sync.Source = "nonsense"
			},
			wantErr: true,
		},
		{
			name: "kubernetes source defaults to no resources is invalid",
			mutate: func(c *Config) {
				c.Sync.Source = SourceKubernetes
				c.Kubernetes.Resources = nil
			},
			wantErr: true,
		},
		{
			name: "kubernetes source with valid resources does not require traefik.base_url",
			mutate: func(c *Config) {
				c.Sync.Source = SourceKubernetes
				c.Kubernetes.Resources = []string{ResourceIngress}
				c.Traefik.BaseURL = ""
			},
			wantErr: false,
		},
		{
			name: "kubernetes source with unknown resource kind",
			mutate: func(c *Config) {
				c.Sync.Source = SourceKubernetes
				c.Kubernetes.Resources = []string{"deployment"}
			},
			wantErr: true,
		},
		{
			name: "kubernetes namespaces mutually exclusive",
			mutate: func(c *Config) {
				c.Sync.Source = SourceKubernetes
				c.Kubernetes.Resources = []string{ResourceIngress}
				c.Kubernetes.IncludeNamespaces = []string{"default"}
				c.Kubernetes.IgnoreNamespaces = []string{"kube-system"}
			},
			wantErr: true,
		},
		{
			name: "kubernetes ignore_resources missing kind suffix",
			mutate: func(c *Config) {
				c.Sync.Source = SourceKubernetes
				c.Kubernetes.Resources = []string{ResourceIngress}
				c.Kubernetes.IgnoreResources = []string{"app.default"}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseValidConfig()
			tt.mutate(cfg)
			err := validate(cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
