# onlava Doctor Command

This ExecPlan is a living document. Update Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective as work proceeds.

## Purpose / Big Picture

Add a new top-level CLI command, `onlava doctor`, that explains whether the local machine is ready for onlava development. The command should be fast, read-only, deterministic, and useful both to humans and agents.

Today onlava has strong app-facing validation through `onlava check --json`, repo validation through `onlava harness self --json --write`, and build/version reporting through `onlava version --json`. Those commands answer different questions. `onlava doctor` should answer the environment question before a developer or agent spends time debugging an app failure that is really caused by a missing toolchain, unsupported OS, low disk space, insufficient memory, or an optional local-development dependency that is absent.

After this work, a contributor can run:

```sh
onlava doctor
onlava doctor --json
onlava doctor --app-root /path/to/app --json
```

and get a concise report covering OS/architecture, CPU count, RAM, disk space for the working/app/cache paths, the onlava binary version, the Go command version, and relevant external command dependencies such as `bun`, `psql`, `pg_dump`, `docker`, `atlas`, and `sqlc` when they are relevant. The command must not install tools, mutate trust stores, start services, connect to databases, download binaries, or run app builds. It diagnoses; it does not repair.

## Progress

- [x] 2026-06-01: Created this ExecPlan for `onlava doctor` as `docs/plans/0060-onlava-doctor-command.md` and linked it from `docs/plans/active.md`.
- [x] 2026-06-01: Implemented `cmd/onlava/doctor.go` with text and JSON output, `--app-root`, `--json`, required/optional check semantics, and dependency-injected collection tests.
- [x] 2026-06-01: Added platform resource probes for Unix disk, Darwin/Linux memory, Windows disk/memory, and unsupported-platform fallback, with fake-probe unit tests for thresholds.
- [x] 2026-06-01: Added `docs/schemas/onlava.doctor.result.v1.schema.json`, CLI usage, local-contract docs, README guidance, agent docs, and self-harness schema validation coverage.
- [x] 2026-06-01: Validation passed for `go test ./cmd/onlava`, focused doctor tests, `go test ./...`, `go install ./cmd/onlava`, `git diff --check`, `onlava inspect docs --json`, Linux/Windows `cmd/onlava` compile checks, and manual `onlava doctor` text/JSON/fixture-app smoke checks.
- [x] 2026-06-01: Re-ran `onlava harness self --json --write` after changing the default timing budget to advisory. The self-harness passed with `ok: true`; the full Go suite timing was reported as a warning at 8.260s over the 7.000s target.

## Surprises & Discoveries

- 2026-06-01: `docs/plans/0059-frozen-toolchain-manifest.md` already exists, so this plan uses the next permanent historical ID, `0060`.
- 2026-06-01: `cmd/onlava/main.go` dispatches top-level commands through a single `run(args []string)` switch and embeds usage text in `usageError()`. `doctor` should be added there rather than introducing a command framework.
- 2026-06-01: `cmd/onlava/check.go` has the right testable command shape for new CLI work: `checkCommand` calls a `runOnlavaCheck(ctx, stdout, args)` helper, JSON failures use `silentCLIError`, and JSON output is indented through `json.Encoder`.
- 2026-06-01: `cmd/onlava/version.go` already exposes `buildVersionResponse()` with onlava version, commit, build time, build Go version, and module version. `doctor` should reuse that payload instead of duplicating version metadata.
- 2026-06-01: `go.mod` declares `go 1.26.3`; the human README currently states Go 1.26+. The doctor Go check should make the minimum version explicit in docs/tests and avoid hiding any future drift between the module directive and README.
- 2026-06-01: `github.com/shirou/gopsutil/v4` and `golang.org/x/sys` are already in the module graph indirectly. The first implementation should still prefer the standard library and tiny OS-specific helpers; only promote an indirect dependency to direct if it materially simplifies reliable cross-platform RAM/disk probing.
- 2026-06-01: `onlava doctor --json` from the onlava repo root currently returns `ok: true` with 11 ok checks on this machine; because the repo root itself is not an onlava app, the app field is omitted and disk checks cover cwd plus the onlava cache root.
- 2026-06-01: The self-harness schema validation test writes a tiny fake repo with an explicit schema allowlist. Adding a new schema-validation payload requires adding the schema path to `writeHarnessSelfRepo` and updating the expected validated count.
- 2026-06-01: Cross-compiling `cmd/onlava` for Windows exposed that `golang.org/x/sys/windows` does not expose `MemoryStatusEx` helpers directly. The Windows memory probe now calls `kernel32!GlobalMemoryStatusEx` through a small local struct.
- 2026-06-01: The target-app harness inspect parallelization left over from the stopped duration experiment could race SQLite metrics inspection with trace inspection and fail `TestRunOnlavaHarnessJSONSuccessWritesLatest` with `SQLITE_BUSY`. Restoring serial inspect execution fixed `go test ./cmd/onlava`.
- 2026-06-01: The only doctor-plan self-harness failure was the pre-existing wall-clock duration budget. Following the user's direction to approach harness duration differently, default self-harness timing is now advisory while release-mode checks can still enforce the total budget.
- 2026-06-01: The onlava repo root is not itself an onlava app root. `onlava doctor` from the repo root correctly returns environment-only output; app-root validation is covered by a tiny fixture app with `.onlava.json`.

## Decision Log

- Decision: Make `doctor` a top-level command, not an `inspect` subcommand.
  Rationale: `inspect` reports on onlava apps and generated state. `doctor` reports on the host environment and dependency readiness, and should be runnable before app discovery succeeds.
  Date/Author: 2026-06-01 / OpenAI assistant

- Decision: Keep the initial flag surface small: `onlava doctor [--app-root <path>] [--json]`.
  Rationale: The command needs a stable baseline contract before adding filtering, fixing, or strict modes. A small surface is easier to document, test, and keep agent-friendly.
  Date/Author: 2026-06-01 / OpenAI assistant

- Decision: Do not run `onlava check`, `go build`, package managers, Docker probes that start containers, database connections, or network checks from `doctor`.
  Rationale: A doctor command should be safe and quick in broken environments. App compilation already belongs to `onlava check`; local runtime validation belongs to `onlava dev`, `onlava logs`, and harness commands.
  Date/Author: 2026-06-01 / OpenAI assistant

- Decision: Optional tools should warn, not fail, unless the discovered app configuration makes them required for an explicitly configured feature.
  Rationale: README requirements already treat Bun and `psql` as conditional. Failing a minimal Go-service environment because optional UI, database shell, or generator tools are absent would create noise.
  Date/Author: 2026-06-01 / OpenAI assistant

- Decision: Text output should be plain, stable ASCII by default; JSON is the automation contract.
  Rationale: onlava already prefers machine-readable JSON surfaces for agents. Text should be easy to read in any terminal without depending on color support, Unicode rendering, or TTY detection.
  Date/Author: 2026-06-01 / OpenAI assistant

- Decision: Promote `golang.org/x/sys` to a direct dependency for resource probes.
  Rationale: reliable disk and physical-memory data needs platform syscalls. `x/sys` was already in the module graph indirectly and provides small, OS-specific syscall wrappers without shelling out to locale- and distribution-dependent commands.
  Date/Author: 2026-06-01 / OpenAI assistant

- Decision: Validate the doctor JSON schema in focused tests and self-harness with a synthetic payload, not a real host probe.
  Rationale: real doctor output depends on host tools, memory, disk, and app-root discovery. A synthetic payload gives stable schema coverage while command tests cover collection behavior through injected probes.
  Date/Author: 2026-06-01 / OpenAI assistant

## Outcomes & Retrospective

Completed. `onlava doctor` is implemented, documented, schema-validated, and covered by focused tests plus manual CLI smoke checks. Default self-harness now keeps full-suite duration telemetry as warnings rather than blocking unrelated feature work on wall-clock variance, and `onlava harness self --json --write` passes with the complete Go suite still running.

## Context and Orientation

Relevant files for the implementation:

```text
cmd/onlava/main.go
cmd/onlava/check.go
cmd/onlava/check_test.go
cmd/onlava/version.go
cmd/onlava/version_test.go
cmd/onlava/psql.go
cmd/onlava/harness_schema.go
docs/local-contract.md
docs/schemas/
README.md
AGENTS.md
SKILL.md
PLANS.md
docs/plans/active.md
```

The command should follow existing CLI conventions:

- Add `case "doctor": return doctorCommand(args[1:])` to `run(args []string)` in `cmd/onlava/main.go`.
- Add a usage line under the stable/dev command list: `onlava doctor [--app-root <path>] [--json]`.
- Implement `doctorCommand(args []string) error` as a thin wrapper around `runOnlavaDoctor(context.Background(), os.Stdout, args)` so tests can capture output.
- Keep JSON mode quiet on stderr. When the report has error-level checks, write the JSON payload and return `&silentCLIError{err: ...}` so `main()` exits non-zero without appending a second human error line.
- Use `buildVersionResponse()` from `version.go` for onlava binary metadata.
- Use `resolveAppRoot()` and `appcfg.DiscoverRoot()` only when `--app-root` is provided or when discovering the current directory would add useful app-sensitive checks. A missing `.onlava.json` should not make plain `onlava doctor` fail.

Terms used in this plan:

- A **required check** is a baseline condition needed for ordinary onlava development. Missing or invalid required checks make `ok: false` and produce exit code 1.
- An **optional check** is a dependency needed only for a specific feature, such as Bun for dashboard UI or TypeScript workers, `psql` for database shell commands, or Docker for image-backed dev services. Missing optional checks produce warnings and exit code 0 unless app configuration makes the tool required.
- An **app-sensitive check** is a check whose severity depends on `.onlava.json`. For example, `sqlc` and `atlas` are optional globally, but become more important when `generators.sqlc` is configured.

## Milestones

### Milestone 1: Command skeleton and output contract

Create `cmd/onlava/doctor.go` and `cmd/onlava/doctor_test.go`. Implement argument parsing, response types, text rendering, JSON rendering, and exit semantics.

Acceptance:

- `onlava doctor` prints a deterministic plain-text summary with one line per check and a final count of errors/warnings/skipped checks.
- `onlava doctor --json` prints a single JSON object with schema version `onlava.doctor.result.v1`.
- Unknown flags and missing `--app-root` values return errors consistent with other onlava commands.
- Unit tests cover parse errors, text rendering, JSON rendering, and non-zero exit behavior for required failures.

Suggested JSON shape:

```json
{
  "schema_version": "onlava.doctor.result.v1",
  "ok": true,
  "summary": {
    "ok": 8,
    "warnings": 2,
    "errors": 0,
    "skipped": 1
  },
  "onlava": {
    "schema_version": "onlava.version.v1",
    "version": "dev",
    "go_version": "go1.26.3"
  },
  "app": {
    "root": "/repo/app",
    "config_path": "/repo/app/.onlava.json",
    "name": "myapp",
    "id": "myapp-dev"
  },
  "environment": {
    "goos": "linux",
    "goarch": "amd64",
    "num_cpu": 8,
    "total_memory_bytes": 34359738368,
    "paths": [
      {
        "path": "/repo/app",
        "kind": "app_root",
        "free_bytes": 1234567890,
        "total_bytes": 9876543210
      }
    ]
  },
  "checks": [
    {
      "id": "tool.go",
      "category": "dependency",
      "name": "Go toolchain",
      "status": "ok",
      "severity": "required",
      "message": "go1.26.3 found at /usr/local/go/bin/go",
      "suggested_action": "",
      "observed": {
        "path": "/usr/local/go/bin/go",
        "version": "go1.26.3"
      }
    }
  ]
}
```

The exact field set may change during implementation, but keep these invariants: schema version, top-level `ok`, counted summary, onlava version payload, environment facts, and ordered check records with stable IDs.

### Milestone 2: Host resource checks

Add host probes for OS/architecture, CPU count, RAM, and disk space. Keep the collection layer separate from the rendering layer.

Acceptance:

- `os.runtime` reports `runtime.GOOS` and `runtime.GOARCH` and warns on untested platforms rather than panicking.
- `resource.cpu` reports `runtime.NumCPU()` and warns when the machine has fewer than two logical CPUs.
- `resource.memory` reports total physical memory when available. If a platform-specific probe is not implemented, emit `skipped` with a clear message instead of guessing from current process memory.
- `resource.disk.<kind>` checks free space for the current working directory or app root and for the onlava cache/state directory when that path can be resolved.
- Disk thresholds are documented in code and tests. Start with warning below 5 GiB and error below 1 GiB for app/cache paths unless implementation evidence suggests better thresholds.
- Tests use fake probe functions and do not assert against the developer or CI machine's real CPU, RAM, or disk.

Implementation notes:

- Prefer small OS-specific files such as `doctor_resource_unix.go`, `doctor_resource_windows.go`, and `doctor_resource_other.go` over shelling out to `df`, `wmic`, `sysctl`, or PowerShell.
- Use `golang.org/x/sys/unix` or `golang.org/x/sys/windows` only if the standard library cannot provide reliable cross-platform disk/RAM data. If imported directly, move the dependency from indirect to direct in `go.mod` and explain that in this plan's Decision Log.
- Do not use current Go process heap metrics as a substitute for total system RAM. That would be misleading.

### Milestone 3: Dependency checks

Add dependency probes that use `exec.LookPath` and tightly bounded commands where version output matters.

Baseline checks:

- `tool.go`: required. Locate `go`, run `go version`, parse the toolchain version, and require at least Go 1.26 unless docs/go.mod alignment leads to a stricter single-source constant.
- `tool.bun`: optional unless app config or discovered TypeScript worker/frontend workflows make it relevant.
- `tool.psql`: optional, required only for explicit database shell/snapshot flows.
- `tool.pg_dump`: optional, required only for database snapshot create flows.
- `tool.docker`: optional, relevant for Docker-backed managed Postgres/Electric and some generator dev URLs.
- `tool.atlas`: optional globally, relevant when configured SQLC schema refresh uses Atlas source files.
- `tool.sqlc`: optional globally, relevant when `generators.sqlc` is configured.
- `tool.git`: optional, useful for source checkouts and release/debug metadata, but should not block app development if the installed binary and Go toolchain are otherwise usable.

Acceptance:

- Missing `go` is an error with a suggested action.
- A Go version below the supported minimum is an error.
- Optional missing tools are warnings with feature-scoped messages, not generic failure noise.
- Dependency command execution has a short context timeout and never invokes commands that can install, download, start daemons, or prompt.
- Tests inject fake `LookPath` and command-runner functions. No test depends on `go`, `bun`, `psql`, Docker, Atlas, or SQLC being installed on the test host.

### Milestone 4: App-sensitive context

When an app root is available, discover `.onlava.json` and use its configuration to tune check severity and messages.

Acceptance:

- `onlava doctor --app-root <path>` reports app name, app ID, root, and config path when discovery succeeds.
- A missing or invalid app root is an error only when `--app-root` was explicitly provided.
- Plain `onlava doctor` from outside an app still succeeds and reports environment-only checks.
- SQLC/Atlas checks become higher-signal when `generators.sqlc` is configured.
- Bun checks mention dashboard UI, benchmark fixture, and TypeScript workers only when relevant evidence exists.
- Database tool checks mention `onlava db psql`, `onlava psql`, and snapshot commands without implying the app must use a database.

### Milestone 5: Documentation and schemas

Update human and agent-facing documentation in the same change as the command.

Acceptance:

- `docs/local-contract.md` lists `onlava doctor --json` under implemented JSON surfaces and classifies it as dev-only/agent-DX until the team decides it belongs in stable v0.
- `docs/local-contract.md` defines the command grammar, exit semantics, check statuses, severity values, and JSON schema path.
- Add `docs/schemas/onlava.doctor.result.v1.schema.json` for the JSON response.
- Update `cmd/onlava/harness_schema.go` to validate a representative doctor payload when practical. If real host probing would make harness validation flaky, validate the schema in focused doctor tests with synthetic payloads instead and record the reason here.
- Update `README.md` requirements/CLI overview with `onlava doctor` as the first diagnostic command to run after install.
- Update `AGENTS.md` and `SKILL.md` so agents prefer `onlava doctor --json` before expensive troubleshooting when environment readiness is in doubt.

## Plan of Work

Start by adding the command skeleton, response model, and rendering tests. This makes the intended contract visible before the platform-specific probes are added. Then add resource probes behind interfaces so unit tests can exercise low-memory, low-disk, missing-tool, and unsupported-platform cases without relying on the current host.

Once the response model is stable, wire dependency probes and app-sensitive severity rules. Keep all checks read-only and quick. Finish by updating CLI usage, local contract docs, the JSON schema, README, agent instructions, and this plan. Validate with focused tests first, then run the full repository validation commands.

The implementation should be additive. If a platform probe is uncertain, mark that check as skipped with a clear message rather than blocking the entire command. A partial but truthful doctor report is more useful than a fragile command that fails on less common environments.

## Concrete Steps

1. Add `doctorOptions`, `doctorResponse`, `doctorSummary`, `doctorEnvironment`, `doctorPathReport`, and `doctorCheck` types in `cmd/onlava/doctor.go`.
2. Implement `parseDoctorArgs(args []string) (doctorOptions, error)` with support for `--app-root` and `--json`.
3. Add `doctorCommand(args []string) error` and `runOnlavaDoctor(ctx context.Context, stdout io.Writer, args []string) error`.
4. Wire `doctor` into `cmd/onlava/main.go` command dispatch and usage text.
5. Implement a check collector that accepts dependencies for path lookup, command execution, app discovery, and resource probing.
6. Reuse `buildVersionResponse()` for the top-level `onlava` field.
7. Add OS/architecture and CPU checks with no platform-specific files.
8. Add disk and RAM platform probes. Use `skipped` for unsupported platforms rather than fake values.
9. Add dependency probes for `go`, `bun`, `psql`, `pg_dump`, `docker`, `atlas`, `sqlc`, and `git`.
10. Add Go version parsing/comparison helpers with table-driven tests for normal versions, prerelease/devel strings, malformed output, and old versions.
11. Add app-root discovery and app-sensitive severity/message rules.
12. Add text rendering with stable ordering and a concise summary.
13. Add JSON rendering and schema tests.
14. Add `docs/schemas/onlava.doctor.result.v1.schema.json`.
15. Update `docs/local-contract.md`, `README.md`, `AGENTS.md`, `SKILL.md`, and any relevant docs index/knowledge metadata if the docs index expects the new surface.
16. Update this ExecPlan's Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective after implementation and validation.

## Validation and Acceptance

Focused validation:

```sh
go test ./cmd/onlava -run 'Test(ParseDoctor|RunOnlavaDoctor|Doctor|GoVersion|Disk|Memory)'
go test ./cmd/onlava
```

Repository validation:

```sh
go test ./...
go install ./cmd/onlava
git diff --check
onlava harness self --json --write
```

Manual CLI validation from the repository root after install:

```sh
onlava doctor
onlava doctor --json
onlava doctor --app-root <fixture-app> --json
```

Manual app validation with a tiny fixture app:

```sh
app=$(mktemp -d)
cat > "$app/.onlava.json" <<'JSON'
{"name":"doctorfixture"}
JSON
cat > "$app/go.mod" <<'EOF'
module example.com/doctorfixture

go 1.26.3
EOF
onlava doctor --app-root "$app" --json
```

A successful implementation satisfies these observable outcomes:

- Missing or old Go produces `ok: false`, an error-level `tool.go` check, and exit code 1.
- Missing optional tools produce warning-level checks and exit code 0 when no required checks fail.
- Low disk space and low memory produce deterministic warning/error statuses based on documented thresholds.
- JSON output is valid against `docs/schemas/onlava.doctor.result.v1.schema.json`.
- Text output is concise enough for humans but does not become the automation contract.
- The command is safe to run repeatedly in any directory because it is read-only and does not start or mutate services.

## Idempotence and Recovery

`onlava doctor` must be idempotent. It reads host facts, looks up commands, optionally reads `.onlava.json`, and optionally executes short version commands. It must not write `.onlava/`, modify caches, update trust stores, start agents, start Docker containers, run migrations, or create database snapshots.

If an OS-specific resource probe fails, the collector should add a `skipped` check with the error message and continue. If app discovery fails and `--app-root` was explicit, report a required app-root error. If app discovery fails during implicit current-directory discovery, continue with environment-only checks.

If a future implementation adds `--strict`, `--check <id>`, or `--fix`, that work should update this ExecPlan or create a follow-up plan. The initial implementation should not include repair behavior.

## Artifacts and Notes

Expected new or changed artifacts:

```text
cmd/onlava/doctor.go
cmd/onlava/doctor_test.go
cmd/onlava/doctor_resource_unix.go
cmd/onlava/doctor_resource_darwin.go
cmd/onlava/doctor_resource_linux.go
cmd/onlava/doctor_resource_windows.go
cmd/onlava/doctor_resource_other.go
docs/schemas/onlava.doctor.result.v1.schema.json
docs/local-contract.md
README.md
AGENTS.md
SKILL.md
cmd/onlava/main.go
cmd/onlava/harness_schema.go
```

Potential text output shape:

```text
onlava doctor

ok      onlava.version      onlava dev built with go1.26.3
ok      os.runtime          linux/amd64
ok      resource.cpu        8 logical CPUs
ok      resource.memory     32 GiB total memory
ok      resource.disk.root  120 GiB free at /repo/app
ok      tool.go             go1.26.3 at /usr/local/go/bin/go
warn    tool.bun            bun not found; only needed for dashboard UI, benchmark, or TypeScript worker work
warn    tool.psql           psql not found; only needed for onlava psql/db shell workflows

summary: 6 ok, 2 warnings, 0 errors, 0 skipped
```

Keep check IDs stable once documented. Agents and tests should be able to key off IDs such as `tool.go`, `resource.disk.app_root`, and `resource.memory` without scraping messages.

## Interfaces and Dependencies

Proposed internal interfaces:

```go
type doctorProbeDeps struct {
    LookPath func(file string) (string, error)
    RunCommand func(ctx context.Context, name string, args ...string) ([]byte, error)
    ResourceProbe doctorResourceProbe
    DiscoverApp func(start string) (doctorAppInfo, bool, error)
}

type doctorResourceProbe interface {
    Runtime() doctorRuntimeInfo
    Memory(ctx context.Context) (doctorMemoryInfo, error)
    Disk(ctx context.Context, path string) (doctorDiskInfo, error)
}
```

The exact shape can change, but keep dependency injection at the collector boundary so tests do not mutate package globals or depend on the host.

External dependencies and policy:

- Prefer the Go standard library for command lookup, version execution, JSON, runtime facts, and file paths.
- Use direct `golang.org/x/sys` imports only if needed for reliable platform disk/RAM probes. It is already in the module graph indirectly, but direct imports must still be intentional and recorded.
- Avoid shelling out for resource data. Shell output differs too much across Linux distributions, macOS versions, Windows locales, and CI images.
- Do not introduce a CLI framework or colored-output dependency.
- Do not call package managers, Docker APIs, network endpoints, database endpoints, or `go` subcommands that can download modules or toolchains. `go version` is acceptable; `go env`, `go list`, and `go build` are not part of the doctor baseline.
