# onlava Data Platform

The `github.com/pbrazdil/onlava/data` package exposes onlava's beta dynamic data platform for Go apps.

It is metadata-driven, but not an ORM. Objects and fields live in PostgreSQL metadata tables, while ordinary scalar fields are backed by real PostgreSQL tables, columns, indexes, foreign keys, and outbox rows.

## Open A Store

```go
store, err := data.Open(ctx, pool, data.Options{})
if err != nil {
	return err
}
actor := data.ActorFromContext(ctx)
```

`pool` can be a `pgxpool.Pool` or any value implementing `data.DB`.

## Objects And Fields

```go
_, err = store.CreateObject(ctx, actor, data.CreateObjectRequest{
	TenantKey:    "acme",
	NameSingular: "company",
	NamePlural:   "companies",
})

_, err = store.CreateField(ctx, actor, "company", data.CreateFieldRequest{
	TenantKey: "acme",
	Name:      "stage",
	Type:      data.FieldSelect,
	Options: []data.FieldOptionRequest{
		{Value: "lead"},
		{Value: "won"},
	},
})
```

Select fields use text metadata options, not PostgreSQL enum types.

Fields can opt into PostgreSQL-backed full-text search:

```go
_, err = store.CreateField(ctx, actor, "company", data.CreateFieldRequest{
	TenantKey:    "acme",
	Name:         "name",
	Type:         data.FieldText,
	Searchable:   true,
	SearchWeight: "A",
})
```

Supported weights are `A`, `B`, `C`, and `D`. Searchable fields currently support text-like values: text, rich text, select, multi-select, full name, address, emails, and phones.

## Records And Queries

```go
_, err = store.CreateRecord(ctx, actor, "company", data.CreateRecordRequest{
	TenantKey: "acme",
	Values: data.Record{
		"name":  "Acme",
		"stage": "won",
	},
})

page, err := store.QueryRecords(ctx, actor, "company", data.QueryRecordsRequest{
	TenantKey: "acme",
	Query: data.Query{
		Select: []string{"name", "stage"},
		Filter: data.And(data.EQ("stage", "won"), data.Search("acme")),
		Sort:   []data.Sort{data.Asc("name")},
		Limit:  50,
	},
})
```

`RecordPage.NextCursor` is an opaque keyset cursor. Reuse the same object and sort shape when passing it back as `Query.Cursor`.

`data.Search("term")` uses an indexed `onlava_data.search_documents` table maintained by normal data mutations in the same transaction as record writes. Direct SQL edits can update records without refreshing search documents in this version; use the public data mutation path for searchable data until trigger-backed search rebuilds are added.

## Standard Auth Tenant Permissions

Standard auth exposes an active `tenant_id`. The data package maps that value to `TenantKey`:

```go
store, err := data.Open(ctx, pool, data.Options{
	Permissions: data.StandardAuthPermissions{},
})
tenantKey, err := data.RequireTenantKeyFromContext(ctx)
actor := data.ActorFromContext(ctx)
```

`StandardAuthPermissions` fails closed when the actor has no standard-auth tenant or when a request tries to access another data tenant. Apps can wrap their own permission provider with `Base` to add object, field, or row-level rules after the tenant check passes.

## Relations

```go
_, err = store.CreateField(ctx, actor, "deal", data.CreateFieldRequest{
	TenantKey:      "acme",
	Name:           "company",
	Type:           data.FieldRelation,
	RelationObject: "company",
	Relation: data.RelationSettings{
		Kind:     data.RelationManyToOne,
		OnDelete: data.RelationDeleteRestrict,
	},
})
```

`many_to_one` creates a UUID column and PostgreSQL foreign key. Queries can use one-hop paths such as `company.name`. `many_to_many` creates a join table; ergonomic record mutation helpers for many-to-many fields are not stable yet.

## Indexes And Saved Views

```go
_, err = store.CreateIndex(ctx, actor, "company", data.CreateIndexRequest{
	TenantKey: "acme",
	Name:      "company_stage_name",
	Fields: []data.IndexField{
		{Field: "stage"},
		{Field: "name"},
	},
})

_, err = store.CreateView(ctx, actor, "company", data.CreateViewRequest{
	TenantKey:  "acme",
	Name:       "won_companies",
	Columns:    []string{"name", "stage"},
	Filter:     data.EQ("stage", "won"),
	Sort:       []data.Sort{data.Asc("name")},
	Visibility: data.ViewVisibilityShared,
})
```

Saved views are reusable query shapes. Use `QueryView` to execute one.

## Import And Export

`ExportTenant` returns a portable `onlava.data.export.v1` bundle with logical metadata and records:

```go
bundle, err := store.ExportTenant(ctx, actor, data.ExportTenantRequest{
	TenantKey: "acme",
})
```

`ImportTenant` recreates objects, fields, indexes, saved views, and records through the normal mutation paths:

```go
resp, err := store.ImportTenant(ctx, actor, data.ImportTenantRequest{
	Bundle:          *bundle,
	TargetTenantKey: "acme_copy",
})
```

Imported records receive new IDs. The response includes `RecordIDMap`, keyed by exported record ID, so apps can reconcile fixture references. Import writes normal outbox rows and publishes imported record events after the import transaction commits.

## Errors

Public data methods wrap failures in `*data.Error` where possible. Use `data.CodeOf(err)` to classify errors:

```go
if err != nil {
	switch data.CodeOf(err) {
	case data.ErrorInvalidCursor:
		// Ask the caller to restart pagination.
	case data.ErrorFieldNotFound:
		// The app asked for a field not present in metadata.
	}
}
```

The data package is still beta. `docs/local-contract.md` is the source of truth for which parts are stable, beta, or dev-only.
