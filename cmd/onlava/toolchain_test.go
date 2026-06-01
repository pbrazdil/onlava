package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseToolchainArgs(t *testing.T) {
	t.Parallel()

	opts, err := parseToolchainArgs([]string{"verify", "--json", "--tool", "grafana", "--platform", "linux/amd64", "--images", "--strict"})
	if err != nil {
		t.Fatalf("parseToolchainArgs() error = %v", err)
	}
	if opts.Command != "verify" || !opts.JSON || opts.Tool != "grafana" || opts.Platform.String() != "linux/amd64" || !opts.Images || !opts.Strict {
		t.Fatalf("opts = %+v", opts)
	}
	if _, err := parseToolchainArgs([]string{"path"}); err == nil {
		t.Fatal("expected path without --tool to fail")
	}
}

func TestRunToolchainListJSON(t *testing.T) {
	t.Setenv("ONLAVA_TOOLCHAIN_DIR", t.TempDir())
	var out bytes.Buffer
	if err := runToolchain(t.Context(), &out, []string{"list", "--json"}); err != nil {
		t.Fatalf("runToolchain list: %v", err)
	}
	var payload struct {
		SchemaVersion string `json:"schema_version"`
		Artifacts     []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"artifacts"`
		SourceLocks []struct {
			Name string `json:"name"`
		} `json:"source_locks"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v\n%s", err, out.String())
	}
	if payload.SchemaVersion != "onlava.toolchain.status.v1" {
		t.Fatalf("schema_version = %q", payload.SchemaVersion)
	}
	if len(payload.Artifacts) == 0 || len(payload.SourceLocks) == 0 {
		t.Fatalf("payload missing artifacts or source locks: %+v", payload)
	}
}

func TestVersionJSONIncludesToolchainManifest(t *testing.T) {
	t.Parallel()

	resp := buildVersionResponse()
	if resp.Toolchain == nil {
		t.Fatal("toolchain manifest metadata missing")
	}
	if resp.Toolchain.SchemaVersion != "onlava.toolchain.v1" {
		t.Fatalf("toolchain schema = %q", resp.Toolchain.SchemaVersion)
	}
	if len(resp.Toolchain.SHA256) != 64 {
		t.Fatalf("toolchain sha = %q", resp.Toolchain.SHA256)
	}
}

func TestRunToolchainPathJSON(t *testing.T) {
	t.Setenv("ONLAVA_TOOLCHAIN_DIR", t.TempDir())
	var out bytes.Buffer
	if err := runToolchain(t.Context(), &out, []string{"path", "--json", "--tool", "grafana"}); err != nil {
		t.Fatalf("runToolchain path: %v", err)
	}
	if !strings.Contains(out.String(), `"managed_path"`) {
		t.Fatalf("path output missing managed_path: %s", out.String())
	}
}

func TestRunToolchainStrictImagesRejectsTagOnlyRefs(t *testing.T) {
	t.Setenv("ONLAVA_TOOLCHAIN_DIR", t.TempDir())
	var out bytes.Buffer
	err := runToolchain(t.Context(), &out, []string{"verify", "--json", "--tool", "postgres", "--images", "--strict"})
	if err == nil {
		t.Fatal("expected strict tag-only image verification to fail")
	}
	if !strings.Contains(out.String(), `"status": "invalid"`) {
		t.Fatalf("strict image output missing invalid status: %s", out.String())
	}
}
