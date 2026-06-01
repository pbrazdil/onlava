package devtools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/pbrazdil/onlava/internal/toolchain"
)

type PinnedVersionsConfig struct {
	SchemaVersion string           `json:"schema_version"`
	Grafana       GrafanaVersions  `json:"grafana"`
	Victoria      VictoriaVersions `json:"victoria"`
}

type GrafanaVersions struct {
	Version string          `json:"version"`
	Plugins []GrafanaPlugin `json:"plugins"`
}

type GrafanaPlugin struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type VictoriaVersions struct {
	Metrics VersionPin `json:"metrics"`
	Logs    VersionPin `json:"logs"`
	Traces  VersionPin `json:"traces"`
}

type VersionPin struct {
	Version string `json:"version"`
}

var (
	pinnedVersionsOnce sync.Once
	pinnedVersions     PinnedVersionsConfig
	pinnedVersionsErr  error
)

func PinnedVersions() PinnedVersionsConfig {
	pinnedVersionsOnce.Do(func() {
		pinnedVersions, pinnedVersionsErr = pinnedVersionsFromToolchainManifest()
	})
	if pinnedVersionsErr != nil {
		panic(pinnedVersionsErr)
	}
	return pinnedVersions
}

func pinnedVersionsFromToolchainManifest() (PinnedVersionsConfig, error) {
	manifest, err := toolchain.LoadBundledManifest()
	if err != nil {
		return PinnedVersionsConfig{}, err
	}
	cfg := PinnedVersionsConfig{
		SchemaVersion: "onlava.internal.devtools.versions.v1",
	}
	for _, artifact := range manifest.Artifacts {
		switch artifact.Name {
		case "grafana":
			cfg.Grafana.Version = artifact.Version
		case "victoria-metrics":
			cfg.Victoria.Metrics.Version = artifact.Version
		case "victoria-logs":
			cfg.Victoria.Logs.Version = artifact.Version
		case "victoria-traces":
			cfg.Victoria.Traces.Version = artifact.Version
		default:
			if artifact.Kind == "plugin" {
				cfg.Grafana.Plugins = append(cfg.Grafana.Plugins, GrafanaPlugin{
					ID:      artifact.Name,
					Version: artifact.Version,
				})
			}
		}
	}
	if err := validatePinnedVersions(cfg); err != nil {
		return PinnedVersionsConfig{}, err
	}
	return cfg, nil
}

func GrafanaPluginPreinstallSync() string {
	cfg := PinnedVersions().Grafana
	plugins := make([]string, 0, len(cfg.Plugins))
	for _, plugin := range cfg.Plugins {
		id := strings.TrimSpace(plugin.ID)
		version := strings.TrimSpace(plugin.Version)
		if id == "" {
			continue
		}
		if version != "" {
			plugins = append(plugins, id+"@"+version)
		} else {
			plugins = append(plugins, id)
		}
	}
	return strings.Join(plugins, ",")
}

func ParsePinnedVersions(data []byte) (PinnedVersionsConfig, error) {
	var cfg PinnedVersionsConfig
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("parse internal devtool versions: %w", err)
	}
	if err := validatePinnedVersions(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func validatePinnedVersions(cfg PinnedVersionsConfig) error {
	if cfg.SchemaVersion != "onlava.internal.devtools.versions.v1" {
		return fmt.Errorf("unsupported internal devtool versions schema %q", cfg.SchemaVersion)
	}
	if strings.TrimSpace(cfg.Grafana.Version) == "" {
		return fmt.Errorf("internal devtool versions missing grafana.version")
	}
	if strings.TrimSpace(cfg.Victoria.Metrics.Version) == "" {
		return fmt.Errorf("internal devtool versions missing victoria.metrics.version")
	}
	if strings.TrimSpace(cfg.Victoria.Logs.Version) == "" {
		return fmt.Errorf("internal devtool versions missing victoria.logs.version")
	}
	if strings.TrimSpace(cfg.Victoria.Traces.Version) == "" {
		return fmt.Errorf("internal devtool versions missing victoria.traces.version")
	}
	for i, plugin := range cfg.Grafana.Plugins {
		if strings.TrimSpace(plugin.ID) == "" {
			return fmt.Errorf("internal devtool versions grafana.plugins[%d] missing id", i)
		}
		if strings.TrimSpace(plugin.Version) == "" {
			return fmt.Errorf("internal devtool versions grafana.plugins[%d] missing version", i)
		}
	}
	return nil
}
