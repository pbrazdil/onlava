package envfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".env")
	if err := os.WriteFile(path, []byte("\ufeff# comment\nexport A=one\nB=\"two\\nlines\"\nC='three'\nD=four\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	want := map[string]string{
		"A": "one",
		"B": "two\nlines",
		"C": "three",
		"D": "four",
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("ParseFile()[%q] = %q, want %q", key, got[key], value)
		}
	}
}

func TestMergeFilesAndAppendMissing(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("A=from-env\nB=from-env\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env.local"), []byte("B=from-local\nC=from-local\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	values, err := MergeFiles(root, ".env", ".env.local")
	if err != nil {
		t.Fatalf("MergeFiles: %v", err)
	}
	env := AppendMissing([]string{"A=from-process"}, values)

	if !containsEnv(env, "A=from-process") {
		t.Fatalf("AppendMissing missing process value: %v", env)
	}
	if containsEnv(env, "A=from-env") {
		t.Fatalf("AppendMissing overwrote process value: %v", env)
	}
	if !containsEnv(env, "B=from-local") {
		t.Fatalf("AppendMissing missing .env.local override: %v", env)
	}
	if !containsEnv(env, "C=from-local") {
		t.Fatalf("AppendMissing missing .env.local value: %v", env)
	}
}

func TestParseFileMissingReturnsEmpty(t *testing.T) {
	got, err := ParseFile(filepath.Join(t.TempDir(), ".env"))
	if err != nil {
		t.Fatalf("ParseFile missing: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ParseFile missing returned %#v, want empty map", got)
	}
}

func containsEnv(env []string, want string) bool {
	for _, item := range env {
		if item == want {
			return true
		}
	}
	return false
}
