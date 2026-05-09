# Standard Auth and Data Tenant Permissions

This ExecPlan is a living document. Update Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective as work proceeds.

## Purpose / Big Picture

onlava now has standard auth and a dynamic data platform. This plan connects them so data tenants can map to auth tenants/organizations and record access can be enforced consistently.

The goal:

```text
auth tenant/org/user
      |
      v
data tenant
      |
      v
permission hook
      |
      v
row/field/object access
```

## Progress

- [ ] Create ExecPlan.
- [ ] Audit standard auth tenant model.
- [ ] Design data tenant mapping.
- [ ] Implement default permission provider.
- [ ] Add tests and docs.

## Surprises & Discoveries

Record discoveries here.

## Decision Log

Record decisions here.

## Outcomes & Retrospective

Fill when complete.

## Context and Orientation

Relevant files:

```text
auth/*
data/data.go
internal/objectstore/*
runtime/*
testdata/apps/data-platform
docs/local-contract.md
```

## Scope

Implement:

```text
ActorFromContext tenant awareness
data tenant key from auth claims
object-level permission default
field-level permission default
row filter hook integration
```

Non-goals:

```text
full RBAC UI
policy language
ABAC engine
external authorization service
```

## Milestones

### Milestone 1: Tenant mapping

Define how `auth.Data()` tenant/org claims map to `data.TenantKey`.

### Milestone 2: Default permissions

Add optional standard-auth-aware permission provider.

### Milestone 3: Tests

Verify:

```text
user can access own tenant
user cannot access other tenant
row filters apply
field restrictions apply
live events respect permissions
```

## Interfaces and Dependencies

- Use standard auth request state where available.
- Keep data permission hooks explicit; do not add a policy language in this plan.
- Live event filtering must use the same permission model as record queries.
- Default allow-all behavior should remain available for apps that opt out of standard auth integration.

## Plan of Work

Map standard auth tenant/org claims to a data tenant key, then implement a default permission provider and verify query, mutation, and live-event paths.

## Concrete Steps

1. Audit standard auth tenant and organization data.
2. Define the mapping to data tenant keys.
3. Add actor/context helpers.
4. Implement a standard-auth-aware permission provider.
5. Apply row and field filters consistently.
6. Add tests for cross-tenant denial and live-event filtering.

## Validation and Acceptance

```sh
ONLAVA_TEST_DATABASE_URL=... go test ./auth ./data ./internal/objectstore
go test ./...
go install ./cmd/onlava
onlava harness self --json --write
```

Acceptance criteria:

```text
- data access can be tied to standard auth
- live events respect auth-derived permissions
- default remains explicit and understandable
```

## Idempotence and Recovery

Permission checks should not mutate data. If tenant mapping is ambiguous, fail closed with a clear error rather than guessing.

## Artifacts and Notes

Expected artifacts include data/auth integration tests, docs, and local-contract classification.
