package devtools

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

//go:embed versions.json
var pinnedVersionsJSON []byte

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
		pinnedVersions, pinnedVersionsErr = parsePinnedVersions(pinnedVersionsJSON)
	})
	if pinnedVersionsErr != nil {
		panic(pinnedVersionsErr)
	}
	return pinnedVersions
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

func parsePinnedVersions(data []byte) (PinnedVersionsConfig, error) {
	var cfg PinnedVersionsConfig
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("parse internal devtool versions: %w", err)
	}
	if cfg.SchemaVersion != "onlava.internal.devtools.versions.v1" {
		return cfg, fmt.Errorf("unsupported internal devtool versions schema %q", cfg.SchemaVersion)
	}
	if strings.TrimSpace(cfg.Grafana.Version) == "" {
		return cfg, fmt.Errorf("internal devtool versions missing grafana.version")
	}
	if strings.TrimSpace(cfg.Victoria.Metrics.Version) == "" {
		return cfg, fmt.Errorf("internal devtool versions missing victoria.metrics.version")
	}
	if strings.TrimSpace(cfg.Victoria.Logs.Version) == "" {
		return cfg, fmt.Errorf("internal devtool versions missing victoria.logs.version")
	}
	if strings.TrimSpace(cfg.Victoria.Traces.Version) == "" {
		return cfg, fmt.Errorf("internal devtool versions missing victoria.traces.version")
	}
	for i, plugin := range cfg.Grafana.Plugins {
		if strings.TrimSpace(plugin.ID) == "" {
			return cfg, fmt.Errorf("internal devtool versions grafana.plugins[%d] missing id", i)
		}
		if strings.TrimSpace(plugin.Version) == "" {
			return cfg, fmt.Errorf("internal devtool versions grafana.plugins[%d] missing version", i)
		}
	}
	return cfg, nil
}
