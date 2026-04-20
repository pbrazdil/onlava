package dbstudio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverFromDotEnv(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("DatabaseURL=postgres://localhost/app\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, ok, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}
	if !ok {
		t.Fatal("Discover returned ok=false")
	}
	if cfg.Source != ".env:DatabaseURL" {
		t.Fatalf("cfg.Source = %q, want %q", cfg.Source, ".env:DatabaseURL")
	}
	if cfg.Dialect != "postgresql" {
		t.Fatalf("cfg.Dialect = %q, want %q", cfg.Dialect, "postgresql")
	}
}

func TestDiscoverPrefersEnvironment(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("DATABASE_URL=postgres://localhost/file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATABASE_URL", "mysql://localhost/env")

	cfg, ok, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}
	if !ok {
		t.Fatal("Discover returned ok=false")
	}
	if cfg.Source != "DATABASE_URL" {
		t.Fatalf("cfg.Source = %q, want %q", cfg.Source, "DATABASE_URL")
	}
	if cfg.Dialect != "mysql" {
		t.Fatalf("cfg.Dialect = %q, want %q", cfg.Dialect, "mysql")
	}
}

func TestInferDialect(t *testing.T) {
	tests := []struct {
		rawURL string
		want   string
	}{
		{rawURL: "postgres://localhost/db", want: "postgresql"},
		{rawURL: "postgresql://localhost/db", want: "postgresql"},
		{rawURL: "mysql://localhost/db", want: "mysql"},
		{rawURL: "sqlite:///tmp/app.db", want: "sqlite"},
		{rawURL: "file:app.db", want: "sqlite"},
		{rawURL: "libsql://localhost", want: "turso"},
	}
	for _, tt := range tests {
		t.Run(tt.rawURL, func(t *testing.T) {
			got, err := inferDialect(tt.rawURL)
			if err != nil {
				t.Fatalf("inferDialect returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("inferDialect(%q) = %q, want %q", tt.rawURL, got, tt.want)
			}
		})
	}
}

func TestInferDialectRejectsUnsupportedSchemes(t *testing.T) {
	_, err := inferDialect("sqlserver://localhost/db")
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported scheme error, got %v", err)
	}
}

func TestWriteConfigFile(t *testing.T) {
	root := t.TempDir()
	path, err := writeConfigFile(root, Config{
		DatabaseURL: "postgres://localhost/db",
		Dialect:     "postgresql",
	})
	if err != nil {
		t.Fatalf("writeConfigFile returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`dialect: "postgresql"`,
		`schema: "./drizzle/*.ts"`,
		`url: "postgres://localhost/db"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("config %q missing %q", text, want)
		}
	}
}

func TestWriteWorkspacePackageJSON(t *testing.T) {
	root := t.TempDir()
	if err := writeWorkspacePackageJSON(root, "postgresql"); err != nil {
		t.Fatalf("writeWorkspacePackageJSON returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`"drizzle-kit": "latest"`,
		`"drizzle-orm": "latest"`,
		`"pg": "latest"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("package.json %q missing %q", text, want)
		}
	}
}

func TestDriverPackagesForDialect(t *testing.T) {
	tests := []struct {
		dialect string
		want    []string
	}{
		{dialect: "postgresql", want: []string{"pg"}},
		{dialect: "mysql", want: []string{"mysql2"}},
		{dialect: "sqlite", want: []string{"@libsql/client"}},
		{dialect: "turso", want: []string{"@libsql/client"}},
		{dialect: "mssql", want: []string{"mssql"}},
	}
	for _, tt := range tests {
		got := driverPackagesForDialect(tt.dialect)
		if strings.Join(got, ",") != strings.Join(tt.want, ",") {
			t.Fatalf("driverPackagesForDialect(%q) = %v, want %v", tt.dialect, got, tt.want)
		}
	}
}

func TestCommandArgs(t *testing.T) {
	opts := Options{
		Config: Config{
			DatabaseURL: "postgres://localhost/db",
			Dialect:     "postgresql",
		},
		Port: 4002,
	}
	pull := strings.Join(pullArgs("/tmp/drizzle.config.ts", opts), " ")
	studio := strings.Join(studioArgs("/tmp/drizzle.config.ts", opts), " ")
	for _, want := range []string{
		"--silent",
		"drizzle-kit pull",
		"--config=/tmp/drizzle.config.ts",
	} {
		if !strings.Contains(pull, want) {
			t.Fatalf("pull args %q missing %q", pull, want)
		}
	}
	for _, want := range []string{
		"--silent",
		"drizzle-kit studio",
		"--config=/tmp/drizzle.config.ts",
		"--host=127.0.0.1",
		"--port=4002",
	} {
		if !strings.Contains(studio, want) {
			t.Fatalf("studio args %q missing %q", studio, want)
		}
	}
}
