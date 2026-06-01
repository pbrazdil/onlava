package envpolicy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryFindExactPrefixAndGlob(t *testing.T) {
	registry := &Registry{
		SchemaVersion: SchemaVersion,
		Variables: []Variable{
			testVariable("ONLAVA_APP_ID", "exact"),
			testVariable("ONLAVA_TEST_", "prefix"),
			testVariable("ONLAVA_FRONTEND_*_ADDR", "glob"),
		},
	}
	if err := registry.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	registry.index()
	for _, name := range []string{"ONLAVA_APP_ID", "ONLAVA_TEST_HELPER", "ONLAVA_FRONTEND_WEB_ADDR"} {
		if _, ok := registry.Find(name); !ok {
			t.Fatalf("Find(%q) = false", name)
		}
	}
	if _, ok := registry.Find("ONLAVA_FRONTEND_WEB_URL"); ok {
		t.Fatal("Find matched non-glob suffix")
	}
}

func TestRegistryRedactsSecretValues(t *testing.T) {
	registry := &Registry{
		SchemaVersion: SchemaVersion,
		Variables: []Variable{
			func() Variable {
				v := testVariable("ONLAVA_AUTH_JWT_SECRET", "exact")
				v.Secret = true
				return v
			}(),
		},
	}
	if err := registry.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	registry.index()
	if got := registry.RedactValue("ONLAVA_AUTH_JWT_SECRET", "secret"); got != RedactedValue {
		t.Fatalf("RedactValue(secret) = %q", got)
	}
	if got := registry.RedactValue("DATABASE_URL", "postgres://user:pass@localhost/db"); got != RedactedValue {
		t.Fatalf("RedactValue(database url) = %q", got)
	}
	if got := registry.RedactValue("ONLAVA_APP_ID", "app"); got != "app" {
		t.Fatalf("RedactValue(non-secret) = %q", got)
	}
}

func TestScanClassifiesRuntimeTestDocsAndFixtureEnv(t *testing.T) {
	root := t.TempDir()
	writeEnvPolicyFile(t, root, "cmd/onlava/main.go", "package main\n\nconst _ = \"ONLAVA_APP_ID\"\n")
	writeEnvPolicyFile(t, root, "cmd/onlava/main_test.go", "package main\n\nconst _ = \"ONLAVA_TEST_HELPER\"\n")
	writeEnvPolicyFile(t, root, "docs/environment.md", "`TEMPORAL_ADDRESS`\n")
	writeEnvPolicyFile(t, root, "testdata/apps/basic/main.go", "package main\n\nconst _ = \"DATABASE_URL\"\n")

	result := Scan(ScanOptions{RepoRoot: root})
	if got := EffectiveScope(result.Variables["ONLAVA_APP_ID"], "ONLAVA_APP_ID"); got != "runtime" {
		t.Fatalf("ONLAVA_APP_ID scope = %q", got)
	}
	if got := EffectiveScope(result.Variables["ONLAVA_TEST_HELPER"], "ONLAVA_TEST_HELPER"); got != "test" {
		t.Fatalf("ONLAVA_TEST_HELPER scope = %q", got)
	}
	if got := EffectiveScope(result.Variables["TEMPORAL_ADDRESS"], "TEMPORAL_ADDRESS"); got != "docs" {
		t.Fatalf("TEMPORAL_ADDRESS scope = %q", got)
	}
	if got := EffectiveScope(result.Variables["DATABASE_URL"], "DATABASE_URL"); got != "fixture" {
		t.Fatalf("DATABASE_URL scope = %q", got)
	}
}

func testVariable(name, match string) Variable {
	return Variable{
		Name:             name,
		Match:            match,
		Scope:            "runtime",
		Direction:        "internal",
		Category:         "test",
		Stability:        "stable",
		AllowedIn:        []string{"code", "docs", "tests"},
		Owner:            "test",
		Rationale:        "test variable",
		PreferredSurface: "test",
	}
}

func writeEnvPolicyFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}
