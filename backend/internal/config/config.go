// Package config holds application configuration loaded from env/file.
package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/yamlwrangler/image-roundup/backend/internal/models"
)

// Config is the global runtime configuration.
type Config struct {
	// ListenAddr is the HTTP bind address, e.g. ":8080".
	ListenAddr string

	// InCluster indicates whether to use the in-cluster Kubernetes config.
	InCluster bool

	// KubeConfig is the path to a kubeconfig file (out-of-cluster only).
	KubeConfig string

	// DataDir is the directory used for persistent storage.
	// When empty, results are kept in memory only.
	DataDir string

	Settings models.Settings
}

// Load reads configuration from environment variables.
// All variables are optional; defaults are applied where missing.
func Load() Config {
	cfg := Config{
		ListenAddr: envOr("LISTEN_ADDR", ":8080"),
		InCluster:  envBool("IN_CLUSTER", true),
		KubeConfig: os.Getenv("KUBECONFIG"),
		DataDir:    os.Getenv("DATA_DIR"),
		Settings:   models.DefaultSettings(),
	}

	if v := envInt("SCAN_INTERVAL_SECONDS", 0); v > 0 {
		cfg.Settings.ScanIntervalSeconds = v
	}
	if v := envInt("REGISTRY_TIMEOUT_SECONDS", 0); v > 0 {
		cfg.Settings.RegistryTimeoutSeconds = v
	}
	if v := os.Getenv("INCLUDED_NAMESPACES"); v != "" {
		cfg.Settings.IncludedNamespaces = splitCSV(v)
	}
	if v := os.Getenv("EXCLUDED_NAMESPACES"); v != "" {
		cfg.Settings.ExcludedNamespaces = splitCSV(v)
	}
	if v := os.Getenv("THEME"); v != "" {
		cfg.Settings.Theme = v
	}
	if envBool("INCLUDE_COMPLETED_PODS", false) {
		cfg.Settings.IncludeCompletedPods = true
	}
	if envBool("EXCLUDE_INTERNAL_REGISTRY", false) {
		cfg.Settings.ExcludeInternalRegistry = true
	}
	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
