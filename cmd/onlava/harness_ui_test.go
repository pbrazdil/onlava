package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestUIStaticRegistryValidation(t *testing.T) {
	tests := []struct {
		name        string
		itemJSON    string
		wantMessage string
	}{
		{
			name: "dependency URL rejected",
			itemJSON: `{
  "name": "bad",
  "type": "registry:component",
  "registryDependencies": ["https://example.com/r/button.json"],
  "files": [{"source": "src/components/primitives/Button.tsx", "target": "@components/primitives/Button.tsx", "type": "registry:component"}]
}`,
			wantMessage: "depends on non-onlava item",
		},
		{
			name: "root target rejected",
			itemJSON: `{
  "name": "bad",
  "type": "registry:component",
  "files": [{"source": "src/components/primitives/Button.tsx", "target": "~/.env", "type": "registry:file"}]
}`,
			wantMessage: "must not be absolute, root-relative, or traversing",
		},
		{
			name: "package target rejected",
			itemJSON: `{
  "name": "bad",
  "type": "registry:component",
  "files": [{"source": "src/components/primitives/Button.tsx", "target": "@components/package.json", "type": "registry:file"}]
}`,
			wantMessage: "may not write package.json",
		},
		{
			name: "source traversal rejected",
			itemJSON: `{
  "name": "bad",
  "type": "registry:component",
  "files": [{"source": "../outside.tsx", "target": "@components/primitives/Button.tsx", "type": "registry:component"}]
}`,
			wantMessage: "source must be a relative path under ui/src",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := newUIStaticFixture(t)
			writeFile(t, filepath.Join(root, "ui", "registry", "onlava", "bad.json"), tt.itemJSON)
			var summary uiStaticSummary
			diagnostics := checkUIStatic(root, &summary)
			if !diagnosticsContain(diagnostics, tt.wantMessage) {
				t.Fatalf("expected diagnostic containing %q, got %#v", tt.wantMessage, diagnostics)
			}
		})
	}
}

func TestUIStaticPackageScriptRejectsRawShadcn(t *testing.T) {
	root := newUIStaticFixture(t)
	writeFile(t, filepath.Join(root, "ui", "package.json"), `{
  "scripts": {
    "shadcn:add": "node scripts/onlava-shadcn.mjs",
    "bad": "bunx shadcn@4.7.0 add button"
  }
}`)
	var summary uiStaticSummary
	diagnostics := checkUIStatic(root, &summary)
	if !diagnosticsContain(diagnostics, "package script uses raw shadcn add") {
		t.Fatalf("expected raw shadcn script diagnostic, got %#v", diagnostics)
	}
}

func TestUIImportSpecifiersCatchBypasses(t *testing.T) {
	text := `
import {
  Button
} from "@/components/ui/button";
import "@/components/ui/dialog";
export { Card } from "@/components/ui/card";
export * from "@/components/ui/table";
const mod = await import("@/components/ui/dynamic");
const req = require("@/components/ui/require");
`
	got := uiImportSpecifiers(text)
	for _, want := range []string{
		"@/components/ui/button",
		"@/components/ui/dialog",
		"@/components/ui/card",
		"@/components/ui/table",
		"@/components/ui/dynamic",
		"@/components/ui/require",
	} {
		if !slices.Contains(got, want) {
			t.Fatalf("expected %q in %#v", want, got)
		}
	}
}

func TestUIStaticImportBypassesRejected(t *testing.T) {
	root := newUIStaticFixture(t)
	writeFile(t, filepath.Join(root, "ui", "src", "routes", "bad.tsx"), `
export { Button } from "@/components/ui/button";
const mod = await import("@/components/vendor/shadcn/dialog");
const req = require("@radix-ui/react-dialog");
`)
	var summary uiStaticSummary
	diagnostics := checkUIStatic(root, &summary)
	for _, want := range []string{
		"legacy components/ui",
		"vendor shadcn directly",
		"Radix imports",
	} {
		if !diagnosticsContain(diagnostics, want) {
			t.Fatalf("expected diagnostic containing %q, got %#v", want, diagnostics)
		}
	}
}

func TestUIClassNameDiagnosticsCatchExpressions(t *testing.T) {
	longClass := strings.Repeat("grid ", 50)
	text := `
<div className={cn("` + longClass + `", active && "hover:[&>*]:text-red-500")} />
<div className={active ? "` + longClass + `" : "short"} />
<div className={` + "`" + longClass + "`" + `} />
`
	diagnostics := uiClassNameDiagnostics("/tmp/ui", "/tmp/ui/src/routes/page.tsx", "src/routes/page.tsx", text)
	if len(diagnostics) < 3 {
		t.Fatalf("expected className expression warnings, got %#v", diagnostics)
	}
}

func newUIStaticFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "ui", "components.json"), `{
  "aliases": {
    "components": "@/components",
    "ui": "@/components/vendor/shadcn",
    "lib": "@/lib",
    "hooks": "@/hooks",
    "utils": "@/lib/utils"
  },
  "registries": {
    "@onlava": "http://127.0.0.1:4873/r/{name}.json"
  }
}`)
	writeFile(t, filepath.Join(root, "ui", "package.json"), `{
  "scripts": {
    "shadcn:add": "node scripts/onlava-shadcn.mjs"
  }
}`)
	writeFile(t, filepath.Join(root, "ui", "src", "components", "primitives", "Button.tsx"), "export function Button() { return null }\n")
	writeFile(t, filepath.Join(root, "ui", "src", "routes", "ok.tsx"), "export function OK() { return null }\n")
	writeFile(t, filepath.Join(root, "ui", "registry", "onlava", "button.json"), `{
  "name": "button",
  "type": "registry:component",
  "files": [{"source": "src/components/primitives/Button.tsx", "target": "@components/primitives/Button.tsx", "type": "registry:component"}]
}`)
	return root
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func diagnosticsContain(diagnostics []checkDiagnostic, needle string) bool {
	for _, diag := range diagnostics {
		if strings.Contains(diag.Message, needle) {
			return true
		}
	}
	return false
}
