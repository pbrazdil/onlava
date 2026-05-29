package parse

import (
	"go/ast"
	"path/filepath"
	"reflect"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestSyntaxFilePathsPrefersCompiledGoFilesWhenTheyMatchSyntax(t *testing.T) {
	t.Parallel()

	pkg := &packages.Package{
		GoFiles:         []string{"api.go"},
		CompiledGoFiles: []string{"api.go", "cgo_gen.go"},
		Syntax:          make([]*ast.File, 2),
	}

	got := syntaxFilePaths(pkg)
	want := []string{"api.go", "cgo_gen.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("syntaxFilePaths() = %v, want %v", got, want)
	}
}

func TestServiceRootForPackagePrefersExplicitNestedServiceRoot(t *testing.T) {
	t.Parallel()

	projectsRoot := filepath.Join("solar", "projects")
	explicit := map[string]bool{projectsRoot: true}

	tests := map[string]string{
		projectsRoot:                             projectsRoot,
		filepath.Join(projectsRoot, "db", "gen"): projectsRoot,
		filepath.Join("solar", "tasks"):          "solar",
		"billing":                                "billing",
	}

	for relDir, want := range tests {
		if got := serviceRootForPackage(relDir, explicit); got != want {
			t.Fatalf("serviceRootForPackage(%q) = %q, want %q", relDir, got, want)
		}
	}
}

func TestSyntaxFilePathsFallsBackToGoFilesWhenTheyMatchSyntax(t *testing.T) {
	t.Parallel()

	pkg := &packages.Package{
		GoFiles:         []string{"api.go", "extra.go"},
		CompiledGoFiles: []string{"api.go"},
		Syntax:          make([]*ast.File, 2),
	}

	got := syntaxFilePaths(pkg)
	want := []string{"api.go", "extra.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("syntaxFilePaths() = %v, want %v", got, want)
	}
}
