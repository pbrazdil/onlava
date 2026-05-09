package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

func runHarnessUIStaticStep(repoRoot string) harnessStep {
	started := time.Now()
	step := harnessStep{
		Name:    "ui static architecture",
		Command: []string{"onlava", "harness", "self", "internal:ui-static-check", repoRoot},
		Summary: map[string]any{
			"registry_namespace": "@onlava",
		},
	}
	var summary uiStaticSummary
	diagnostics := checkUIStatic(repoRoot, &summary)
	errors, warnings := countDiagnosticsBySeverity(diagnostics)
	step.Summary["checked_files"] = summary.CheckedFiles
	step.Summary["registry_items"] = summary.RegistryItems
	step.Summary["script_checks"] = summary.ScriptChecks
	step.Summary["class_warnings"] = summary.ClassWarnings
	step.Summary["errors"] = errors
	step.Summary["warnings"] = warnings
	step.Diagnostics = diagnostics
	step.OK = errors == 0
	step.DurationMS = time.Since(started).Milliseconds()
	return step
}

type uiStaticSummary struct {
	CheckedFiles  int
	RegistryItems int
	ScriptChecks  int
	ClassWarnings int
}

func checkUIStatic(repoRoot string, summary *uiStaticSummary) []checkDiagnostic {
	uiRoot := filepath.Join(repoRoot, "ui")
	var diagnostics []checkDiagnostic
	diagnostics = append(diagnostics, checkUIComponentsJSON(uiRoot, summary)...)
	diagnostics = append(diagnostics, checkUIPackageScripts(uiRoot, summary)...)
	diagnostics = append(diagnostics, checkUIRegistryItems(uiRoot, summary)...)
	sourceDiagnostics, err := checkUISourceBoundaries(uiRoot, summary)
	if err != nil {
		diagnostics = append(diagnostics, checkDiagnostic{
			Stage:           "ui static architecture",
			Severity:        "error",
			File:            filepath.ToSlash(uiRoot),
			Message:         err.Error(),
			SuggestedAction: "Fix the UI source walk error, then rerun `onlava harness self --json`.",
		})
	} else {
		diagnostics = append(diagnostics, sourceDiagnostics...)
	}
	return diagnostics
}

func checkUIComponentsJSON(uiRoot string, summary *uiStaticSummary) []checkDiagnostic {
	path := filepath.Join(uiRoot, "components.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return []checkDiagnostic{{
			Stage:           "ui static architecture",
			Severity:        "error",
			File:            filepath.ToSlash(path),
			Message:         err.Error(),
			SuggestedAction: "Create `ui/components.json` with the approved @onlava registry.",
		}}
	}
	var payload struct {
		Aliases    map[string]string          `json:"aliases"`
		Registries map[string]json.RawMessage `json:"registries"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return []checkDiagnostic{{
			Stage:           "ui static architecture",
			Severity:        "error",
			File:            filepath.ToSlash(path),
			Message:         err.Error(),
			SuggestedAction: "Fix `ui/components.json` JSON syntax.",
		}}
	}
	var diagnostics []checkDiagnostic
	if len(payload.Registries) != 1 {
		diagnostics = append(diagnostics, checkDiagnostic{
			Stage:           "ui static architecture",
			Severity:        "error",
			File:            filepath.ToSlash(path),
			Message:         fmt.Sprintf("components.json must configure exactly one registry namespace, found %d", len(payload.Registries)),
			SuggestedAction: "Configure only the @onlava registry namespace.",
		})
	}
	if _, ok := payload.Registries["@onlava"]; !ok {
		diagnostics = append(diagnostics, checkDiagnostic{
			Stage:           "ui static architecture",
			Severity:        "error",
			File:            filepath.ToSlash(path),
			Message:         "components.json does not configure @onlava",
			SuggestedAction: "Add the approved @onlava registry and remove other registry namespaces.",
		})
	}
	for namespace := range payload.Registries {
		if namespace != "@onlava" {
			diagnostics = append(diagnostics, checkDiagnostic{
				Stage:           "ui static architecture",
				Severity:        "error",
				File:            filepath.ToSlash(path),
				Message:         "unapproved shadcn registry namespace: " + namespace,
				SuggestedAction: "Use @onlava only.",
			})
		}
	}
	if payload.Aliases["ui"] != "@/components/vendor/shadcn" {
		diagnostics = append(diagnostics, checkDiagnostic{
			Stage:           "ui static architecture",
			Severity:        "error",
			File:            filepath.ToSlash(path),
			Message:         "components.json aliases.ui must point at @/components/vendor/shadcn",
			SuggestedAction: "Keep generated shadcn-derived files under the vendor layer.",
		})
	}
	return diagnostics
}

func checkUIPackageScripts(uiRoot string, summary *uiStaticSummary) []checkDiagnostic {
	path := filepath.Join(uiRoot, "package.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return []checkDiagnostic{{
			Stage:           "ui static architecture",
			Severity:        "error",
			File:            filepath.ToSlash(path),
			Message:         err.Error(),
			SuggestedAction: "Restore `ui/package.json`.",
		}}
	}
	var payload struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return []checkDiagnostic{{
			Stage:           "ui static architecture",
			Severity:        "error",
			File:            filepath.ToSlash(path),
			Message:         err.Error(),
			SuggestedAction: "Fix `ui/package.json` JSON syntax.",
		}}
	}
	var diagnostics []checkDiagnostic
	for name, script := range payload.Scripts {
		summary.ScriptChecks++
		normalized := strings.Join(strings.Fields(script), " ")
		if strings.Contains(normalized, "shadcn add") || strings.Contains(normalized, "shadcn@latest add") {
			diagnostics = append(diagnostics, checkDiagnostic{
				Stage:           "ui static architecture",
				Severity:        "error",
				File:            filepath.ToSlash(path),
				Message:         "package script uses raw shadcn add: " + name,
				SuggestedAction: "Use `node scripts/onlava-shadcn.mjs` so installs are constrained to @onlava/*.",
			})
		}
	}
	if payload.Scripts["shadcn:add"] != "node scripts/onlava-shadcn.mjs" {
		diagnostics = append(diagnostics, checkDiagnostic{
			Stage:           "ui static architecture",
			Severity:        "error",
			File:            filepath.ToSlash(path),
			Message:         "missing approved shadcn:add wrapper script",
			SuggestedAction: "Set `shadcn:add` to `node scripts/onlava-shadcn.mjs`.",
		})
	}
	return diagnostics
}

func checkUIRegistryItems(uiRoot string, summary *uiStaticSummary) []checkDiagnostic {
	registryRoot := filepath.Join(uiRoot, "registry", "onlava")
	entries, err := os.ReadDir(registryRoot)
	if err != nil {
		return []checkDiagnostic{{
			Stage:           "ui static architecture",
			Severity:        "error",
			File:            filepath.ToSlash(registryRoot),
			Message:         err.Error(),
			SuggestedAction: "Create the onlava registry under `ui/registry/onlava`.",
		}}
	}
	var diagnostics []checkDiagnostic
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || entry.Name() == "registry.json" {
			continue
		}
		summary.RegistryItems++
		path := filepath.Join(registryRoot, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			diagnostics = append(diagnostics, checkDiagnostic{
				Stage:           "ui static architecture",
				Severity:        "error",
				File:            filepath.ToSlash(path),
				Message:         err.Error(),
				SuggestedAction: "Fix the unreadable registry item.",
			})
			continue
		}
		var item struct {
			Name                 string   `json:"name"`
			Type                 string   `json:"type"`
			RegistryDependencies []string `json:"registryDependencies"`
		}
		if err := json.Unmarshal(data, &item); err != nil {
			diagnostics = append(diagnostics, checkDiagnostic{
				Stage:           "ui static architecture",
				Severity:        "error",
				File:            filepath.ToSlash(path),
				Message:         err.Error(),
				SuggestedAction: "Fix the registry item JSON syntax.",
			})
			continue
		}
		if item.Name == "" || item.Type == "" {
			diagnostics = append(diagnostics, checkDiagnostic{
				Stage:           "ui static architecture",
				Severity:        "error",
				File:            filepath.ToSlash(path),
				Message:         "registry item must include name and type",
				SuggestedAction: "Add the required shadcn registry item metadata.",
			})
		}
		for _, dep := range item.RegistryDependencies {
			if !strings.HasPrefix(dep, "@onlava/") {
				diagnostics = append(diagnostics, checkDiagnostic{
					Stage:           "ui static architecture",
					Severity:        "error",
					File:            filepath.ToSlash(path),
					Message:         "registry item depends on non-onlava item: " + dep,
					SuggestedAction: "Promote the dependency into @onlava or remove it.",
				})
			}
		}
	}
	return diagnostics
}

func checkUISourceBoundaries(uiRoot string, summary *uiStaticSummary) ([]checkDiagnostic, error) {
	srcRoot := filepath.Join(uiRoot, "src")
	var diagnostics []checkDiagnostic
	err := filepath.WalkDir(srcRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".ts" && ext != ".tsx" {
			return nil
		}
		rel, err := filepath.Rel(uiRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		summary.CheckedFiles++
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(data)
		for _, spec := range uiImportSpecifiers(text) {
			if diag, ok := uiForbiddenImportDiagnostic(uiRoot, path, rel, spec); ok {
				diagnostics = append(diagnostics, diag)
			}
		}
		classDiagnostics := uiClassNameDiagnostics(uiRoot, path, rel, text)
		summary.ClassWarnings += len(classDiagnostics)
		diagnostics = append(diagnostics, classDiagnostics...)
		return nil
	})
	return diagnostics, err
}

func uiForbiddenImportDiagnostic(uiRoot, path, rel, spec string) (checkDiagnostic, bool) {
	allowLowLevel := strings.HasPrefix(rel, "src/components/primitives/") ||
		strings.HasPrefix(rel, "src/components/layouts/") ||
		strings.HasPrefix(rel, "src/components/vendor/shadcn/")
	if strings.Contains(spec, "/components/ui") || strings.HasPrefix(spec, "@/components/ui") {
		return uiBoundaryDiag(path, "import from legacy components/ui is forbidden: "+spec, "Import from @/components/primitives or @/components/layouts."), true
	}
	if strings.Contains(spec, "/components/vendor/shadcn") || strings.HasPrefix(spec, "@/components/vendor/shadcn") {
		if !allowLowLevel {
			return uiBoundaryDiag(path, "app screens must not import vendor shadcn directly: "+spec, "Wrap the vendor component in an onlava primitive first."), true
		}
	}
	switch spec {
	case "class-variance-authority", "clsx", "tailwind-merge":
		if !allowLowLevel && rel != "src/lib/utils.ts" {
			return uiBoundaryDiag(path, "styling utility import is only allowed in onlava primitives/layouts/vendor: "+spec, "Expose a typed primitive or layout instead."), true
		}
	case "lucide-react":
		if rel != "src/components/primitives/icons.tsx" && !strings.HasPrefix(rel, "src/components/layouts/") {
			return uiBoundaryDiag(path, "lucide-react imports must go through an onlava icons wrapper", "Create or use @/components/primitives/icons."), true
		}
	case "radix-ui":
		if !allowLowLevel {
			return uiBoundaryDiag(path, "radix-ui imports are only allowed inside onlava primitives/layouts/vendor", "Wrap Radix behavior in an onlava primitive."), true
		}
	default:
		if strings.HasPrefix(spec, "@radix-ui/") && !allowLowLevel {
			return uiBoundaryDiag(path, "Radix imports are only allowed inside onlava primitives/layouts/vendor: "+spec, "Wrap Radix behavior in an onlava primitive."), true
		}
	}
	return checkDiagnostic{}, false
}

func uiBoundaryDiag(path, message, suggestion string) checkDiagnostic {
	return checkDiagnostic{
		Stage:           "ui static architecture",
		Severity:        "error",
		File:            filepath.ToSlash(path),
		Message:         message,
		SuggestedAction: suggestion,
	}
}

func uiClassNameDiagnostics(uiRoot, path, rel, text string) []checkDiagnostic {
	if strings.HasPrefix(rel, "src/components/primitives/") ||
		strings.HasPrefix(rel, "src/components/layouts/") ||
		strings.HasPrefix(rel, "src/components/vendor/shadcn/") {
		return nil
	}
	var diagnostics []checkDiagnostic
	for _, value := range uiClassNameLiterals(text) {
		if len(value) > 180 {
			diagnostics = append(diagnostics, checkDiagnostic{
				Stage:           "ui static architecture",
				Severity:        "warning",
				File:            filepath.ToSlash(path),
				Message:         fmt.Sprintf("long className literal (%d chars) should move into an onlava primitive or layout", len(value)),
				SuggestedAction: "When touching this file, compose existing onlava primitives/layouts instead of extending class soup.",
			})
			continue
		}
		if strings.Contains(value, "[&") || strings.Contains(value, "![") || strings.Contains(value, "!") {
			diagnostics = append(diagnostics, checkDiagnostic{
				Stage:           "ui static architecture",
				Severity:        "warning",
				File:            filepath.ToSlash(path),
				Message:         "advanced Tailwind-style className syntax outside primitives/layouts should be promoted",
				SuggestedAction: "Move the behavior into an onlava primitive or layout before expanding it.",
			})
		}
	}
	return diagnostics
}

func uiImportSpecifiers(text string) []string {
	var specs []string
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "import ") {
			continue
		}
		if idx := strings.LastIndex(trimmed, " from "); idx >= 0 {
			if spec, ok := uiQuotedString(trimmed[idx+len(" from "):]); ok {
				specs = append(specs, spec)
			}
			continue
		}
		if spec, ok := uiQuotedString(strings.TrimPrefix(trimmed, "import ")); ok {
			specs = append(specs, spec)
		}
	}
	return specs
}

func uiClassNameLiterals(text string) []string {
	var values []string
	for _, marker := range []string{`className="`, `className='`} {
		offset := 0
		for {
			idx := strings.Index(text[offset:], marker)
			if idx < 0 {
				break
			}
			start := offset + idx + len(marker)
			quote := marker[len(marker)-1]
			end := strings.IndexByte(text[start:], quote)
			if end < 0 {
				break
			}
			values = append(values, text[start:start+end])
			offset = start + end + 1
		}
	}
	return values
}

func uiQuotedString(text string) (string, bool) {
	text = strings.TrimSpace(strings.TrimSuffix(text, ";"))
	if text == "" {
		return "", false
	}
	quote := rune(text[0])
	if quote != '"' && quote != '\'' {
		return "", false
	}
	var b strings.Builder
	escaped := false
	for _, r := range text[1:] {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == quote {
			return b.String(), true
		}
		if unicode.IsSpace(r) {
			return "", false
		}
		b.WriteRune(r)
	}
	return "", false
}
