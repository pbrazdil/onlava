package build

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"

	"pulse.dev/internal/app"
	"pulse.dev/internal/codegen"
	inspectdata "pulse.dev/internal/inspect"
	"pulse.dev/internal/model"
	"pulse.dev/internal/parse"
	"pulse.dev/internal/wiremodel"
)

type Result struct {
	AppRoot               string
	AppName               string
	AppID                 string
	Dir                   string
	Binary                string
	NeedsTidy             bool
	DependencyFingerprint string
	GraphFingerprint      string
	Metadata              json.RawMessage
	APIEncoding           json.RawMessage
	SourceFiles           []string
	GeneratedFiles        []string
	Ephemeral             bool
}

type buildState struct {
	Version               string   `json:"version,omitempty"`
	DependencyFingerprint string   `json:"dependency_fingerprint"`
	GraphFingerprint      string   `json:"graph_fingerprint,omitempty"`
	Metadata              []byte   `json:"metadata,omitempty"`
	APIEncoding           []byte   `json:"api_encoding,omitempty"`
	SourceFiles           []string `json:"source_files,omitempty"`
	GeneratedFiles        []string `json:"generated_files,omitempty"`
}

const (
	buildStateFile    = ".pulse-build-state.json"
	buildStateVersion = "2"
)

type PrepareOptions struct {
	ChangedPaths []string
}

type CachedGraph struct {
	Result      *Result
	Metadata    json.RawMessage
	APIEncoding json.RawMessage
}

type GeneratedManifest struct {
	SchemaVersion string                  `json:"schema_version"`
	App           inspectdata.AppRef      `json:"app"`
	Counts        inspectdata.AppCounts   `json:"counts"`
	Artifacts     GeneratedManifestPaths  `json:"artifacts"`
	Schemas       GeneratedManifestSchema `json:"schemas"`
	Hashes        GeneratedManifestHashes `json:"hashes"`
}

type GeneratedManifestPaths struct {
	App              string `json:"app"`
	Routes           string `json:"routes"`
	Services         string `json:"services"`
	Endpoints        string `json:"endpoints"`
	WireCapabilities string `json:"wire_capabilities"`
	BuildLatest      string `json:"build_latest"`
}

type GeneratedManifestSchema struct {
	App              string `json:"app"`
	Routes           string `json:"routes"`
	Services         string `json:"services"`
	Endpoints        string `json:"endpoints"`
	WireCapabilities string `json:"wire_capabilities"`
	BuildLatest      string `json:"build_latest"`
}

type GeneratedManifestHashes struct {
	App              string `json:"app"`
	Routes           string `json:"routes"`
	Services         string `json:"services"`
	Endpoints        string `json:"endpoints"`
	WireCapabilities string `json:"wire_capabilities"`
}

type generatedInspectArtifacts struct {
	App                  inspectdata.AppResponse
	Routes               inspectdata.RoutesResponse
	Services             inspectdata.ServicesResponse
	Endpoints            inspectdata.EndpointsResponse
	WireCapabilities     any
	AppJSON              []byte
	RoutesJSON           []byte
	ServicesJSON         []byte
	EndpointsJSON        []byte
	WireCapabilitiesJSON []byte
}

type LatestBuildManifest struct {
	SchemaVersion string                    `json:"schema_version"`
	App           LatestBuildManifestApp    `json:"app"`
	Build         LatestBuildManifestRecord `json:"build"`
}

type LatestBuildManifestApp struct {
	Name       string `json:"name"`
	ID         string `json:"id,omitempty"`
	Root       string `json:"root"`
	ConfigPath string `json:"config_path"`
}

type LatestBuildManifestRecord struct {
	Phase                 string `json:"phase"`
	WorkspaceDir          string `json:"workspace_dir"`
	BinaryPath            string `json:"binary_path"`
	WorkspaceExists       bool   `json:"workspace_exists"`
	BinaryExists          bool   `json:"binary_exists"`
	BuildStatePath        string `json:"build_state_path"`
	BuildStateExists      bool   `json:"build_state_exists"`
	BuildStateVersion     string `json:"build_state_version,omitempty"`
	DependencyFingerprint string `json:"dependency_fingerprint,omitempty"`
	GraphFingerprint      string `json:"graph_fingerprint,omitempty"`
	MetadataPresent       bool   `json:"metadata_present"`
	APIEncodingPresent    bool   `json:"api_encoding_present"`
	SourceFileCount       int    `json:"source_file_count"`
	GeneratedFileCount    int    `json:"generated_file_count"`
}

func App(appRoot string, cfg app.Config) (*Result, error) {
	model, err := parse.App(appRoot, cfg.Name)
	if err != nil {
		return nil, err
	}
	result, err := Prepare(appRoot, model, cfg, PrepareOptions{})
	if err != nil {
		return nil, err
	}
	if err := Compile(result); err != nil {
		if result.Ephemeral {
			_ = os.RemoveAll(result.Dir)
		}
		return nil, err
	}
	return result, nil
}

func Prepare(appRoot string, model *model.App, cfg app.Config, opts PrepareOptions) (*Result, error) {
	artifacts, err := writeGeneratedInspectArtifacts(appRoot, cfg, model)
	if err != nil {
		return nil, err
	}
	gen, err := codegen.GenerateWithConfig(model, cfg)
	if err != nil {
		return nil, err
	}

	workspaceDir, err := workspaceDir(appRoot, cfg.Name)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		return nil, err
	}
	state, err := loadBuildState(workspaceDir)
	if err != nil {
		return nil, err
	}
	sourceFiles, err := syncSourceFiles(workspaceDir, appRoot, state.SourceFiles, opts.ChangedPaths)
	if err != nil {
		return nil, err
	}
	generatedFiles, err := syncGeneratedFiles(workspaceDir, appRoot, gen, state.GeneratedFiles, sourceFiles)
	if err != nil {
		return nil, err
	}
	if err := removeUnexpectedFilesFromLists(workspaceDir, sourceFiles, generatedFiles); err != nil {
		return nil, err
	}
	depFingerprint, err := dependencyFingerprintFromWorkspace(workspaceDir)
	if err != nil {
		return nil, err
	}
	needsTidy := state.DependencyFingerprint != depFingerprint
	binary := filepath.Join(workspaceDir, "pulse-app")
	result := &Result{
		AppRoot:               appRoot,
		AppName:               cfg.Name,
		AppID:                 cfg.ID,
		Dir:                   workspaceDir,
		Binary:                binary,
		NeedsTidy:             needsTidy,
		DependencyFingerprint: depFingerprint,
		SourceFiles:           sourceFiles,
		GeneratedFiles:        generatedFiles,
	}
	if err := WriteLatestBuildManifest(result, "prepared"); err != nil {
		return nil, err
	}
	if err := writeGeneratedManifest(appRoot, artifacts); err != nil {
		return nil, err
	}
	return result, nil
}

func writeGeneratedInspectArtifacts(appRoot string, cfg app.Config, appModel *model.App) (*generatedInspectArtifacts, error) {
	artifacts := &generatedInspectArtifacts{
		App:              inspectdata.BuildAppResponse(appRoot, cfg, appModel),
		Routes:           inspectdata.BuildRoutesResponse(appRoot, cfg, appModel),
		Services:         inspectdata.BuildServicesResponse(appRoot, cfg, appModel),
		Endpoints:        inspectdata.BuildEndpointsResponse(appRoot, cfg, appModel),
		WireCapabilities: wiremodel.AppCapabilities(appModel),
	}
	genDir := filepath.Join(appRoot, ".pulse", "gen")
	files := map[string]*[]byte{
		"app.json":               &artifacts.AppJSON,
		"routes.json":            &artifacts.RoutesJSON,
		"services.json":          &artifacts.ServicesJSON,
		"endpoints.json":         &artifacts.EndpointsJSON,
		"wire/capabilities.json": &artifacts.WireCapabilitiesJSON,
	}
	payloads := map[string]any{
		"app.json":               artifacts.App,
		"routes.json":            artifacts.Routes,
		"services.json":          artifacts.Services,
		"endpoints.json":         artifacts.Endpoints,
		"wire/capabilities.json": artifacts.WireCapabilities,
	}
	for name, target := range files {
		data, err := json.MarshalIndent(payloads[name], "", "  ")
		if err != nil {
			return nil, err
		}
		data = append(data, '\n')
		if err := writeFileIfChanged(genDir, name, data); err != nil {
			return nil, err
		}
		*target = data
	}
	return artifacts, nil
}

func writeGeneratedManifest(appRoot string, artifacts *generatedInspectArtifacts) error {
	if artifacts == nil {
		return fmt.Errorf("nil generated inspect artifacts")
	}
	manifest := GeneratedManifest{
		SchemaVersion: "pulse.gen.manifest.v1",
		App:           artifacts.App.App,
		Counts:        artifacts.App.Counts,
		Artifacts: GeneratedManifestPaths{
			App:              ".pulse/gen/app.json",
			Routes:           ".pulse/gen/routes.json",
			Services:         ".pulse/gen/services.json",
			Endpoints:        ".pulse/gen/endpoints.json",
			WireCapabilities: ".pulse/gen/wire/capabilities.json",
			BuildLatest:      ".pulse/build/latest.json",
		},
		Schemas: GeneratedManifestSchema{
			App:              artifacts.App.SchemaVersion,
			Routes:           artifacts.Routes.SchemaVersion,
			Services:         artifacts.Services.SchemaVersion,
			Endpoints:        artifacts.Endpoints.SchemaVersion,
			WireCapabilities: "pulse.wire.capabilities.v1",
			BuildLatest:      "pulse.build.latest.v1",
		},
		Hashes: GeneratedManifestHashes{
			App:              sha256Hex(artifacts.AppJSON),
			Routes:           sha256Hex(artifacts.RoutesJSON),
			Services:         sha256Hex(artifacts.ServicesJSON),
			Endpoints:        sha256Hex(artifacts.EndpointsJSON),
			WireCapabilities: sha256Hex(artifacts.WireCapabilitiesJSON),
		},
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFileIfChanged(filepath.Join(appRoot, ".pulse", "gen"), "manifest.json", data)
}

func Compile(result *Result) error {
	return CompileContext(context.Background(), result)
}

func PrimeWorkspace(result *Result) error {
	return PrimeWorkspaceContext(context.Background(), result)
}

func PrimeWorkspaceContext(ctx context.Context, result *Result) error {
	if result == nil {
		return fmt.Errorf("nil build result")
	}
	if result.NeedsTidy {
		if err := runGoContext(ctx, result.Dir, "mod", "tidy"); err != nil {
			return err
		}
		fingerprint, err := dependencyFingerprintFromWorkspace(result.Dir)
		if err != nil {
			return err
		}
		result.DependencyFingerprint = fingerprint
		result.NeedsTidy = false
	}
	if err := saveBuildState(result.Dir, buildState{
		Version:               buildStateVersion,
		DependencyFingerprint: result.DependencyFingerprint,
		GraphFingerprint:      result.GraphFingerprint,
		Metadata:              append([]byte(nil), result.Metadata...),
		APIEncoding:           append([]byte(nil), result.APIEncoding...),
		SourceFiles:           append([]string(nil), result.SourceFiles...),
		GeneratedFiles:        append([]string(nil), result.GeneratedFiles...),
	}); err != nil {
		return err
	}
	if err := WriteLatestBuildManifest(result, "primed"); err != nil {
		return err
	}
	return nil
}

func CompileContext(ctx context.Context, result *Result) error {
	if result == nil {
		return fmt.Errorf("nil build result")
	}
	if err := PrimeWorkspaceContext(ctx, result); err != nil {
		return err
	}
	if err := runGoContext(ctx, result.Dir, "build", "-o", result.Binary, "./pulse_internal_main"); err != nil {
		return err
	}
	if err := WriteLatestBuildManifest(result, "compiled"); err != nil {
		return err
	}
	return nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() && shouldSkipDir(rel) {
			return filepath.SkipDir
		}
		if !d.IsDir() && shouldSkipFile(rel) {
			return nil
		}
		if shouldSkipSymlink(path, d) {
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func syncSourceFiles(root, appRoot string, prevFiles, changedPaths []string) ([]string, error) {
	if len(prevFiles) == 0 || len(changedPaths) == 0 {
		return syncAllSourceFiles(root, appRoot, nil)
	}
	currentFiles, err := listSourceFiles(appRoot)
	if err != nil {
		return nil, err
	}
	current := make(map[string]struct{}, len(currentFiles))
	for _, rel := range currentFiles {
		current[rel] = struct{}{}
	}
	prev := make(map[string]struct{}, len(prevFiles))
	for _, rel := range prevFiles {
		prev[filepath.ToSlash(rel)] = struct{}{}
	}
	changed := make(map[string]struct{}, len(changedPaths))
	for _, rel := range changedPaths {
		rel = filepath.ToSlash(rel)
		changed[rel] = struct{}{}
		if _, ok := current[rel]; !ok {
			if removeErr := os.Remove(filepath.Join(root, filepath.FromSlash(rel))); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				return nil, removeErr
			}
			continue
		}
	}

	for _, rel := range currentFiles {
		_, wasTracked := prev[rel]
		_, wasChanged := changed[rel]
		target := filepath.Join(root, filepath.FromSlash(rel))
		if !wasChanged && wasTracked {
			if _, err := os.Stat(target); err == nil {
				continue
			} else if !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
		}
		data, err := sourceFileData(filepath.Join(appRoot, filepath.FromSlash(rel)), rel)
		if err != nil {
			return nil, err
		}
		if err := writeFileIfChanged(root, rel, data); err != nil {
			return nil, err
		}
	}

	for rel := range prev {
		if _, ok := current[rel]; ok {
			continue
		}
		if removeErr := os.Remove(filepath.Join(root, filepath.FromSlash(rel))); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return nil, removeErr
		}
	}
	return currentFiles, nil
}

func syncAllSourceFiles(root, appRoot string, skip map[string]struct{}) ([]string, error) {
	currentFiles, err := listSourceFiles(appRoot)
	if err != nil {
		return nil, err
	}
	files := make(map[string]struct{}, len(currentFiles))
	for _, rel := range currentFiles {
		files[rel] = struct{}{}
		if _, ok := skip[rel]; ok {
			continue
		}
		data, err := sourceFileData(filepath.Join(appRoot, filepath.FromSlash(rel)), rel)
		if err != nil {
			return nil, err
		}
		if err := writeFileIfChanged(root, rel, data); err != nil {
			return nil, err
		}
	}
	return sortedKeys(files), nil
}

func listSourceFiles(appRoot string) ([]string, error) {
	files := make(map[string]struct{})
	err := filepath.WalkDir(appRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(appRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() && shouldSkipDir(rel) {
			return filepath.SkipDir
		}
		if d.IsDir() || !isSourceFile(rel) || shouldSkipFile(rel) || shouldSkipSymlink(path, d) {
			return nil
		}
		files[filepath.ToSlash(rel)] = struct{}{}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return sortedKeys(files), nil
}

func shouldSkipDir(rel string) bool {
	base := filepath.Base(rel)
	if strings.HasPrefix(base, ".") {
		return true
	}
	return base == "node_modules" || base == "pulse_internal_main"
}

func shouldSkipFile(rel string) bool {
	return filepath.Base(rel) == "encore.gen.go"
}

func shouldSkipSymlink(path string, d os.DirEntry) bool {
	if d.Type()&os.ModeSymlink == 0 {
		return false
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	return err == nil && info.IsDir()
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if filepath.Ext(src) == ".go" {
		data, err = rewriteEncoreCompat(src, data)
		if err != nil {
			return err
		}
	}
	return os.WriteFile(dst, data, 0o644)
}

func sourceFileData(path, rel string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	switch rel {
	case "go.mod":
		return patchGoModData(data, app.RepoRoot())
	}
	if filepath.Ext(rel) == ".go" {
		return rewriteEncoreCompat(path, data)
	}
	return data, nil
}

func writeFileIfChanged(root, rel string, data []byte) error {
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	current, err := os.ReadFile(path)
	if err == nil && string(current) == string(data) {
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func patchGoModData(data []byte, repoRoot string) ([]byte, error) {
	file, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return nil, err
	}
	if err := file.AddRequire("pulse.dev", "v0.0.0"); err != nil && !strings.Contains(err.Error(), "already exists") {
		return nil, err
	}
	_ = file.DropReplace("pulse.dev", "")
	if err := file.AddReplace("pulse.dev", "", repoRoot, ""); err != nil {
		return nil, err
	}
	formatted, err := file.Format()
	if err != nil {
		return nil, err
	}
	return formatted, nil
}

func runGoContext(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go %s failed: %w\n%s", strings.Join(args, " "), err, output)
	}
	return nil
}

func workspaceDir(appRoot, appName string) (string, error) {
	cacheRoot, err := pulseCacheRoot()
	if err != nil {
		return "", err
	}
	absRoot, err := filepath.Abs(appRoot)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(absRoot))
	name := sanitizeWorkspaceLabel(appName)
	if name == "" {
		name = "app"
	}
	return filepath.Join(cacheRoot, "build", name+"-"+hex.EncodeToString(sum[:8])), nil
}

func pulseCacheRoot() (string, error) {
	if root := strings.TrimSpace(os.Getenv("PULSE_DEV_CACHE_DIR")); root != "" {
		return root, nil
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pulse"), nil
}

func CacheRoot() (string, error) {
	return pulseCacheRoot()
}

func WorkspaceDir(appRoot, appName string) (string, error) {
	return workspaceDir(appRoot, appName)
}

func BuildStatePath(appRoot, appName string) (string, error) {
	root, err := workspaceDir(appRoot, appName)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, buildStateFile), nil
}

func LatestBuildPath(appRoot string) string {
	return filepath.Join(appRoot, ".pulse", "build", "latest.json")
}

type StateInfo struct {
	Path                  string
	Exists                bool
	Version               string
	DependencyFingerprint string
	GraphFingerprint      string
	MetadataPresent       bool
	APIEncodingPresent    bool
	SourceFiles           []string
	GeneratedFiles        []string
}

func ReadStateInfo(appRoot, appName string) (*StateInfo, error) {
	statePath, err := BuildStatePath(appRoot, appName)
	if err != nil {
		return nil, err
	}
	info := &StateInfo{Path: statePath}
	if _, err := os.Stat(statePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return info, nil
		}
		return nil, err
	}
	state, err := loadBuildState(filepath.Dir(statePath))
	if err != nil {
		return nil, err
	}
	info.Exists = true
	info.Version = state.Version
	info.DependencyFingerprint = state.DependencyFingerprint
	info.GraphFingerprint = state.GraphFingerprint
	info.MetadataPresent = len(state.Metadata) > 0
	info.APIEncodingPresent = len(state.APIEncoding) > 0
	info.SourceFiles = append([]string(nil), state.SourceFiles...)
	info.GeneratedFiles = append([]string(nil), state.GeneratedFiles...)
	return info, nil
}

func ReadLatestBuildManifest(appRoot string) (*LatestBuildManifest, bool, error) {
	path := LatestBuildPath(appRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var manifest LatestBuildManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, false, err
	}
	return &manifest, true, nil
}

func WriteLatestBuildManifest(result *Result, phase string) error {
	if result == nil {
		return fmt.Errorf("nil build result")
	}
	if result.AppRoot == "" {
		return fmt.Errorf("missing app root for latest build manifest")
	}
	state, err := ReadStateInfo(result.AppRoot, result.AppName)
	if err != nil {
		return err
	}
	manifest := LatestBuildManifest{
		SchemaVersion: "pulse.build.latest.v1",
		App: LatestBuildManifestApp{
			Name:       result.AppName,
			ID:         result.AppID,
			Root:       result.AppRoot,
			ConfigPath: filepath.Join(result.AppRoot, "pulse.app"),
		},
		Build: LatestBuildManifestRecord{
			Phase:                 phase,
			WorkspaceDir:          result.Dir,
			BinaryPath:            result.Binary,
			WorkspaceExists:       pathExists(result.Dir),
			BinaryExists:          pathExists(result.Binary),
			BuildStatePath:        state.Path,
			BuildStateExists:      state.Exists,
			BuildStateVersion:     state.Version,
			DependencyFingerprint: state.DependencyFingerprint,
			GraphFingerprint:      state.GraphFingerprint,
			MetadataPresent:       state.MetadataPresent,
			APIEncodingPresent:    state.APIEncodingPresent,
			SourceFileCount:       len(state.SourceFiles),
			GeneratedFileCount:    len(state.GeneratedFiles),
		},
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFileIfChanged(filepath.Dir(LatestBuildPath(result.AppRoot)), filepath.Base(LatestBuildPath(result.AppRoot)), data)
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func sanitizeWorkspaceLabel(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func removeUnexpectedFilesFromLists(root string, sourceFiles, generatedFiles []string) error {
	keepFiles := make(map[string]struct{}, len(sourceFiles)+len(generatedFiles)+2)
	keepDirs := map[string]struct{}{
		".": {},
	}
	for _, rel := range append(append([]string(nil), sourceFiles...), generatedFiles...) {
		rel = filepath.ToSlash(rel)
		keepFiles[rel] = struct{}{}
		dir := filepath.Dir(rel)
		for dir != "." && dir != "/" {
			keepDirs[filepath.ToSlash(dir)] = struct{}{}
			dir = filepath.Dir(dir)
		}
	}
	keepFiles["pulse-app"] = struct{}{}
	keepFiles[buildStateFile] = struct{}{}
	keepFiles["go.sum"] = struct{}{}

	var files []string
	var dirs []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			dirs = append(dirs, path)
			return nil
		}
		if _, ok := keepFiles[rel]; ok {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return err
	}
	for _, path := range files {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	sort.Slice(dirs, func(i, j int) bool {
		return len(dirs[i]) > len(dirs[j])
	})
	for _, path := range dirs {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if _, ok := keepDirs[filepath.ToSlash(rel)]; ok {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, fs.ErrExist) {
			var pathErr *fs.PathError
			if errors.As(err, &pathErr) && errors.Is(pathErr.Err, fs.ErrExist) {
				continue
			}
			if strings.Contains(err.Error(), "directory not empty") {
				continue
			}
			return err
		}
	}
	return nil
}

func dependencyFingerprintFromWorkspace(root string) (string, error) {
	h := sha256.New()
	if data, err := os.ReadFile(filepath.Join(root, "go.mod")); err == nil {
		_, _ = h.Write([]byte("go.mod\x00"))
		_, _ = h.Write(data)
	}
	if data, err := os.ReadFile(filepath.Join(root, "go.sum")); err == nil {
		_, _ = h.Write([]byte("go.sum\x00"))
		_, _ = h.Write(data)
	}
	var goFiles []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel != "." && shouldSkipDir(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ".go" {
			goFiles = append(goFiles, rel)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(goFiles)
	for _, rel := range goFiles {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return "", err
		}
		imports, err := goImports(data)
		if err != nil {
			return "", err
		}
		_, _ = h.Write([]byte(rel))
		_, _ = h.Write([]byte{0})
		for _, imp := range imports {
			_, _ = h.Write([]byte(imp))
			_, _ = h.Write([]byte{0})
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func goImports(src []byte) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	imports := make([]string, 0, len(file.Imports))
	for _, imp := range file.Imports {
		imports = append(imports, strings.Trim(imp.Path.Value, `"`))
	}
	sort.Strings(imports)
	return imports, nil
}

func loadBuildState(root string) (buildState, error) {
	path := filepath.Join(root, buildStateFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return buildState{}, nil
		}
		return buildState{}, err
	}
	var state buildState
	if err := json.Unmarshal(data, &state); err != nil {
		return buildState{}, err
	}
	return state, nil
}

func LoadCachedGraph(appRoot, appName, graphFingerprint string) (*CachedGraph, bool, error) {
	root, err := workspaceDir(appRoot, appName)
	if err != nil {
		return nil, false, err
	}
	state, err := loadBuildState(root)
	if err != nil {
		return nil, false, err
	}
	if state.Version != buildStateVersion {
		return nil, false, nil
	}
	if state.GraphFingerprint == "" || state.GraphFingerprint != graphFingerprint {
		return nil, false, nil
	}
	if _, err := os.Stat(filepath.Join(root, "pulse_internal_main", "main.go")); err != nil {
		return nil, false, nil
	}
	result := &Result{
		AppRoot:               appRoot,
		AppName:               appName,
		Dir:                   root,
		Binary:                filepath.Join(root, "pulse-app"),
		NeedsTidy:             false,
		DependencyFingerprint: state.DependencyFingerprint,
		GraphFingerprint:      state.GraphFingerprint,
		Metadata:              append(json.RawMessage(nil), state.Metadata...),
		APIEncoding:           append(json.RawMessage(nil), state.APIEncoding...),
		SourceFiles:           append([]string(nil), state.SourceFiles...),
		GeneratedFiles:        append([]string(nil), state.GeneratedFiles...),
	}
	return &CachedGraph{
		Result:      result,
		Metadata:    append(json.RawMessage(nil), state.Metadata...),
		APIEncoding: append(json.RawMessage(nil), state.APIEncoding...),
	}, true, nil
}

func RefreshCachedWorkspace(appRoot string, result *Result) (bool, error) {
	if result == nil {
		return false, fmt.Errorf("nil build result")
	}
	generated := make(map[string]struct{}, len(result.GeneratedFiles))
	for _, rel := range result.GeneratedFiles {
		rel = filepath.ToSlash(rel)
		generated[rel] = struct{}{}
		if _, err := os.Stat(filepath.Join(result.Dir, filepath.FromSlash(rel))); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return false, nil
			}
			return false, err
		}
	}
	sourceFiles, err := syncAllSourceFiles(result.Dir, appRoot, generated)
	if err != nil {
		return false, err
	}
	result.SourceFiles = sourceFiles
	if err := removeUnexpectedFilesFromLists(result.Dir, result.SourceFiles, result.GeneratedFiles); err != nil {
		return false, err
	}
	depFingerprint, err := dependencyFingerprintFromWorkspace(result.Dir)
	if err != nil {
		return false, err
	}
	result.NeedsTidy = result.DependencyFingerprint != depFingerprint
	result.DependencyFingerprint = depFingerprint
	return true, nil
}

func saveBuildState(root string, state buildState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, buildStateFile), data, 0o644)
}

func syncGeneratedFiles(root, appRoot string, gen *codegen.Output, prev, sourceFiles []string) ([]string, error) {
	next := make(map[string][]byte, len(gen.Rewritten)+len(gen.Generated))
	for rel, data := range gen.Rewritten {
		rel = filepath.ToSlash(rel)
		if filepath.Ext(rel) == ".go" {
			var err error
			data, err = rewriteEncoreCompat(filepath.Join(appRoot, rel), data)
			if err != nil {
				return nil, err
			}
		}
		next[rel] = data
	}
	for rel, data := range gen.Generated {
		next[filepath.ToSlash(rel)] = data
	}
	for rel, data := range next {
		if err := writeFileIfChanged(root, rel, data); err != nil {
			return nil, err
		}
	}
	for _, rel := range prev {
		rel = filepath.ToSlash(rel)
		if _, ok := next[rel]; ok {
			continue
		}
		if slices.Contains(sourceFiles, rel) {
			continue
		}
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(rel))); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	paths := make([]string, 0, len(next))
	for rel := range next {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	return paths, nil
}

func isSourceFile(rel string) bool {
	return true
}

func sortedKeys(set map[string]struct{}) []string {
	paths := make([]string, 0, len(set))
	for rel := range set {
		paths = append(paths, filepath.ToSlash(rel))
	}
	sort.Strings(paths)
	return paths
}

func rewriteEncoreCompat(path string, src []byte) ([]byte, error) {
	text := string(src)
	needsCronRewrite := strings.Contains(text, "encore.dev/cron")
	needsRlogRewrite := strings.Contains(text, "encore.dev/rlog")
	needsAuthRewrite := strings.Contains(text, "encore.dev/beta/auth")
	needsErrsRewrite := strings.Contains(text, "encore.dev/beta/errs")
	needsMiddlewareRewrite := strings.Contains(text, "encore.dev/middleware")
	needsPubSubRewrite := strings.Contains(text, "encore.dev/pubsub")
	needsPGXPoolRewrite := strings.Contains(text, "github.com/jackc/pgx/v5/pgxpool")
	needsRootRewrite := strings.Contains(text, "\"encore.dev\"")
	if !needsCronRewrite && !needsRlogRewrite && !needsAuthRewrite && !needsErrsRewrite && !needsMiddlewareRewrite && !needsPubSubRewrite && !needsPGXPoolRewrite && !needsRootRewrite {
		return src, nil
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	changed := false
	if rewriteImportPath(file, "encore.dev/rlog", "pulse.dev/rlog", "") {
		changed = true
	}
	if rewriteImportPath(file, "encore.dev/cron", "pulse.dev/cron", "") {
		changed = true
	}
	if rewriteImportPath(file, "encore.dev/beta/auth", "pulse.dev/auth", "") {
		changed = true
	}
	if rewriteImportPath(file, "encore.dev/beta/errs", "pulse.dev/errs", "") {
		changed = true
	}
	if rewriteImportPath(file, "encore.dev/middleware", "pulse.dev/middleware", "") {
		changed = true
	}
	if rewriteImportPath(file, "encore.dev/pubsub", "pulse.dev/pubsub", "") {
		changed = true
	}
	if rewriteImportPath(file, "github.com/jackc/pgx/v5/pgxpool", "pulse.dev/pgxpool", "") {
		changed = true
	}
	if rewriteImportPath(file, "encore.dev", "pulse.dev", "encore") {
		changed = true
	}

	if !changed {
		return src, nil
	}

	out, err := format.Source(renderAST(fset, file))
	if err != nil {
		return nil, err
	}
	return out, nil
}

func rewriteImportPath(file *ast.File, oldPath, newPath, alias string) bool {
	changed := false
	for _, imp := range file.Imports {
		if strings.Trim(imp.Path.Value, "\"") != oldPath {
			continue
		}
		imp.Path.Value = fmt.Sprintf("%q", newPath)
		if alias != "" && imp.Name == nil {
			imp.Name = ast.NewIdent(alias)
		}
		changed = true
	}
	return changed
}

func renderAST(fset *token.FileSet, file *ast.File) []byte {
	var buf strings.Builder
	_ = format.Node(&buf, fset, file)
	return []byte(buf.String())
}
