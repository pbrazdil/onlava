package runtime

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestPopulateSecretsFromDotEnv(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeFile(t, dir, ".env", "JWTSecret=top-secret\nDatabaseURL=\"postgres://localhost/db\"\n")

	prevDir, restoreDir := chdirRuntimeTest(t, dir)
	defer restoreDir(prevDir)
	resetSecretsEnvCache()

	var secrets struct {
		JWTSecret   string
		DatabaseURL string
	}
	if err := PopulateSecrets(&secrets); err != nil {
		t.Fatalf("PopulateSecrets returned error: %v", err)
	}
	if secrets.JWTSecret != "top-secret" {
		t.Fatalf("JWTSecret = %q, want %q", secrets.JWTSecret, "top-secret")
	}
	if secrets.DatabaseURL != "postgres://localhost/db" {
		t.Fatalf("DatabaseURL = %q, want %q", secrets.DatabaseURL, "postgres://localhost/db")
	}
}

func TestPopulateSecretsUsesEnvironmentOverrideAndSnakeCase(t *testing.T) {
	dir := t.TempDir()
	writeRuntimeFile(t, dir, ".env", "DatabaseURL=from-file\n")

	prevDir, restoreDir := chdirRuntimeTest(t, dir)
	defer restoreDir(prevDir)
	t.Setenv("DATABASE_URL", "from-env")
	resetSecretsEnvCache()

	var secrets struct {
		DatabaseURL string
	}
	if err := PopulateSecrets(&secrets); err != nil {
		t.Fatalf("PopulateSecrets returned error: %v", err)
	}
	if secrets.DatabaseURL != "from-env" {
		t.Fatalf("DatabaseURL = %q, want %q", secrets.DatabaseURL, "from-env")
	}
}

func TestPopulateSecretsRejectsNonStringFields(t *testing.T) {
	resetSecretsEnvCache()

	var secrets struct {
		Enabled bool
	}
	if err := PopulateSecrets(&secrets); err == nil {
		t.Fatal("PopulateSecrets returned nil error for non-string field")
	}
}

func resetSecretsEnvCache() {
	secretsEnvOnce = sync.Once{}
	secretsEnvData = nil
	secretsEnvErr = nil
}

func chdirRuntimeTest(t *testing.T, dir string) (string, func(string)) {
	t.Helper()
	prevDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return prevDir, func(path string) {
		t.Helper()
		if err := os.Chdir(path); err != nil {
			t.Fatal(err)
		}
	}
}

func writeRuntimeFile(t *testing.T, root, rel, data string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
