package parse

import (
	"go/ast"
	"reflect"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestSyntaxFilePathsPrefersCompiledGoFilesWhenTheyMatchSyntax(t *testing.T) {
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

func TestSyntaxFilePathsFallsBackToGoFilesWhenTheyMatchSyntax(t *testing.T) {
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
