package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureOnlavaLocalStateIgnoredCreatesGitignore(t *testing.T) {
	root := t.TempDir()
	if err := ensureOnlavaLocalStateIgnored(root); err != nil {
		t.Fatalf("ensureOnlavaLocalStateIgnored: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".onlava", ".gitignore"))
	if err != nil {
		t.Fatalf("read .onlava/.gitignore: %v", err)
	}
	if !bytes.Contains(data, []byte("*\n")) || !bytes.Contains(data, []byte("!.gitignore\n")) {
		t.Fatalf("unexpected .gitignore contents:\n%s", data)
	}
}

func TestEnsureOnlavaLocalStateIgnoredPreservesExistingGitignore(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".onlava", ".gitignore")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("custom-cache/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureOnlavaLocalStateIgnored(root); err != nil {
		t.Fatalf("ensureOnlavaLocalStateIgnored: %v", err)
	}
	if err := ensureOnlavaLocalStateIgnored(root); err != nil {
		t.Fatalf("second ensureOnlavaLocalStateIgnored: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read .onlava/.gitignore: %v", err)
	}
	if !bytes.Contains(data, []byte("custom-cache/\n")) {
		t.Fatalf("existing rule was not preserved:\n%s", data)
	}
	if bytes.Count(data, []byte("!.gitignore")) != 1 {
		t.Fatalf("onlava ignore block was duplicated:\n%s", data)
	}
}

func TestEnsureLocalStateDirIgnored(t *testing.T) {
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
