package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureSceneryLocalStateIgnoredCreatesGitignore(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := ensureSceneryLocalStateIgnored(root); err != nil {
		t.Fatalf("ensureSceneryLocalStateIgnored: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".scenery", ".gitignore"))
	if err != nil {
		t.Fatalf("read .scenery/.gitignore: %v", err)
	}
	if !bytes.Contains(data, []byte("*\n")) || !bytes.Contains(data, []byte("!.gitignore\n")) {
		t.Fatalf("unexpected .gitignore contents:\n%s", data)
	}
}

func TestEnsureSceneryLocalStateIgnoredPreservesExistingGitignore(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, ".scenery", ".gitignore")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("custom-cache/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureSceneryLocalStateIgnored(root); err != nil {
		t.Fatalf("ensureSceneryLocalStateIgnored: %v", err)
	}
	if err := ensureSceneryLocalStateIgnored(root); err != nil {
		t.Fatalf("second ensureSceneryLocalStateIgnored: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read .scenery/.gitignore: %v", err)
	}
	if !bytes.Contains(data, []byte("custom-cache/\n")) {
		t.Fatalf("existing rule was not preserved:\n%s", data)
	}
	if bytes.Count(data, []byte("!.gitignore")) != 1 {
		t.Fatalf("scenery ignore block was duplicated:\n%s", data)
	}
}

func TestEnsureLocalStateDirIgnored(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "custom-cache")
	if err := ensureLocalStateDirIgnored(root); err != nil {
		t.Fatalf("ensureLocalStateDirIgnored: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("read custom .gitignore: %v", err)
	}
	if !bytes.Contains(data, []byte("*\n")) || !bytes.Contains(data, []byte("!.gitignore\n")) {
		t.Fatalf("unexpected .gitignore contents:\n%s", data)
	}
}
