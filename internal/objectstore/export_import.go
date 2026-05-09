package objectstore

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

const dataExportSchemaVersion = "onlava.data.export.v1"

func (s *Store) ExportTenant(ctx context.Context, actor Actor, req ExportTenantRequest) (*ExportBundle, error) {
	tenantKey := strings.TrimSpace(req.TenantKey)
	if tenantKey == "" {
		return nil, fmt.Errorf("export tenant_key is required")
	}
	tenant, err := s.loadTenant(ctx, tenantKey)
	if err != nil {
		return nil, err
	}
	names, err := s.listObjectNames(ctx, tenant.ID)
	if err != nil {
		return nil, err
	}
	bundle := &ExportBundle{
		SchemaVersion: dataExportSchemaVersion,
		Tenant: ExportTenant{
			Key:  tenant.Key,
			Name: tenant.Name,
		},
	}
	for _, objectName := range names {
		state, err := s.loadState(ctx, tenantKey, objectName)
		if err != nil {
			return nil, err
		}
		if err := s.perms.CanReadObject(ctx, actor, objectRef(state)); err != nil {
			return nil, err
		}
		exported, err := s.exportObject(ctx, actor, state)
		if err != nil {
			return nil, err
		}
		bundle.Objects = append(bundle.Objects, exported)
	}
	return bundle, nil
}

func (s *Store) ImportTenant(ctx context.Context, actor Actor, req ImportTenantRequest) (*ImportTenantResponse, error) {
	if req.Bundle.SchemaVersion != dataExportSchemaVersion {
		return nil, fmt.Errorf("unsupported data export schema_version %q", req.Bundle.SchemaVersion)
	}
	targetTenantKey := strings.TrimSpace(req.TargetTenantKey)
	if targetTenantKey == "" {
		targetTenantKey = strings.TrimSpace(req.Bundle.Tenant.Key)
	}
	if targetTenantKey == "" {
		return nil, fmt.Errorf("import target tenant key is required")
	}
	targetTenantName := firstNonEmpty(req.TargetTenantName, req.Bundle.Tenant.Name, targetTenantKey)

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txStore := &Store{db: tx, perms: s.perms, now: s.now, router: newLiveRouter()}
	resp := &ImportTenantResponse{
		TenantKey:   targetTenantKey,
		RecordIDMap: map[string]string{},
	}
	for _, object := range req.Bundle.Objects {
		if _, err := txStore.CreateObject(ctx, actor, CreateObjectRequest{
			TenantKey:     targetTenantKey,
			TenantName:    targetTenantName,
			NameSingular:  object.NameSingular,
			NamePlural:    object.NamePlural,
			LabelSingular: object.LabelSingular,
			LabelPlural:   object.LabelPlural,
		}); err != nil {
			return nil, err
		}
		resp.Objects++
	}
	for _, object := range req.Bundle.Objects {
		for _, field := range object.Fields {
			nullable := field.Nullable
			if _, err := txStore.CreateField(ctx, actor, object.NameSingular, CreateFieldRequest{
				TenantKey:      targetTenantKey,
				Name:           field.Name,
				Label:          field.Label,
				Type:           field.Type,
				Nullable:       &nullable,
				Unique:         field.Unique,
				Array:          field.Array,
				Searchable:     field.Searchable,
				SearchWeight:   field.SearchWeight,
				RelationObject: field.RelationObject,
				Relation:       field.Relation,
				Settings:       copySettings(field.Settings),
				Options:        append([]FieldOptionRequest(nil), field.Options...),
			}); err != nil {
				return nil, err
			}
			resp.Fields++
		}
	}
	for _, object := range req.Bundle.Objects {
		for _, index := range object.Indexes {
			if _, err := txStore.CreateIndex(ctx, actor, object.NameSingular, CreateIndexRequest{
				TenantKey: targetTenantKey,
				Name:      index.Name,
				Method:    index.Method,
				Unique:    index.Unique,
				Fields:    append([]IndexField(nil), index.Fields...),
			}); err != nil {
				return nil, err
			}
			resp.Indexes++
		}
		for _, view := range object.Views {
			if _, err := txStore.CreateView(ctx, actor, object.NameSingular, CreateViewRequest{
				TenantKey:  targetTenantKey,
				Name:       view.Name,
				Type:       view.Type,
				Columns:    append([]string(nil), view.Columns...),
				Filter:     cloneFilter(view.Filter),
				Sort:       append([]Sort(nil), view.Sort...),
				Limit:      view.Limit,
				Visibility: view.Visibility,
				OwnerID:    view.OwnerID,
				Layout:     copySettings(view.Layout),
			}); err != nil {
				return nil, err
			}
			resp.Views++
		}
	}

	var events []*Event
	for _, object := range req.Bundle.Objects {
		for _, record := range object.Records {
			oldID := stringValue(record["id"])
			values := importRecordValues(record, resp.RecordIDMap)
			created, err := txStore.CreateRecord(ctx, actor, object.NameSingular, CreateRecordRequest{
				TenantKey: targetTenantKey,
				Values:    values,
			})
			if err != nil {
				return nil, err
			}
			newID := stringValue(created.Record["id"])
			if oldID != "" && newID != "" {
				resp.RecordIDMap[oldID] = newID
			}
			if created.Event != nil {
				events = append(events, created.Event)
			}
			resp.Records++
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	for _, event := range events {
		s.router.publish(event)
	}
	if len(resp.RecordIDMap) == 0 {
		resp.RecordIDMap = nil
	}
	return resp, nil
}

func (s *Store) listObjectNames(ctx context.Context, tenantID string) ([]string, error) {
	rows, err := s.db.Query(ctx, `
		select name_singular
		from `+qualifiedIdent(MetadataSchema, "objects")+`
		where tenant_id = $1
		order by name_singular
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func (s *Store) exportObject(ctx context.Context, actor Actor, state *metadataState) (ExportObject, error) {
	out := ExportObject{
		NameSingular:  state.Object.NameSingular,
		NamePlural:    state.Object.NamePlural,
		LabelSingular: state.Object.LabelSingular,
		LabelPlural:   state.Object.LabelPlural,
	}
	fieldNames := make([]string, 0, len(state.Fields))
	for name := range state.Fields {
		fieldNames = append(fieldNames, name)
	}
	sort.Strings(fieldNames)
	for _, name := range fieldNames {
		field := state.Fields[name]
		if err := s.perms.CanReadField(ctx, actor, fieldRef(state, field)); err != nil {
			return ExportObject{}, err
		}
		out.Fields = append(out.Fields, exportField(state, field))
	}
	indexes, err := s.ListIndexes(ctx, actor, state.Object.NameSingular, ListIndexesRequest{TenantKey: state.Tenant.Key})
	if err != nil {
		return ExportObject{}, err
	}
	for _, index := range indexes {
		out.Indexes = append(out.Indexes, exportIndex(index))
	}
	views, err := s.ListViews(ctx, actor, state.Object.NameSingular, ListViewsRequest{TenantKey: state.Tenant.Key})
	if err != nil {
		return ExportObject{}, err
	}
	for _, view := range views {
		out.Views = append(out.Views, exportView(view))
	}
	page, err := s.QueryRecords(ctx, actor, state.Object.NameSingular, QueryRecordsRequest{
		TenantKey: state.Tenant.Key,
		Query:     Query{Sort: []Sort{{Field: "id"}}, Limit: maxQueryLimit},
	})
	if err != nil {
		return ExportObject{}, err
	}
	for {
		for _, record := range page.Records {
			out.Records = append(out.Records, cloneRecord(record))
		}
		if page.NextCursor == "" {
			break
		}
		page, err = s.QueryRecords(ctx, actor, state.Object.NameSingular, QueryRecordsRequest{
			TenantKey: state.Tenant.Key,
			Query:     Query{Sort: []Sort{{Field: "id"}}, Limit: maxQueryLimit, Cursor: page.NextCursor},
		})
		if err != nil {
			return ExportObject{}, err
		}
	}
	return out, nil
}

func exportField(state *metadataState, field *Field) ExportField {
	out := ExportField{
		Name:         field.Name,
		Label:        field.Label,
		Type:         field.Type,
		Nullable:     field.IsNullable,
		Unique:       field.IsUnique,
		Array:        field.IsArray,
		Searchable:   field.IsSearchable,
		SearchWeight: field.SearchWeight,
		Settings:     exportFieldSettings(field),
	}
	for _, option := range field.Options {
		if option.IsArchived {
			continue
		}
		out.Options = append(out.Options, FieldOptionRequest{
			Value: option.Value,
			Label: option.Label,
			Color: option.Color,
		})
	}
	if field.Type == FieldRelation {
		if target := state.Relations[field.Name]; target != nil && target.Object != nil {
			out.RelationObject = target.Object.NameSingular
		} else {
			out.RelationObject = stringSetting(field.Settings, "relation_object")
		}
		out.Relation = RelationSettings{
			Kind:         relationKindForField(field),
			InverseField: stringSetting(field.Settings, "inverse_field"),
			OnDelete:     RelationDeleteBehavior(firstNonEmpty(stringSetting(field.Settings, "on_delete"), string(RelationDeleteRestrict))),
		}
	}
	return out
}

func exportFieldSettings(field *Field) map[string]any {
	settings := copySettings(field.Settings)
	if field.Type == FieldRelation {
		for _, key := range []string{"relation_kind", "kind", "relation_object", "relation_object_id", "on_delete", "inverse_field", "join_table_name"} {
			delete(settings, key)
		}
	}
	if len(settings) == 0 {
		return nil
	}
	return settings
}

func exportIndex(index Index) ExportIndex {
	fields := make([]IndexField, 0, len(index.Fields))
	for _, field := range index.Fields {
		fields = append(fields, IndexField{Field: field.Field, Desc: field.Desc})
	}
	return ExportIndex{
		Name:   index.Name,
		Method: index.Method,
		Unique: index.IsUnique,
		Fields: fields,
	}
}

func exportView(view View) ExportView {
	return ExportView{
		Name:       view.Name,
		Type:       view.Type,
		Columns:    append([]string(nil), view.Columns...),
		Filter:     cloneFilter(view.Filter),
		Sort:       append([]Sort(nil), view.Sort...),
		Limit:      view.Limit,
		Visibility: view.Visibility,
		OwnerID:    view.OwnerID,
		Layout:     copySettings(view.Layout),
	}
}

func cloneFilter(filter *Filter) *Filter {
	if filter == nil {
		return nil
	}
	clone := *filter
	clone.Values = append([]any(nil), filter.Values...)
	clone.Filters = append([]Filter(nil), filter.Filters...)
	for i := range clone.Filters {
		child := cloneFilter(&clone.Filters[i])
		if child != nil {
			clone.Filters[i] = *child
		}
	}
	return &clone
}

func importRecordValues(record Record, idMap map[string]string) Record {
	out := Record{}
	for key, value := range record {
		switch key {
		case "id", "created_at", "updated_at":
			continue
		}
		if mapped, ok := idMap[stringValue(value)]; ok {
			out[key] = mapped
			continue
		}
		out[key] = value
	}
	return out
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}
