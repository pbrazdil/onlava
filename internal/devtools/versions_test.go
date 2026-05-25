package devtools

import "testing"

func TestPinnedVersionsConfig(t *testing.T) {
	cfg := PinnedVersions()
	if cfg.Grafana.Version != "13.0.1+security-01" {
		t.Fatalf("grafana version = %q", cfg.Grafana.Version)
	}
	if cfg.Victoria.Metrics.Version != "v1.141.0" {
		t.Fatalf("victoria metrics version = %q", cfg.Victoria.Metrics.Version)
	}
	if cfg.Victoria.Logs.Version != "v1.50.0" {
		t.Fatalf("victoria logs version = %q", cfg.Victoria.Logs.Version)
	}
	if cfg.Victoria.Traces.Version != "v0.8.1" {
		t.Fatalf("victoria traces version = %q", cfg.Victoria.Traces.Version)
	}
}

func TestGrafanaPluginPreinstallSyncPinsVersions(t *testing.T) {
	got := GrafanaPluginPreinstallSync()
	want := "victoriametrics-metrics-datasource@0.24.0,victoriametrics-logs-datasource@0.27.1"
	if got != want {
		t.Fatalf("GrafanaPluginPreinstallSync = %q, want %q", got, want)
	}
}

func TestPinnedVersionsRejectsMissingValues(t *testing.T) {
	_, err := parsePinnedVersions([]byte(`{
		"schema_version": "onlava.internal.devtools.versions.v1",
		"grafana": {
			"version": "",
			"plugins": []
		},
		"victoria": {
			"metrics": {"version": "v1"},
			"logs": {"version": "v2"},
			"traces": {"version": "v3"}
		}
	}`))
	if err == nil {
		t.Fatal("expected missing grafana version error")
	}
}
