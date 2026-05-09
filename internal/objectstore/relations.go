package objectstore

import (
	"context"
	"fmt"
	"strings"
)

type relationConfig struct {
	Kind          RelationKind
	Target        *Object
	InverseField  string
	OnDelete      RelationDeleteBehavior
	JoinTableName string
}

func (s *Store) fieldSettings(ctx context.Context, state *metadataState, fieldID string, fieldType FieldType, nullable bool, req CreateFieldRequest) (map[string]any, string, *relationConfig, error) {
	settings := copySettings(req.Settings)
	if fieldType != FieldRelation {
		if strings.TrimSpace(req.RelationObject) != "" {
			return nil, "", nil, fmt.Errorf("relation_object is only valid for relation fields")
		}
		return settings, "", nil, nil
	}
	targetName := firstNonEmpty(req.RelationObject, stringSetting(settings, "relation_object"))
	if targetName == "" {
		return nil, "", nil, fmt.Errorf("relation field %s requires relation_object", req.Name)
	}
	if err := validateName("relation object", targetName); err != nil {
		return nil, "", nil, err
	}
	target, err := s.loadObject(ctx, state.Tenant.ID, targetName)
	if err != nil {
		return nil, "", nil, fmt.Errorf("load relation object %q for field %s: %w", targetName, req.Name, err)
	}
	kind := RelationKind(firstNonEmpty(string(req.Relation.Kind), stringSetting(settings, "relation_kind"), stringSetting(settings, "kind"), string(RelationManyToOne)))
	switch kind {
	case RelationManyToOne, RelationManyToMany:
	default:
		return nil, "", nil, fmt.Errorf("relation field %s has unsupported kind %q", req.Name, kind)
	}
	onDelete := RelationDeleteBehavior(firstNonEmpty(string(req.Relation.OnDelete), stringSetting(settings, "on_delete"), string(RelationDeleteRestrict)))
	switch onDelete {
	case RelationDeleteRestrict, RelationDeleteSetNull, RelationDeleteCascade:
	default:
		return nil, "", nil, fmt.Errorf("relation field %s has unsupported on_delete %q", req.Name, onDelete)
	}
	if kind == RelationManyToOne && onDelete == RelationDeleteSetNull && !nullable {
		return nil, "", nil, fmt.Errorf("relation field %s cannot use on_delete=set_null when nullable=false", req.Name)
	}
	if kind == RelationManyToMany && onDelete == RelationDeleteSetNull {
		return nil, "", nil, fmt.Errorf("many_to_many relation field %s cannot use on_delete=set_null", req.Name)
	}
	inverseField := firstNonEmpty(req.Relation.InverseField, stringSetting(settings, "inverse_field"))
	if inverseField != "" {
		if err := validateName("inverse relation field", inverseField); err != nil {
			return nil, "", nil, err
		}
	}
	joinTable := ""
	if kind == RelationManyToMany {
		if fieldID == "" {
			return nil, "", nil, fmt.Errorf("many_to_many relation field %s requires a field id", req.Name)
		}
		joinTable = physicalJoinTableName(fieldID, state.Object.NameSingular, req.Name)
	}
	settings["relation_kind"] = string(kind)
	settings["relation_object"] = target.NameSingular
	settings["relation_object_id"] = target.ID
	settings["on_delete"] = string(onDelete)
	if inverseField != "" {
		settings["inverse_field"] = inverseField
	} else {
		delete(settings, "inverse_field")
	}
	if joinTable != "" {
		settings["join_table_name"] = joinTable
	} else {
		delete(settings, "join_table_name")
	}
	return settings, target.ID, &relationConfig{
		Kind:          kind,
		Target:        target,
		InverseField:  inverseField,
		OnDelete:      onDelete,
		JoinTableName: joinTable,
	}, nil
}

func relationFieldDDL(sourceTable string, field *Field, relation *relationConfig) ([]string, error) {
	if relation == nil {
		return nil, nil
	}
	switch relation.Kind {
	case RelationManyToOne:
		if len(field.Columns) != 1 {
			return nil, fmt.Errorf("many_to_one relation field %s requires one physical column", field.Name)
		}
		constraint := physicalConstraintName(field.ID, "fk", field.Name)
		return []string{
			`alter table ` + qualifiedIdent(RecordsSchema, sourceTable) +
				` add constraint ` + quoteIdent(constraint) +
				` foreign key (` + quoteIdent(field.Columns[0].Name) + `)` +
				` references ` + qualifiedIdent(RecordsSchema, relation.Target.TableName) + ` (id)` +
				` on delete ` + relationDeleteSQL(relation.OnDelete),
		}, nil
	case RelationManyToMany:
		if relation.JoinTableName == "" {
			return nil, fmt.Errorf("many_to_many relation field %s is missing join table name", field.Name)
		}
		return []string{
			`create table ` + qualifiedIdent(RecordsSchema, relation.JoinTableName) + ` (
				tenant_id uuid not null,
				source_record_id uuid not null references ` + qualifiedIdent(RecordsSchema, sourceTable) + ` (id) on delete cascade,
				target_record_id uuid not null references ` + qualifiedIdent(RecordsSchema, relation.Target.TableName) + ` (id) on delete ` + relationDeleteSQL(relation.OnDelete) + `,
				created_at timestamptz not null,
				primary key (tenant_id, source_record_id, target_record_id)
			)`,
		}, nil
	default:
		return nil, fmt.Errorf("relation kind %q is not supported", relation.Kind)
	}
}

func (s *Store) verifyRelationField(ctx context.Context, q Queryer, sourceTable string, field *Field) error {
	if field.Type != FieldRelation {
		return nil
	}
	kind := relationKindForField(field)
	switch kind {
	case RelationManyToOne:
		constraint := physicalConstraintName(field.ID, "fk", field.Name)
		var exists bool
		err := q.QueryRow(ctx, `
			select exists (
				select 1
				from pg_constraint c
				join pg_class tbl on tbl.oid = c.conrelid
				join pg_namespace n on n.oid = tbl.relnamespace
				where n.nspname = $1
				  and tbl.relname = $2
				  and c.conname = $3
				  and c.contype = 'f'
			)
		`, RecordsSchema, sourceTable, constraint).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("relation field %s foreign key %s was not created", field.Name, constraint)
		}
	case RelationManyToMany:
		joinTable := stringSetting(field.Settings, "join_table_name")
		if joinTable == "" {
			return fmt.Errorf("many_to_many relation field %s is missing join table metadata", field.Name)
		}
		var exists bool
		err := q.QueryRow(ctx, `
			select exists (
				select 1 from information_schema.tables
				where table_schema = $1 and table_name = $2
			)
		`, RecordsSchema, joinTable).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("many_to_many relation field %s join table %s was not created", field.Name, joinTable)
		}
	}
	return nil
}

func relationKindForField(field *Field) RelationKind {
	if field == nil || field.Type != FieldRelation {
		return ""
	}
	kind := RelationKind(firstNonEmpty(stringSetting(field.Settings, "relation_kind"), stringSetting(field.Settings, "kind"), string(RelationManyToOne)))
	switch kind {
	case RelationManyToOne, RelationManyToMany:
		return kind
	default:
		return RelationManyToOne
	}
}

func relationDeleteSQL(value RelationDeleteBehavior) string {
	switch value {
	case RelationDeleteSetNull:
		return "set null"
	case RelationDeleteCascade:
		return "cascade"
	default:
		return "restrict"
	}
}

func copySettings(in map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range in {
		out[key] = value
	}
	return out
}

func stringSetting(settings map[string]any, key string) string {
	if settings == nil {
		return ""
	}
	switch value := settings[key].(type) {
	case string:
		return strings.TrimSpace(value)
	default:
		return ""
	}
}
