# Frozen Toolchain Manifest and Managed Tool Store
This ExecPlan is a living document. Keep `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` current as work proceeds. Maintain this file according to `PLANS.md`.
## Purpose / Big Picture
Onlava should have a root-level, checked-in toolchain manifest that freezes every Onlava-owned external executable, image, plugin, and source lock reference for the current Onlava version.
A contributor, release binary, or CI job should be able to answer:
1. Which external tools belong to this Onlava version?
2. Which exact binary versions should be used?
3. Which Docker images are allowed?
4. Which source dependency lock files define Go, UI, or generated-runtime dependencies?
5. Where are local controlled binaries installed?
6. Which command downloads or verifies the required local toolchain?
7. Whether the current machine is accidentally relying on a system `PATH` binary.
After this change, bumping a managed tool version in the manifest changes the artifact key. The next `onlava toolchain sync`, or the next command that needs that tool, downloads the matching binary or verifies/pulls the matching Docker image into an Onlava-owned location.
Onlava must not silently use binaries found in the system `PATH`. The allowed sources are:
- the Onlava-managed toolchain store;
- explicit per-tool env overrides;
- explicit external-service URLs where the command is configured to reuse an external service;
- manifest-pinned Docker images.
The user-visible surface is:
    onlava toolchain list --json
    onlava toolchain sync --json
    onlava toolchain verify --json
    onlava toolchain path --tool <name>
The root manifest is:
    onlava.toolchain.json
The default local store is:
    <app-root>/.onlava/toolchain/
The global override is:
    ONLAVA_TOOLCHAIN_DIR=/some/controlled/path
Downloaded binaries, extracted tool homes, image presence metadata, and installation metadata are local state and must not be committed.
## Progress
- [x] (2026-06-01 02:05Z) Initial `toolchain`-named ExecPlan drafted.
- [x] (2026-06-01 09:02Z) Add root toolchain manifest and schema.
- [x] (2026-06-01 09:02Z) Add managed toolchain resolver and artifact store.
- [x] (2026-06-01 09:02Z) Add `onlava toolchain` CLI commands.
- [x] (2026-06-01 09:02Z) Migrate Grafana and Victoria to the managed resolver.
- [x] (2026-06-01 09:02Z) Audit Temporal, Postgres, Electric, frontend worker runtimes, and other dev services for implicit `PATH` or unpinned Docker image resolution.
- [x] (2026-06-01 09:02Z) Add docs, schemas, environment contract updates, and knowledge index updates.
- [x] (2026-06-01 09:02Z) Add tests and self-harness coverage.
- [ ] (YYYY-MM-DD HH:MMZ) Complete validation and update retrospective.
## Surprises & Discoveries
- Observation: Onlava already has a partial internal pin file at `internal/devtools/versions.json`.
  Evidence: It currently pins Grafana, Grafana plugins, VictoriaMetrics, VictoriaLogs, and VictoriaTraces.
- Observation: The existing internal pin file is not enough for the desired contract.
  Evidence: It is not root-level, is named as internal devtool state, does not describe download URLs per platform, does not describe Docker images, does not expose source locks, and does not define the local artifact store contract.
- Observation: `internal/devtools/versions.go` embeds and parses `internal/devtools/versions.json`.
  Evidence: The parser validates the existing internal schema and exposes helper functions such as `PinnedVersions()` and `GrafanaPluginPreinstallSync()`.
- Observation: Victoria binary resolution currently allows implicit system `PATH` fallback.
  Evidence: `cmd/onlava/victoria.go` resolves explicit env override, then a local bin directory, then `exec.LookPath`, then download.
- Observation: Grafana binary resolution currently allows implicit system `PATH` fallback.
  Evidence: `cmd/onlava/grafana.go` resolves explicit env override and local paths, probes versions, and can fall back to `PATH`.
- Observation: `.onlava/` is already ignored.
  Evidence: `.gitignore` ignores `.onlava/`, so `.onlava/toolchain/` is appropriate for local downloaded state.
- Observation: Temporal CLI release assets include platform tarballs and a checksum sidecar.
  Evidence: `https://github.com/temporalio/cli/releases/download/v1.7.0/checksums.txt` lists `temporal_cli_1.7.0_linux_amd64.tar.gz` and `temporal_cli_1.7.0_darwin_arm64.tar.gz`.
- Observation: The old `internal/devtools/versions.json` pin file can be deleted instead of generated as compatibility output.
  Evidence: `internal/devtools.PinnedVersions()` now derives Grafana, plugin, and Victoria versions from the bundled `onlava.toolchain.json` manifest while preserving the old Go API.
- Observation: Docker image refs remain tag-only in this migration step.
  Evidence: `onlava toolchain verify --strict --images --json` exits non-zero and reports `status: "invalid"` for Victoria and Postgres image refs because they intentionally have no digest yet and are marked `stability: "unstable"`.
- Observation: Source package-manager lookups are distinct from managed toolchain artifacts.
  Evidence: remaining `bun`, `npm`, `node`, and `tsx` lookups are used for app-local scripts, frontends, UI builds, or generated TypeScript workers and are documented as source/package-manager tooling rather than hidden Onlava-managed downloads.
- Observation: The remaining validation failure is unrelated to the toolchain contract and comes from the existing self-harness Go timing gate.
  Evidence: after replacing the duplicate `TestRunHarnessParallelDevStep` real parallel-dev check with a fast wrapper test, `onlava harness self --json --write` still reports only `full Go suite took 8.911s, over 7.000s target`; the real parallel-dev validation still runs as its own self-harness step.
## Decision Log
- Decision: Use **toolchain** as the user-facing noun.
  Rationale: `deps` and `dependencies` are overloaded with Go modules, npm packages, and generated package locks. `toolchain` better describes Onlava-controlled executables, images, plugins, and local runtime tools.
  Date/Author: 2026-06-01 / Codex.
- Decision: Add the root manifest as `onlava.toolchain.json`.
  Rationale: The user wants root-level visibility and per-Onlava-version freezing. The file should be obvious to humans and agents inspecting the repository root.
  Date/Author: 2026-06-01 / Codex.
- Decision: Use schema version `onlava.toolchain.v1`.
  Rationale: Onlava already uses versioned machine-readable contracts. The toolchain manifest should be validated and evolvable.
  Date/Author: 2026-06-01 / Codex.
- Decision: Use `.onlava/toolchain/` as the default local store.
  Rationale: `.onlava/` is already ignored local state. A dedicated `toolchain` subtree is explicit and separates managed tools from generated app metadata, build artifacts, harness output, Victoria state, and other local runtime files.
  Date/Author: 2026-06-01 / Codex.
- Decision: Use `ONLAVA_TOOLCHAIN_DIR` as the global store override.
  Rationale: Operators and agents need full control over where binaries live, including shared cache directories and hermetic CI workspaces.
  Date/Author: 2026-06-01 / Codex.
- Decision: Use `ONLAVA_TOOLCHAIN_DOWNLOAD=0` as the global automatic-download disable switch.
  Rationale: Some environments must be offline or audit downloads explicitly. A global switch gives deterministic failure instead of surprise network access.
  Date/Author: 2026-06-01 / Codex.
- Decision: Remove implicit `PATH` fallback for managed toolchain artifacts.
  Rationale: The user explicitly wants Onlava to avoid system/path binaries even when available. Managed tools should resolve from explicit env override, Onlava store, or manifest-driven download.
  Date/Author: 2026-06-01 / Codex.
- Decision: Keep explicit per-tool binary env overrides.
  Rationale: Explicit env variables such as `ONLAVA_GRAFANA_BIN` are deliberate control points. They are acceptable because they are not silent system discovery.
  Date/Author: 2026-06-01 / Codex.
- Decision: Treat Go modules and UI package-manager files as `source_locks`, not managed toolchain downloads.
  Rationale: `go.mod`, `go.sum`, `package.json`, and package lock files already freeze source dependency graphs. The toolchain manifest should reference and report those lock surfaces without duplicating the entire dependency graph.
  Date/Author: 2026-06-01 / Codex.
- Decision: Make the release binary expose the bundled toolchain manifest SHA.
  Rationale: Each Onlava release should be auditable. `onlava version --json` should prove which toolchain manifest the binary was built with.
  Date/Author: 2026-06-01 / Codex.
- Decision: Leave tag-only image refs as explicit unstable migration metadata for this change.
  Rationale: Digest-pinning every image is desirable, but strict verification already fails on tag-only refs and the manifest exposes the instability instead of hiding it.
  Date/Author: 2026-06-01 / Codex.
## Outcomes & Retrospective
Not yet completed. The implementation, docs, focused tests, full `go test ./...`, `go install ./cmd/onlava`, toolchain smoke checks, isolated `temporal-cli` sync, docs inspection, and `git diff --check` have been run. `onlava harness self --json --write` still reports `ok: false` because the full Go suite exceeded the repo-wide 7s harness timing target; after fixing the `SKILL.md` capability phrase and removing duplicate real parallel-dev work from the unit test path, the remaining self-harness next action points to the existing `docs/plans/0050-test-suite-speed-hardening.md` work.
## Context and Orientation
Onlava is a Go-native service runtime and local development platform. App roots are marked by `.onlava.json`. The CLI starts local development services, generated app processes, observability sidecars, managed Postgres/Electric services, Temporal workers, frontends, and dashboards.
The relevant repository files and packages are:
- `go.mod`
  - Go module dependency manifest.
- `go.sum`
  - Go module dependency lock file.
- `ui/package.json`
  - Dashboard/UI package manifest, if present in the working tree.
- UI lock file, such as `ui/bun.lock`, `ui/bun.lockb`, or another current lock file
  - UI package dependency lock surface.
- `.goreleaser.yaml`
  - Onlava CLI release build configuration.
- `internal/devtools/versions.json`
  - Current internal pin file for Grafana, Grafana plugins, and Victoria components.
- `internal/devtools/versions.go`
  - Current parser and embedded accessor for the internal devtool version file.
- `cmd/onlava/victoria.go`
  - Starts and downloads VictoriaMetrics, VictoriaLogs, and VictoriaTraces for local observability.
- `cmd/onlava/grafana.go`
  - Starts and downloads Grafana for local observability.
- `cmd/onlava/temporal_dev.go`
  - Starts local Temporal dev server when configured.
- Postgres managed-service code
  - Starts or reuses managed Postgres for dev services.
- Electric managed-service code
  - Starts or reuses Electric for dev services, possibly through binary or Docker image paths.
- `docs/local-contract.md`
  - CLI, JSON, artifact path, and stability contract.
- `docs/environment.md`
  - Onlava-owned environment variables.
- `docs/agent-guide.md`
  - Agent-facing Onlava workflow docs.
- `SKILL.md`
  - Installable Onlava skill for agents working inside Onlava apps.
- `docs/schemas/`
  - JSON schemas for machine-readable Onlava contracts.
- `docs/knowledge.json`
  - Indexed documentation metadata.
- `.gitignore`
  - Local-state policy. `.onlava/` is already ignored.
Current behavior already pins some devtool versions, but the pinning is implementation-private and incomplete. The new toolchain contract must be root-visible, release-frozen, schema-validated, and used by runtime startup paths.
The term **toolchain artifact** means an Onlava-managed external thing that Onlava may need to run local development or runtime support commands. Examples include Grafana, VictoriaMetrics, VictoriaLogs, VictoriaTraces, Temporal CLI, Electric binaries or images, Postgres images, and plugins.
The term **source lock** means an existing dependency lock surface such as `go.sum` or a UI lock file. Source locks are listed in `onlava.toolchain.json` for inventory and release audit purposes, but their dependency graph remains owned by the native package manager.
## Milestones
### Milestone 1: Add root toolchain manifest and schema
Add:
    onlava.toolchain.json
    docs/schemas/onlava.toolchain.v1.schema.json
The initial manifest should migrate the information from `internal/devtools/versions.json` and add enough metadata for deterministic local resolution.
The first manifest should cover:
- Grafana
- Grafana plugins
- VictoriaMetrics
- VictoriaLogs
- VictoriaTraces
- Temporal CLI, if Onlava owns auto-starting it
- Electric binary or Docker image, if Onlava owns starting it
- Postgres image or binary channel, if Onlava owns starting it
- any generated TypeScript worker runtime tooling that Onlava installs or invokes directly
- source locks:
  - `go.mod`
  - `go.sum`
  - `ui/package.json`
  - the current UI lock file, if present
Proposed manifest shape:
    {
      "schema_version": "onlava.toolchain.v1",
      "manifest_version": 1,
      "source_locks": [
        {
          "name": "go-modules",
          "kind": "go-modules",
          "manifest": "go.mod",
          "lock": "go.sum"
        },
        {
          "name": "dashboard-ui",
          "kind": "package-manager",
          "manager": "bun",
          "manifest": "ui/package.json",
          "lock": "ui/bun.lock"
        }
      ],
      "artifacts": [
        {
          "name": "victoria-metrics",
          "kind": "binary",
          "version": "v1.141.0",
          "license": "Apache-2.0",
          "default_binary": "victoria-metrics-prod",
          "binaries": ["victoria-metrics-prod", "victoria-metrics"],
          "platforms": {
            "linux/amd64": {
              "archive": "tar.gz",
              "url": "https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v1.141.0/victoria-metrics-linux-amd64-v1.141.0.tar.gz",
              "sha256": "<fill-before-enforce>",
              "extract": "victoria-metrics-prod"
            },
            "darwin/arm64": {
              "archive": "tar.gz",
              "url": "https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v1.141.0/victoria-metrics-darwin-arm64-v1.141.0.tar.gz",
              "sha256": "<fill-before-enforce>",
              "extract": "victoria-metrics-prod"
            }
          },
          "images": [
            {
              "ref": "victoriametrics/victoria-metrics:v1.141.0",
              "digest": "sha256:<fill-before-enforce>",
              "optional": true
            }
          ]
        }
      ]
    }
Schema rules:
- Reject unknown fields.
- Require non-empty `schema_version`.
- Require exact schema version `onlava.toolchain.v1`.
- Require non-empty artifact names.
- Require valid artifact kinds.
- Require non-empty versions for managed artifacts.
- Require valid platform keys in `goos/goarch` format.
- Require non-empty URLs for downloadable platform artifacts.
- Require SHA-256 checksums for enforced downloads.
- Require extraction targets for archive artifacts.
- Require image refs for image artifacts.
- Prefer image digests when Docker images are managed.
- Allow temporary `checksum_status: "pending"` only behind an explicit development-only test path, not in final accepted release state.
### Milestone 2: Add internal toolchain package
Create:
    internal/toolchain
This package owns:
- parsing `onlava.toolchain.json`;
- validating schema version;
- rejecting unknown fields;
- computing manifest SHA-256;
- resolving current platform from `runtime.GOOS` and `runtime.GOARCH`;
- selecting artifacts by name;
- selecting platform downloads;
- computing local install paths;
- downloading archives;
- verifying SHA-256 before extraction;
- extracting only expected files;
- rejecting archive path traversal;
- making extracted binaries executable;
- writing install metadata;
- verifying existing installs;
- reporting machine-readable status;
- optionally verifying or pulling Docker images through an injectable Docker runner.
The default store layout should be versioned and platform-specific:
    .onlava/toolchain/
      manifest/
        onlava.toolchain.sha256
      artifacts/
        victoria-metrics/
          v1.141.0/
            linux-amd64/
              bin/
                victoria-metrics-prod
              archive/
                victoria-metrics-linux-amd64-v1.141.0.tar.gz
              install.json
        grafana/
          13.0.1+security-01/
            darwin-arm64/
              home/
              bin/
                grafana
              install.json
      images/
        index.json
The package should expose APIs shaped like:
    type Manifest struct { ... }
    type Store struct { ... }
    type ArtifactStatus struct { ... }
    func ParseManifest(data []byte) (Manifest, error)
    func LoadBundledManifest() (Manifest, error)
    func BundledManifestBytes() []byte
    func BundledManifestSHA256() string
    func DefaultStoreDir(appRoot string) string
    func NewStore(dir string, manifest Manifest) (*Store, error)
    func (s *Store) List(ctx context.Context, opts ListOptions) (Status, error)
    func (s *Store) Sync(ctx context.Context, opts SyncOptions) (Status, error)
    func (s *Store) Verify(ctx context.Context, opts VerifyOptions) (Status, error)
    func (s *Store) Path(ctx context.Context, artifactName string, platform Platform) (PathStatus, error)
Installation must be atomic:
1. create temp directory under the target store;
2. download archive to temp file;
3. verify SHA-256;
4. extract into temp directory;
5. verify expected binary paths;
6. chmod executable files;
7. rename into final version/platform directory;
8. write `install.json`.
`install.json` should look like:
    {
      "schema_version": "onlava.toolchain.install.v1",
      "name": "victoria-metrics",
      "version": "v1.141.0",
      "platform": "linux/amd64",
      "manifest_sha256": "...",
      "source_url": "...",
      "source_sha256": "...",
      "installed_at": "..."
    }
Retries must be safe. Partial temp directories should be deleted or ignored on the next run.
### Milestone 3: Embed the release-frozen manifest
The Onlava binary must know which toolchain manifest it was built with.
Preferred implementation:
- Add `internal/toolchain/manifest_gen.go`.
- Generate it from root `onlava.toolchain.json`.
- The generated file embeds exact manifest bytes and manifest SHA.
- The generator validates the manifest and writes deterministic Go code.
Possible generator locations:
    internal/toolchain/generate.go
or:
    internal/cmd/gentoolchain/main.go
Expose:
    toolchain.BundledManifest() toolchain.Manifest
    toolchain.BundledManifestBytes() []byte
    toolchain.BundledManifestSHA256() string
Update:
    onlava version --json
to include:
    {
      "toolchain_manifest": {
        "schema_version": "onlava.toolchain.v1",
        "sha256": "...",
        "artifact_count": 7,
        "source_lock_count": 2
      }
    }
This makes every Onlava release auditable: the binary, root manifest, and checked-in source contract line up.
### Milestone 4: Add `onlava toolchain` CLI
Add:
    cmd/onlava/toolchain.go
    cmd/onlava/toolchain_test.go
CLI shape:
    onlava toolchain list [--json] [--include-source-locks]
    onlava toolchain sync [--json] [--all] [--tool <name>] [--platform <goos/goarch>] [--images]
    onlava toolchain verify [--json] [--all] [--tool <name>] [--platform <goos/goarch>] [--images] [--strict]
    onlava toolchain path [--json] --tool <name> [--platform <goos/goarch>]
Behavior:
- `list` reports bundled manifest entries and local install status.
- `sync` downloads missing or invalid managed binaries.
- `sync --images` verifies or pulls manifest-declared Docker images.
- `verify` checks local files without downloading.
- `verify --strict` fails on missing checksums, tag-only image refs, or any artifact with incomplete metadata.
- `path` prints the exact Onlava-managed binary path for a tool.
- `--platform` defaults to current `runtime.GOOS/runtime.GOARCH`.
- `ONLAVA_TOOLCHAIN_DIR` overrides the store root.
- `ONLAVA_TOOLCHAIN_DOWNLOAD=0` disables automatic downloads.
- Existing per-tool download-disable env vars should either be routed through this resolver or explicitly deprecated in docs if they are redundant.
JSON status shape:
    {
      "schema_version": "onlava.toolchain.status.v1",
      "manifest_sha256": "...",
      "store_dir": "/repo/.onlava/toolchain",
      "platform": "darwin/arm64",
      "source_locks": [
        {
          "name": "go-modules",
          "kind": "go-modules",
          "manifest": "go.mod",
          "lock": "go.sum",
          "status": "present"
        }
      ],
      "artifacts": [
        {
          "name": "grafana",
          "kind": "binary",
          "version": "13.0.1+security-01",
          "status": "installed",
          "source": "managed-store",
          "managed_path": ".../.onlava/toolchain/artifacts/grafana/13.0.1+security-01/darwin-arm64/bin/grafana"
        }
      ]
    }
### Milestone 5: Replace implicit binary resolution in Grafana and Victoria
Update:
    cmd/onlava/victoria.go
    cmd/onlava/grafana.go
Resolution order for managed tools must be:
1. explicit per-tool env override, such as `ONLAVA_GRAFANA_BIN`;
2. managed toolchain store path;
3. automatic manifest-driven download into the managed store, unless disabled;
4. clear error telling the user which `onlava toolchain sync ...` command to run.
Forbidden for managed tools:
    exec.LookPath("grafana")
    exec.LookPath("grafana-server")
    exec.LookPath("victoria-metrics")
    exec.LookPath("victoria-metrics-prod")
    exec.LookPath("victoria-logs")
    exec.LookPath("victoria-logs-prod")
    exec.LookPath("victoria-traces")
    exec.LookPath("victoria-traces-prod")
Allowed:
- explicit env override pointing to a binary;
- managed store binary path;
- explicit external service URL where documented;
- manifest-pinned Docker image ref where Docker mode is selected.
Keep the existing Grafana and Victoria startup behavior where possible, but change the binary source. The code should report the selected source in structured dev events where practical:
    source: "explicit-env"
    source: "managed-store"
    source: "downloaded"
    source: "external-service"
Do not silently reuse a process that happens to be on the expected port unless the existing external-service reuse path verifies compatibility and the docs explain it.
### Milestone 6: Audit Temporal, Postgres, Electric, and generated worker tooling
Run:
    rg 'exec\.LookPath|execLookPath|LookPath|docker|postgres|initdb|temporal|electric|bun|npm|node|tsx' cmd internal
Classify every hit:
1. Onlava-managed toolchain artifact.
2. Explicit user-provided external tool.
3. Source package-manager command.
4. Test-only fake.
5. Not relevant.
For each Onlava-managed artifact:
- add it to `onlava.toolchain.json`;
- use `internal/toolchain` for resolution;
- remove implicit `PATH` fallback;
- add tests for fake `PATH` poisoning.
For package-manager/runtime commands such as `bun`, `npm`, `node`, or `tsx`, decide explicitly:
- If Onlava only invokes the project’s chosen package manager, document it as a source dependency, not an Onlava-managed toolchain artifact.
- If Onlava downloads or installs a hidden runtime, put it in the toolchain manifest.
Temporal local dev server currently uses a Temporal CLI executable. If Onlava continues to auto-start local Temporal, prefer making Temporal CLI a managed toolchain artifact. If the decision is not to manage it, remove auto-start expectations or require an explicit `ONLAVA_TEMPORAL_BIN`.
Postgres local startup may use `initdb` and `postgres` binaries or Docker. If Onlava owns startup, those binaries/images must be manifest-controlled or explicit. No implicit `PATH`.
Electric local startup may use a local binary or Docker image. If Onlava owns startup, those binaries/images must be manifest-controlled or explicit. No implicit image tag or implicit `PATH`.
### Milestone 7: Add Docker image control
Extend the toolchain manifest for images:
    {
      "name": "postgres",
      "kind": "image",
      "version": "18",
      "images": [
        {
          "ref": "postgres:18",
          "digest": "sha256:...",
          "usage": "dev.services.postgres",
          "optional": true
        }
      ]
    }
Runtime rules:
- If the manifest has a digest, prefer digest-pinned pull/run refs.
- If only a tag is present during migration, mark the image metadata as `unstable`.
- `onlava toolchain verify --strict --images` fails on tag-only image refs.
- `onlava toolchain list --json --images` reports whether images are locally present.
- `onlava toolchain sync --images` pulls missing images when Docker is available.
- If Docker is unavailable and the image is optional, report `unavailable` with a clear message.
- If Docker is required for a selected dev-service mode, fail before startup with the manifest artifact name and expected image ref.
- Docker execution must go through an injectable runner so unit tests do not need Docker.
### Milestone 8: Remove or demote `internal/devtools/versions.json`
There must be only one canonical version source.
Preferred outcome:
- Delete `internal/devtools/versions.json`.
- Replace `internal/devtools.PinnedVersions()` with a small compatibility adapter over `internal/toolchain`.
- Remove the adapter once all callers use `internal/toolchain` directly.
Acceptable temporary outcome:
- Keep `internal/devtools/versions.json` only as generated compatibility output from `onlava.toolchain.json`.
- Add a test proving it is generated and not hand-edited.
Forbidden outcome:
- Two independent manually maintained version files.
### Milestone 9: Update docs, schemas, and agent guidance
Update:
    docs/local-contract.md
Add:
- `onlava toolchain list --json`
- `onlava toolchain sync --json`
- `onlava toolchain verify --json`
- `onlava toolchain path --json`
- `.onlava/toolchain/`
- `onlava.toolchain.json`
- `onlava.toolchain.v1`
- `onlava.toolchain.status.v1`
Update:
    docs/environment.md
Add:
- `ONLAVA_TOOLCHAIN_DIR`
- `ONLAVA_TOOLCHAIN_DOWNLOAD`
- explicit per-tool binary override vars
- statement that managed toolchain artifacts do not use implicit system `PATH`
Update:
    docs/agent-guide.md
Add agent guidance:
- run `onlava toolchain list --json` to inspect required tools;
- run `onlava toolchain sync --json` before local dev if managed tools are missing;
- do not install global Grafana/Victoria/Temporal/Postgres/Electric binaries as a hidden fix;
- prefer explicit env override or managed toolchain store.
Update:
    SKILL.md
Add target-app guidance:
- Onlava-managed tools live under `.onlava/toolchain/`.
- Agents should not rely on system binaries.
- Use `onlava toolchain verify --json` when diagnosing local dev tool issues.
Update:
    README.md
Add a concise human-facing section explaining:
- root frozen toolchain manifest;
- local controlled toolchain store;
- how to sync;
- how to override store location.
Update:
    docs/knowledge.json
Add or update indexed documentation entries for the new toolchain docs.
Add:
    docs/schemas/onlava.toolchain.v1.schema.json
    docs/schemas/onlava.toolchain.status.v1.schema.json
### Milestone 10: Tests and harness
Add tests for `internal/toolchain`:
- parses a valid manifest;
- rejects unknown fields;
- rejects missing versions;
- rejects invalid platform keys;
- rejects missing URLs;
- rejects missing checksums for enforced downloads;
- computes stable manifest SHA;
- resolves current platform;
- computes deterministic store paths;
- verifies installed artifacts;
- handles partial temp install recovery;
- rejects archive path traversal;
- preserves executable bits;
- reports missing artifacts without downloading during verify.
Add CLI tests for `cmd/onlava`:
- `onlava toolchain list --json` emits the expected schema;
- `onlava toolchain verify --json` reports missing artifacts without downloading;
- `onlava toolchain sync --json` uses a fake HTTP server and fake archive;
- `onlava toolchain path --tool <name> --json` reports managed path;
- `ONLAVA_TOOLCHAIN_DIR` changes the store root;
- `ONLAVA_TOOLCHAIN_DOWNLOAD=0` disables automatic downloads.
Add resolver integration tests:
- Put fake `grafana` and `victoria-metrics` executables earlier in `PATH`.
- Ensure managed resolver ignores them.
- Ensure explicit env override still wins.
- Ensure missing managed binary triggers download or clear failure.
- Ensure structured status reports the source.
Add Docker tests:
- Use a fake Docker runner interface.
- Verify digest-pinned refs are preferred.
- Verify optional images degrade cleanly when Docker is unavailable.
- Verify `--strict` fails on tag-only image refs.
Add docs/harness tests:
- update any schema index validation;
- update `onlava harness self --json --write` expected docs knowledge if needed;
- ensure `PLANS.md` required sections are satisfied.
## Plan of Work
Start by creating the root manifest and schema. Do not change runtime behavior in the first step. This gives tests a stable contract and allows review of naming, metadata shape, and completeness.
Next, build `internal/toolchain` as a standalone package. It should not know about Grafana, Victoria, Temporal, Postgres, or Electric specifically. It should only know how to parse a manifest, resolve a platform artifact, manage a local store, verify files, download archives, and report status.
Then add `onlava toolchain` CLI commands. This gives contributors and agents an explicit inspection and sync surface before runtime commands start relying on it.
Then migrate Grafana and Victoria. They are the safest first users because Onlava already has pinned versions and download logic for them. Replace their bespoke version and binary resolution with `internal/toolchain`, while preserving startup behavior, explicit env overrides, and structured dev events.
Then audit Temporal, Postgres, Electric, and generated worker tooling. Each implicit binary/image dependency must become manifest-managed, explicit-only, or documented as a source package-manager dependency.
Then update docs, schemas, and knowledge index. Onlava treats machine-readable contracts and docs as part of the implementation.
Finally, run focused tests, full tests, binary install, CLI smoke checks, and self-harness validation.
## Concrete Steps
From the Onlava repository root:
    cd /path/to/onlava
Create the ExecPlan and link it:
    $EDITOR docs/plans/0057-frozen-toolchain-manifest.md
    $EDITOR docs/plans/active.md
Inspect current docs and contracts:
    onlava inspect docs --json
Add the root manifest:
    $EDITOR onlava.toolchain.json
Add schemas:
    $EDITOR docs/schemas/onlava.toolchain.v1.schema.json
    $EDITOR docs/schemas/onlava.toolchain.status.v1.schema.json
Create the internal package:
    mkdir -p internal/toolchain
    $EDITOR internal/toolchain/manifest.go
    $EDITOR internal/toolchain/platform.go
    $EDITOR internal/toolchain/store.go
    $EDITOR internal/toolchain/download.go
    $EDITOR internal/toolchain/archive.go
    $EDITOR internal/toolchain/docker.go
    $EDITOR internal/toolchain/status.go
    $EDITOR internal/toolchain/manifest_test.go
    $EDITOR internal/toolchain/store_test.go
    $EDITOR internal/toolchain/download_test.go
    $EDITOR internal/toolchain/archive_test.go
    $EDITOR internal/toolchain/docker_test.go
Add manifest embedding or generation:
    $EDITOR internal/toolchain/manifest_gen.go
    $EDITOR internal/toolchain/generate.go
or:
    mkdir -p internal/cmd/gentoolchain
    $EDITOR internal/cmd/gentoolchain/main.go
Run generation if applicable:
    go generate ./internal/toolchain
Add CLI:
    $EDITOR cmd/onlava/toolchain.go
    $EDITOR cmd/onlava/toolchain_test.go
Migrate or remove old internal devtool pins:
    $EDITOR internal/devtools/versions.go
    $EDITOR internal/devtools/versions.json
The target state is no second hand-maintained canonical version file.
Migrate Victoria:
    $EDITOR cmd/onlava/victoria.go
    $EDITOR cmd/onlava/victoria_test.go
Migrate Grafana:
    $EDITOR cmd/onlava/grafana.go
    $EDITOR cmd/onlava/grafana_test.go
Audit remaining implicit resolution:
    rg 'exec\.LookPath|execLookPath|LookPath|docker|postgres|initdb|temporal|electric|bun|npm|node|tsx' cmd internal
Update each Onlava-owned artifact to use `internal/toolchain`.
Update docs:
    $EDITOR docs/local-contract.md
    $EDITOR docs/environment.md
    $EDITOR docs/agent-guide.md
    $EDITOR SKILL.md
    $EDITOR README.md
    $EDITOR docs/knowledge.json
Format:
    gofmt -w internal/toolchain internal/devtools cmd/onlava
Run focused tests:
    go test ./internal/toolchain ./internal/devtools ./cmd/onlava
Run broader tests:
    go test ./...
Install the binary:
    go install ./cmd/onlava
Run toolchain smoke checks:
    onlava toolchain list --json
    onlava toolchain verify --json
Use an isolated store to prove sync behavior:
    tmp="$(mktemp -d)"
    ONLAVA_TOOLCHAIN_DIR="$tmp" onlava toolchain sync --tool victoria-metrics --json
    ONLAVA_TOOLCHAIN_DIR="$tmp" onlava toolchain verify --tool victoria-metrics --json
Use a poisoned `PATH` to prove managed tools do not resolve from system binaries:
    mkdir -p /tmp/onlava-fake-path
    printf '#!/bin/sh\necho fake >&2\nexit 99\n' >/tmp/onlava-fake-path/grafana
    chmod +x /tmp/onlava-fake-path/grafana
    PATH="/tmp/onlava-fake-path:$PATH" ONLAVA_TOOLCHAIN_DIR="$(mktemp -d)" onlava toolchain sync --tool grafana --json
Run self-harness:
    onlava harness self --json --write
Check diff hygiene:
    git diff --check
    git status --short
## Validation and Acceptance
Acceptance criteria:
1. `onlava.toolchain.json` exists at the repository root.
2. `onlava.toolchain.json` validates against `docs/schemas/onlava.toolchain.v1.schema.json`.
3. `onlava toolchain list --json` reports:
   - schema version;
   - manifest SHA-256;
   - store directory;
   - platform;
   - managed artifacts;
   - source locks;
   - local install status.
4. `onlava toolchain sync --tool <name> --json` downloads the selected artifact into `.onlava/toolchain/` or `ONLAVA_TOOLCHAIN_DIR`.
5. Changing an artifact version in `onlava.toolchain.json` changes the computed install path.
6. The next sync after a version bump downloads the new version instead of reusing the old binary.
7. Managed Grafana and Victoria startup no longer call `exec.LookPath` for implicit system binary resolution.
8. Explicit env override still works and status reports `source: "explicit-env"`.
9. A fake binary earlier in `PATH` is ignored by managed resolver tests.
10. Downloads verify SHA-256 before extraction.
11. Extracted archives cannot write outside the intended destination.
12. Docker images used by Onlava-owned dev services are declared in `onlava.toolchain.json`.
13. `onlava toolchain verify --strict --images` fails on tag-only image refs unless the plan explicitly leaves a temporary migration exception with follow-up work.
14. `onlava version --json` includes toolchain manifest metadata.
15. Documentation mentions:
    - `onlava.toolchain.json`;
    - `.onlava/toolchain/`;
    - `ONLAVA_TOOLCHAIN_DIR`;
    - `ONLAVA_TOOLCHAIN_DOWNLOAD`;
    - no implicit `PATH` for managed tools.
Required validation commands:
    go test ./internal/toolchain ./internal/devtools ./cmd/onlava
    go test ./...
    go install ./cmd/onlava
    onlava toolchain list --json
    onlava toolchain verify --json
    onlava harness self --json --write
    git diff --check
## Idempotence and Recovery
`onlava toolchain sync` must be idempotent. Running it repeatedly with the same manifest should not redownload artifacts that are already installed and checksum-valid.
Partial downloads and extractions must use temporary paths. A failed download, checksum mismatch, interrupted extraction, or process kill must not leave a final-looking install directory.
The next run should clean or ignore stale temp paths.
A corrupted installed binary should be detected by:
    onlava toolchain verify --json
A corrupted installed binary should be replaced by:
    onlava toolchain sync --json
A version bump must not mutate old version directories. Old directories may remain until a later cleanup command exists.
Docker image sync should be safe to rerun. Missing Docker should produce structured `unavailable` status for optional images and clear failure for required images.
Explicit env override recovery is manual. If `ONLAVA_GRAFANA_BIN` points to a missing or non-executable file, Onlava should fail with that path and not fall back to `PATH`.
## Artifacts and Notes
Example local status:
    {
      "schema_version": "onlava.toolchain.status.v1",
      "manifest_sha256": "8f...",
      "store_dir": "/repo/.onlava/toolchain",
      "platform": "darwin/arm64",
      "source_locks": [
        {
          "name": "go-modules",
          "kind": "go-modules",
          "manifest": "go.mod",
          "lock": "go.sum",
          "status": "present"
        }
      ],
      "artifacts": [
        {
          "name": "victoria-metrics",
          "kind": "binary",
          "version": "v1.141.0",
          "status": "installed",
          "source": "managed-store",
          "managed_path": "/repo/.onlava/toolchain/artifacts/victoria-metrics/v1.141.0/darwin-arm64/bin/victoria-metrics-prod"
        }
      ]
    }
Example failure when downloads are disabled:
    onlava: toolchain artifact victoria-metrics v1.141.0 for darwin/arm64 is not installed under .onlava/toolchain
    onlava: automatic downloads are disabled by ONLAVA_TOOLCHAIN_DOWNLOAD=0
    onlava: run `onlava toolchain sync --tool victoria-metrics` or set ONLAVA_VICTORIA_METRICS_BIN to an explicit binary
Example failure when an implicit system binary exists but no managed binary is installed:
    onlava: managed toolchain artifact grafana is not installed
    onlava: system PATH binaries are not used for managed toolchain artifacts
    onlava: run `onlava toolchain sync --tool grafana` or set ONLAVA_GRAFANA_BIN explicitly
The exact wording may change, but the behavior must not.
## Interfaces and Dependencies
New interfaces:
- `onlava.toolchain.json`
  - Root checked-in manifest.
  - Freezes Onlava-owned managed artifacts and source lock references for the current source/release version.
- `docs/schemas/onlava.toolchain.v1.schema.json`
  - Schema for the root manifest.
- `docs/schemas/onlava.toolchain.status.v1.schema.json`
  - Schema for CLI status output if this output is declared stable or beta.
- `internal/toolchain`
  - Internal parser, resolver, downloader, verifier, Docker image checker, and status package.
- `onlava toolchain list --json`
  - Machine-readable inventory and local status.
- `onlava toolchain sync --json`
  - Managed artifact installer and optional image puller.
- `onlava toolchain verify --json`
  - Local toolchain verification.
- `onlava toolchain path --json --tool <name>`
  - Exact managed path lookup.
- `.onlava/toolchain/`
  - Default local toolchain store.
  - Ignored local state.
- `ONLAVA_TOOLCHAIN_DIR`
  - Override for the toolchain store root.
- `ONLAVA_TOOLCHAIN_DOWNLOAD`
  - Global control for automatic downloads.
Existing interfaces to preserve deliberately:
- Explicit per-tool binary overrides, such as `ONLAVA_GRAFANA_BIN` and Victoria component `*_BIN` variables.
- Existing Grafana/Victoria enable/disable controls, routed through the new resolver where relevant.
- Existing external-service URL controls, where they represent deliberate external reuse rather than implicit local discovery.
Interfaces to remove or forbid for managed toolchain artifacts:
- Implicit `exec.LookPath` fallback.
- Implicit unpinned Docker image tags.
- Unversioned install locations that allow a version bump to accidentally reuse an old binary.
- Two independent hand-maintained version manifests.
Source lock policy:
- Go module dependencies stay frozen by `go.mod` and `go.sum`.
- UI/package-manager dependencies stay frozen by their package lock files.
- `onlava.toolchain.json` references these lock files and reports them through `onlava toolchain list --json`.
- `onlava.toolchain.json` does not duplicate full Go or package-manager dependency graphs unless a later plan adds generated lock summaries.

The key naming choices are now:

onlava.toolchain.json
.onlava/toolchain/
ONLAVA_TOOLCHAIN_DIR
ONLAVA_TOOLCHAIN_DOWNLOAD
onlava toolchain list
onlava toolchain sync
onlava toolchain verify
onlava toolchain path
