# Pulse Local Contract

This document freezes the local developer and agent-facing contract for Pulse v0.

The goal is to make Pulse deterministic and inspectable:
- app shape is explicit
- CLI grammar is explicit
- machine-readable JSON outputs have versioned schemas
- generated and cache artifact locations are named, even where some paths are still reserved for upcoming work

If implementation and this document disagree, treat that as a bug.

## Status

Implemented now:
- `pulse.app`
- `pulse run --json`
- `pulse admin traces clear --json`
- `pulse admin pubsub clear --json`
- `pulse inspect app --json`
- `pulse inspect routes --json`
- `pulse inspect services --json`
- `pulse inspect build --json`
- `pulse inspect paths --json`
- `.pulse/gen/app.json`
- `.pulse/gen/routes.json`
- `.pulse/gen/services.json`
- `.pulse/gen/manifest.json`
- `.pulse/build/latest.json`

Reserved by contract, implementation pending:
- other `pulse admin ... --json` commands beyond `traces clear` and `pubsub clear`
- repo-local runtime and state manifests beyond `.pulse/build/latest.json` and `.pulse/gen/*`

## `pulse.app`

Schema:
- [pulse.app.v1.schema.json](/Users/petrbrazdil/Repos/pulse/docs/schemas/pulse.app.v1.schema.json)

Current shape:

```json
{
  "name": "myapp",
  "id": "myapp-dev",
  "proxy": {
    "workspace": "onlv",
    "api_host": "api.onlv.localhost",
    "console_host": "console.onlv.localhost",
    "mcp_host": "mcp.onlv.localhost",
    "frontend_host": "pulse.onlv.localhost"
  },
  "observability": {
    "logs": {
      "include_endpoints": [],
      "exclude_endpoints": []
    },
    "tracing": {
      "include_endpoints": [],
      "exclude_endpoints": []
    }
  }
}
```

Rules:
- `name` or `id` must be non-empty.
- If `name` is empty, Pulse falls back to `id`.
- `proxy` is optional.
- `observability` is optional.
- Unknown fields are currently ignored by Go JSON decoding, but they are not part of the frozen contract.

## CLI Grammar

Current implemented grammar:

```text
pulse run [--port <n>] [--listen <addr>] [--app-root <path>] [-v|--verbose] [--json]
pulse build [--app-root <path>] [-o <path>] [--db-studio]
pulse check [--app-root <path>]
pulse inspect app|routes|services|build|paths --json [--app-root <path>]
pulse admin traces clear --json [--app-root <path>]
pulse admin pubsub clear --json [--app-root <path>]
pulse logs [--app-root <path>] [--limit <n>] [--stream all|stdout|stderr] [-f|--follow]
pulse test [--app-root <path>] [go test flags/packages...]
pulse gen client [<app-id>] --lang typescript --output <path> [--app-root <path>]
```

Frozen `inspect` rules:
- `pulse inspect` requires a subject.
- `pulse inspect` currently requires `--json`.
- `--app-root` is optional. When omitted, Pulse walks upward from the current working directory to find `pulse.app`.

Implemented `run --json` rules:

```text
pulse run --json
```

- output is JSONL
- each line conforms to `pulse.run.event.v1`
- human-readable console output is suppressed in this mode
- child stdout/stderr are emitted as structured `process.output` events instead of raw terminal writes

Reserved grammar:

```text
pulse admin <subcommand> --json ...
```

Implemented `admin --json` rules:
- current supported commands are `traces clear` and `pubsub clear`
- output conforms to `pulse.admin.result.v1`
- `pubsub clear` requires a running Pulse dashboard/supervisor because it tunnels through the supervisor RPC surface

Any additional admin subcommands are reserved contract surfaces and should produce versioned JSON when implemented.

## Artifact Locations

### Current implemented locations

Use `pulse inspect paths --json` as the source of truth.

Today Pulse uses:
- app config: `<app-root>/pulse.app`
- cache root:
  - `$PULSE_DEV_CACHE_DIR`, if set
  - otherwise OS user cache + `/pulse`
- build workspace: `<cache-root>/build/<sanitized-app-name>-<hash>`
- built app binary: `<workspace>/pulse-app`
- build state: `<workspace>/.pulse-build-state.json`

### Stable repo-local locations

Implemented now:

```text
<app-root>/.pulse/
  gen/
    app.json
    routes.json
    services.json
    manifest.json
  build/
    latest.json
```

Reserved for upcoming work:

```text
<app-root>/.pulse/
  state/
  logs/
```

Rules:
- `app.json`, `routes.json`, and `services.json` mirror the current `pulse inspect ... --json` outputs for those subjects
- `manifest.json` ties the generated inspect artifacts to schema versions, stable artifact paths, and deterministic content hashes
- `build/latest.json` is the stable repo-local pointer to the latest prepared or compiled build workspace
- agents can use either `pulse inspect ... --json` or the corresponding `.pulse/gen/*.json` files
- future implementation should conform to these locations instead of inventing a different layout

## JSON Schemas

Implemented now:
- [pulse.inspect.app.v1.schema.json](/Users/petrbrazdil/Repos/pulse/docs/schemas/pulse.inspect.app.v1.schema.json)
- [pulse.inspect.routes.v1.schema.json](/Users/petrbrazdil/Repos/pulse/docs/schemas/pulse.inspect.routes.v1.schema.json)
- [pulse.inspect.services.v1.schema.json](/Users/petrbrazdil/Repos/pulse/docs/schemas/pulse.inspect.services.v1.schema.json)
- [pulse.inspect.build.v1.schema.json](/Users/petrbrazdil/Repos/pulse/docs/schemas/pulse.inspect.build.v1.schema.json)
- [pulse.inspect.paths.v1.schema.json](/Users/petrbrazdil/Repos/pulse/docs/schemas/pulse.inspect.paths.v1.schema.json)
- [pulse.gen.manifest.v1.schema.json](/Users/petrbrazdil/Repos/pulse/docs/schemas/pulse.gen.manifest.v1.schema.json)
- [pulse.build.latest.v1.schema.json](/Users/petrbrazdil/Repos/pulse/docs/schemas/pulse.build.latest.v1.schema.json)
- [pulse.run.event.v1.schema.json](/Users/petrbrazdil/Repos/pulse/docs/schemas/pulse.run.event.v1.schema.json)
- [pulse.admin.result.v1.schema.json](/Users/petrbrazdil/Repos/pulse/docs/schemas/pulse.admin.result.v1.schema.json)

Reserved now:
- future command-specific admin schemas if `pulse.admin.result.v1` becomes too generic

Schema rules:
- top-level schema field is `schema_version`
- schema names are versioned strings like `pulse.inspect.app.v1`
- additive fields are allowed in future versions only by introducing a new schema version when needed
- consumers should match on `schema_version`, not on command name alone

## Examples

### `pulse inspect app --json`

```json
{
  "schema_version": "pulse.inspect.app.v1",
  "app": {
    "name": "billing",
    "id": "billing-dev",
    "root": "/repo/billing",
    "config_path": "/repo/billing/pulse.app",
    "module_path": "example.com/billing"
  },
  "config": {
    "name": "billing",
    "id": "billing-dev",
    "proxy": {
      "workspace": "billing",
      "api_host": "api.billing.localhost",
      "console_host": "console.billing.localhost",
      "mcp_host": "mcp.billing.localhost",
      "frontend_host": "pulse.billing.localhost"
    },
    "observability": {
      "logs": {
        "include_endpoints": [],
        "exclude_endpoints": []
      },
      "tracing": {
        "include_endpoints": [],
        "exclude_endpoints": []
      }
    }
  },
  "counts": {
    "packages": 3,
    "services": 2,
    "endpoints": 7,
    "middleware": 1,
    "auth_handler": 1
  },
  "services": [
    "auth",
    "users"
  ],
  "auth_handler": {
    "service": "auth",
    "name": "AuthHandler"
  }
}
```

### `pulse inspect build --json`

```json
{
  "schema_version": "pulse.inspect.build.v1",
  "app": {
    "name": "billing",
    "root": "/repo/billing",
    "config_path": "/repo/billing/pulse.app"
  },
  "build": {
    "workspace_dir": "/Users/me/Library/Caches/pulse/build/billing-abcdef0123456789",
    "binary_path": "/Users/me/Library/Caches/pulse/build/billing-abcdef0123456789/pulse-app",
    "workspace_exists": true,
    "binary_exists": true,
    "build_state_path": "/Users/me/Library/Caches/pulse/build/billing-abcdef0123456789/.pulse-build-state.json",
    "build_state_exists": true,
    "build_state_version": "2",
    "dependency_fingerprint": "abc123",
    "graph_fingerprint": "def456",
    "metadata_present": true,
    "api_encoding_present": true,
    "source_file_count": 24,
    "generated_file_count": 6
  }
}
```
