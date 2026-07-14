package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

type traefikCfg struct {
	BaseURL            string   `mapstructure:"base_url"`
	IncludeEntryPoints []string `mapstructure:"include_entrypoints"`
	IgnoreRouters      []string `mapstructure:"ignore_routers"`
	IncludeProviders   []string `mapstructure:"include_providers"`
	IgnoreProviders    []string `mapstructure:"ignore_providers"`
	Username           string   `mapstructure:"username"`
	Password           string   `mapstructure:"password"`
	VerifyTLS          bool     `mapstructure:"verify_tls"`
}

type opnSenseCfg struct {
	BaseURL      string `mapstructure:"base_url"`
	APIKey       string `mapstructure:"api_key"`
	APISecret    string `mapstructure:"api_secret"`
	HostOverride string `mapstructure:"host_override"`
	VerifyTLS    bool   `mapstructure:"verify_tls"`
}

type regexCfg struct {
	MaxGenerated int    `mapstructure:"max_generated"`
	ExrexPath    string `mapstructure:"exrex_path"`
}

type syncCfg struct {
	Source         string        `mapstructure:"source"`
	Interval       time.Duration `mapstructure:"interval"`
	DescriptionTag string        `mapstructure:"description_tag"`
	DryRun         bool          `mapstructure:"dry_run"`
}

type kubernetesCfg struct {
	Resources         []string `mapstructure:"resources"`
	IncludeNamespaces []string `mapstructure:"include_namespaces"`
	IgnoreNamespaces  []string `mapstructure:"ignore_namespaces"`
	IgnoreResources   []string `mapstructure:"ignore_resources"`
}

type Config struct {
	Traefik    traefikCfg    `mapstructure:"traefik"`
	OPNsense   opnSenseCfg   `mapstructure:"opnsense"`
	Regex      regexCfg      `mapstructure:"regex"`
	Sync       syncCfg       `mapstructure:"sync"`
	Kubernetes kubernetesCfg `mapstructure:"kubernetes"`
}

const (
	SourceTraefik    = "traefik"
	SourceKubernetes = "kubernetes"
)

const (
	ResourceIngress      = "ingress"
	ResourceIngressRoute = "ingressroute"
	ResourceHTTPRoute    = "httproute"
)

var validKubernetesResources = []string{ResourceIngress, ResourceIngressRoute, ResourceHTTPRoute}

func LoadConfig() (Config, error) {
	v := viper.NewWithOptions(
		viper.ExperimentalBindStruct(),
		viper.EnvKeyReplacer(strings.NewReplacer(".", "_")),
	)
	v.SetEnvPrefix("TOS")
	v.AutomaticEnv()

	setDefaults(v)

	// *_FILE secrets available in env
	ingestSecretFilesIntoEnv()

	// resolve config file path
	var cfgPath string
	if env := strings.TrimSpace(os.Getenv("TOS_CONFIG")); env != "" {
		cfgPath = env
	} else {
		cfgPath = findDefaultConfigFile()
	}

	if cfgPath != "" {
		if abs, err := absoluteIfRelative(cfgPath); err == nil {
			if _, statErr := os.Stat(abs); statErr == nil {
				v.SetConfigFile(abs)
				if err := v.ReadInConfig(); err != nil {
					return Config{}, fmt.Errorf("read config: %w", err)
				}
			}
		}
	}

	// unmarshal with hooks: durations and CSV -> []string
	var cfg Config
	decodeHooks := mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
	)
	if err := v.Unmarshal(&cfg, viper.DecodeHook(decodeHooks)); err != nil {
		return Config{}, fmt.Errorf("unmarshal: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	// Traefik
	v.SetDefault("traefik.verify_tls", true)

	// OPNsense
	v.SetDefault("opnsense.verify_tls", true)

	// regex
	v.SetDefault("regex.max_generated", 5)
	v.SetDefault("regex.exrex_path", "exrex")

	// sync
	v.SetDefault("sync.source", SourceTraefik)
	v.SetDefault("sync.dry_run", false)
	v.SetDefault("sync.interval", "30s")
	v.SetDefault("sync.description_tag", "Managed by traefik-opnsense-sync")

	// kubernetes
	v.SetDefault("kubernetes.resources", []string{"ingress"})
}

// read TOS_*_FILE envs and set the corresponding TOS_* env with the file contents
func ingestSecretFilesIntoEnv() {
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]

		// only consider TOS_*_FILE keys
		if !strings.HasPrefix(key, "TOS_") || !strings.HasSuffix(key, "_FILE") {
			continue
		}

		path := strings.TrimSpace(val)
		if path == "" {
			continue
		}

		// target env var name without _FILE suffix, e.g. TOS_OPNSENSE_API_SECRET
		target := strings.TrimSuffix(key, "_FILE")

		// if the target env is already set, do not override
		if _, exists := os.LookupEnv(target); exists {
			continue
		}

		// ensure it's a regular file
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}

		file, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		secret := strings.TrimSpace(string(file))

		_ = os.Setenv(target, secret)
	}
}

func validate(config *Config) error {
	var errs []string

	// required fields
	switch config.Sync.Source {
	case SourceTraefik:
		if strings.TrimSpace(config.Traefik.BaseURL) == "" {
			errs = append(errs, "traefik.base_url is required")
		}
	case SourceKubernetes:
		if err := validateKubernetesResources(config.Kubernetes.Resources); err != nil {
			errs = append(errs, err.Error())
		}
		if err := validateIgnoreResources(config.Kubernetes.IgnoreResources); err != nil {
			errs = append(errs, err.Error())
		}
		if len(config.Kubernetes.IncludeNamespaces) > 0 && len(config.Kubernetes.IgnoreNamespaces) > 0 {
			errs = append(errs, "kubernetes.include_namespaces and kubernetes.ignore_namespaces are mutually exclusive")
		}
	default:
		errs = append(errs, fmt.Sprintf("sync.source must be one of %q or %q, got %q", SourceTraefik, SourceKubernetes, config.Sync.Source))
	}

	if strings.TrimSpace(config.OPNsense.BaseURL) == "" {
		errs = append(errs, "opnsense.base_url is required")
	}
	if strings.TrimSpace(config.OPNsense.APIKey) == "" {
		errs = append(errs, "opnsense.api_key is required")
	}
	if strings.TrimSpace(config.OPNsense.APISecret) == "" {
		errs = append(errs, "opnsense.api_secret is required")
	}
	if strings.TrimSpace(config.OPNsense.HostOverride) == "" {
		errs = append(errs, "opnsense.host_override is required")
	}
	if strings.TrimSpace(config.Sync.DescriptionTag) == "" {
		errs = append(errs, "sync.description_tag is required")
	}

	// guardrails
	if config.Regex.MaxGenerated <= 0 {
		errs = append(errs, "regex.max_generated must be > 0")
	}
	if config.Sync.Interval <= 0 {
		errs = append(errs, "sync.interval must be > 0")
	}
	if err := validateIgnoreRouters(config.Traefik.IgnoreRouters); err != nil {
		errs = append(errs, err.Error())
	}
	if len(config.Traefik.IgnoreProviders) > 0 && len(config.Traefik.IncludeProviders) > 0 {
		errs = append(errs, "traefik.ignore_providers and traefik.include_providers are mutually exclusive")
	}

	if config.Sync.Source == SourceTraefik &&
		len(config.Traefik.IncludeEntryPoints) == 0 && len(config.Traefik.IgnoreRouters) == 0 &&
		len(config.Traefik.IncludeProviders) == 0 && len(config.Traefik.IgnoreProviders) == 0 {
		log.Println("[Warning] No Traefik filters configured; all routers will be considered for synchronization. " +
			"If this is not intended, configure at least one of traefik.include_entrypoints, traefik.ignore_routers, traefik.include_providers or traefik.ignore_providers.")
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid configuration:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

func validateIgnoreRouters(routers []string) error {
	for _, router := range routers {
		if !strings.Contains(router, "@") {
			return fmt.Errorf("ignore_routers entry %q must include provider suffix, e.g. 'router@docker' or 'router@file'", router)
		}
	}
	return nil
}

func validateKubernetesResources(resources []string) error {
	if len(resources) == 0 {
		return fmt.Errorf("kubernetes.resources must not be empty when sync.source is %q", SourceKubernetes)
	}
	for _, resource := range resources {
		valid := false
		for _, allowed := range validKubernetesResources {
			if resource == allowed {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("kubernetes.resources entry %q must be one of %v", resource, validKubernetesResources)
		}
	}
	return nil
}

func validateIgnoreResources(resources []string) error {
	for _, resource := range resources {
		if !strings.Contains(resource, "@") {
			return fmt.Errorf("kubernetes.ignore_resources entry %q must include kind suffix, e.g. 'my-ingress.default@ingress'", resource)
		}
	}
	return nil
}

func absoluteIfRelative(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, path), nil
}

func findDefaultConfigFile() string {
	candidates := []string{
		"config.yaml",
		"config.yml",
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}
