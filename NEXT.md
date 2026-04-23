# Next Version Thoughts

## Goal

Keep Pulse itself in Go, but make Pulse apps language-agnostic so apps can be written in:

- Go
- TypeScript
- Python

## Current Reality

Pulse is currently Go-first in several hard ways:

- parsing is implemented with Go AST and `go/packages`
- the app model stores Go-specific data such as AST nodes, token positions, and Go types
- build and codegen generate and compile a Go workspace
- internal endpoint calls are enforced through Go-specific rewriting and generated helpers
- public runtime packages are Go packages (`pulse.dev/...`)

This means the current architecture does not generalize cleanly to TypeScript and Python.

## Core Direction

Split Pulse into:

- `Pulse Core`
- language adapters

`Pulse Core` stays in Go and owns:

- `pulse run`, `pulse build`, `pulse inspect`, `pulse admin`
- supervisor lifecycle
- local HTTPS/proxy
- dashboard
- tracing and logs
- secrets/env loading
- pubsub
- cron
- stable inspect/build/run/admin contracts

Language adapters become separate frontends:

- `pulse-go`
- `pulse-ts`
- `pulse-py`

Each adapter translates source code into a shared, language-neutral internal model.

## IR

IR means Intermediate Representation.

It should be the common Pulse app model used after parsing, regardless of source language.

Instead of Pulse thinking in Go-specific terms like:

- Go packages
- Go functions
- Go AST nodes
- Go types

it should think in Pulse terms like:

- services
- endpoints
- auth handlers
- middleware
- pubsub topics and subscriptions
- cron jobs
- request and response schemas
- source locations

Example shape:

```json
{
  "service": "users",
  "endpoint": "GetProfile",
  "access": "auth",
  "path": "/users/:id",
  "methods": ["GET"],
  "path_params": [{"name": "id", "type": "string"}],
  "request_schema": null,
  "response_schema": {"name": "Profile"},
  "middleware": ["requireAuth"],
  "source": {
    "file": "users/api.ts",
    "line": 12
  }
}
```

Once Pulse has that IR, most of the system no longer needs to care whether the app came from Go, TS, or Python.

## Recommended Architecture

### 1. Make the app model language-neutral

Replace the current Go-heavy internal model with a neutral IR.

The current model should stop storing things like:

- `*ast.FuncDecl`
- `types.Object`
- `types.Type`
- `token.Pos`

Those details can stay inside language-specific adapters, but should not be the shared application model.

### 2. Move parsing into adapters

Each language gets its own frontend:

- Go adapter uses `go/packages`
- TypeScript adapter uses the TypeScript compiler API or `ts-morph`
- Python adapter uses Python `ast` plus decorators or explicit declarations

Each adapter emits the same IR.

### 3. Stop using Go rewriting as the universal mechanism

The current endpoint-to-endpoint rewrite strategy is fine for Go, but it is not the right universal abstraction for a multi-language system.

The better model is:

- each language gets generated Pulse stubs/helpers
- internal calls use generated language-native clients
- Pulse Core enforces routing, auth, and access rules through a stable runtime contract

### 4. Introduce language SDKs

Instead of only Go packages like `pulse.dev/...`, provide equivalent SDKs:

- Go: `pulse.dev/...`
- TS: `@pulse/runtime`
- Python: `pulse_runtime`

They should expose the same concepts:

- request metadata
- auth/user state
- errors
- middleware
- pubsub
- cron
- secrets/config access

Each SDK should be idiomatic for its language, but conceptually aligned.

### 5. Treat the manifest as the contract

The language adapters should emit a stable manifest and/or IR artifact that drives:

- `pulse inspect`
- dashboard metadata
- client generation
- routing
- admin tooling

This fits the current direction around:

- `.pulse/gen/app.json`
- `.pulse/gen/routes.json`
- `.pulse/gen/services.json`
- `.pulse/gen/manifest.json`

That generated contract should evolve into the main handoff point between adapters and Pulse Core.

## Runtime Model

There are two broad options.

### Option A: Embedded model

Pulse injects or generates language-specific runtime code directly into the app process.

Pros:

- efficient for Go
- simpler for in-process service calls

Cons:

- awkward for TS and Python
- harder to standardize across languages

### Option B: Sidecar/supervisor model

Pulse Core acts as the runtime supervisor, and the language app process registers its services/endpoints/hooks with Pulse.

Pros:

- much better fit for multi-language
- cleaner separation between Pulse Core and app implementation
- easier to reason about cross-language behavior

Cons:

- more architecture work up front
- more explicit adapter protocol needed

Recommended direction: prefer the sidecar/supervisor model long term.

## Suggested Phases

### Phase 1: Extract a neutral IR

- keep Go as the only supported app language
- add a language-neutral IR
- make the Go parser emit IR
- keep current behavior working

### Phase 2: Make Pulse Core consume the IR

- `pulse inspect`, dashboard, metadata, and client generation should consume IR/manifest output
- reduce direct dependence on Go parser internals

### Phase 3: Keep Go as the first adapter

- treat the current Go implementation as `pulse-go`
- keep Go-specific codegen only where actually needed

### Phase 4: Add TypeScript support

- define TS declaration style
- implement TS adapter
- run TS app process under Pulse Core

### Phase 5: Add Python support

- define Python decorator/declaration style
- implement Python adapter
- run Python app process under Pulse Core

## Declaration Style

If multi-language is a real goal, explicit declarations are probably better than comment directives over time.

Comments work in Go, but they are not the ideal long-term cross-language abstraction.

Preferred direction:

- Go can keep comments short term for migration compatibility
- long term, favor explicit declarations/helpers/decorators across all languages

That will be easier for:

- tooling
- agents
- editor integration
- language parity

## What Not To Do

- Do not try to make the current Go parser directly understand multiple languages.
- Do not make TypeScript or Python pretend to be Go inside the current codegen pipeline.
- Do not aim for full feature parity across all languages on day one.

Start with the common high-value surface:

- endpoints
- auth
- middleware
- secrets/config
- inspect/build/run/admin

Then add:

- pubsub
- cron

## Recommendation

If multi-language support is a real target, the correct move is:

1. freeze Go as the first adapter
2. define a proper neutral IR
3. make Pulse Core consume that IR
4. add TypeScript next
5. add Python after that

That is the cleanest path to a language-agnostic Pulse without turning the current Go-specific design into an unmaintainable mess.
