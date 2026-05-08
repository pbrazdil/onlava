package datastore

import (
	"strings"
	"testing"
)

func TestValidateNameRejectsUnsafeIdentifiers(t *testing.T) {
	for _, name := range []string{
		"",
		"Company",
		"company-name",
		"company;drop_table",
		"1company",
		"select",
		strings.Repeat("a", 64),
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateName("field", name); err == nil {
				t.Fatalf("validateName(%q) succeeded, want error", name)
			}
		})
	}
	if err := validateName("field", "company_name_1"); err != nil {
		t.Fatalf("validateName(valid) error = %v", err)
	}
}

func TestFieldColumnsMapping(t *testing.T) {
	tests := []struct {
		name      string
		fieldType FieldType
		want      []PhysicalColumn
	}{
		{
			name:      "text",
			fieldType: FieldText,
			want:      []PhysicalColumn{{Name: "text", SQLType: "text", Nullable: true}},
		},
		{
			name:      "amount",
			fieldType: FieldCurrency,
			want: []PhysicalColumn{
				{Name: "amount_amount", Part: "amount", SQLType: "numeric", Nullable: true},
				{Name: "amount_currency_code", Part: "currency_code", SQLType: "text", Nullable: true},
			},
		},
		{
			name:      "name",
			fieldType: FieldFullName,
			want: []PhysicalColumn{
				{Name: "name_first_name", Part: "first_name", SQLType: "text", Nullable: true},
				{Name: "name_last_name", Part: "last_name", SQLType: "text", Nullable: true},
			},
		},
		{
			name:      "stage",
			fieldType: FieldSelect,
			want:      []PhysicalColumn{{Name: "stage", SQLType: "text", Nullable: true}},
		},
		{
			name:      "tags",
			fieldType: FieldMultiSelect,
			want:      []PhysicalColumn{{Name: "tags", SQLType: "text[]", Nullable: true}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fieldColumns(tt.name, tt.fieldType, true)
			if err != nil {
				t.Fatalf("fieldColumns() error = %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("fieldColumns() len = %d, want %d: %#v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("fieldColumns()[%d] = %#v, want %#v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestDDLGenerationUsesRealColumns(t *testing.T) {
	tableName := "t_123_company"
	create := createObjectTableDDL(tableName)
	for _, want := range []string{
		`create table "onlava_data_records"."t_123_company"`,
		`id uuid primary key`,
		`tenant_id uuid not null`,
		`deleted_at timestamptz null`,
	} {
		if !strings.Contains(create, want) {
			t.Fatalf("createObjectTableDDL missing %q:\n%s", want, create)
		}
	}
	field := &Field{
		Name:       "stage",
		Type:       FieldSelect,
		IsNullable: true,
		Columns:    []PhysicalColumn{{Name: "stage", SQLType: "text", Nullable: true}},
	}
	ddl := addFieldDDL(tableName, field)
	if len(ddl) != 1 || ddl[0] != `alter table "onlava_data_records"."t_123_company" add column "stage" text` {
		t.Fatalf("addFieldDDL = %#v", ddl)
	}
}

func TestCompileQueryParameterizesValuesAndQuotesMetadataIdentifiers(t *testing.T) {
	state := testState()
	filter := &Filter{
		Op: "and",
		Filters: []Filter{
			{Op: "contains", Field: "name", Value: "Acme' OR true --"},
			{Op: "in", Field: "stage", Values: []any{"lead", "won"}},
		},
	}
	compiled, err := compileQuery(state, Query{
		Select: []string{"name", "stage"},
		Filter: filter,
		Sort:   []Sort{{Field: "name"}},
		Limit:  25,
	})
	if err != nil {
		t.Fatalf("compileQuery() error = %v", err)
	}
	if strings.Contains(compiled.SQL, "Acme") || strings.Contains(compiled.SQL, "lead") {
		t.Fatalf("compileQuery interpolated user values into SQL:\n%s", compiled.SQL)
	}
	for _, want := range []string{`"name"`, `"stage"`, `$2`, `$3`, `$4`, `limit $5`} {
		if !strings.Contains(compiled.SQL, want) {
			t.Fatalf("compileQuery SQL missing %q:\n%s", want, compiled.SQL)
		}
	}
	if len(compiled.Args) != 5 {
		t.Fatalf("compileQuery args len = %d, want 5: %#v", len(compiled.Args), compiled.Args)
	}
}

func TestCompileQueryRejectsInvalidOperatorForType(t *testing.T) {
	state := testState()
	_, err := compileQuery(state, Query{
		Filter: &Filter{Op: "contains", Field: "arr", Value: "1"},
	})
	if err == nil || !strings.Contains(err.Error(), "operator contains is not valid") {
		t.Fatalf("compileQuery error = %v, want invalid operator", err)
	}
}

func TestEventMatchingAgainstQuerySubscription(t *testing.T) {
	event := &Event{
		TenantID: "tenant-1",
		Object:   "company",
		Action:   "updated",
		Before:   Record{"stage": "lead", "name": "Old"},
		After:    Record{"stage": "won", "name": "New"},
	}
	sub := &liveSubscription{
		tenantID: "tenant-1",
		request: SubscriptionRequest{
			QueryID:        "won-companies",
			Object:         "company",
			Filter:         &Filter{Op: "eq", Field: "stage", Value: "won"},
			SelectedFields: []string{"stage"},
		},
	}
	deliver := eventForSubscription(event, sub)
	if deliver == nil {
		t.Fatal("eventForSubscription returned nil")
	}
	if got := deliver.QueryIDs; len(got) != 1 || got[0] != "won-companies" {
		t.Fatalf("query ids = %#v", got)
	}
	if _, ok := deliver.After["name"]; ok {
		t.Fatalf("selected field stripping kept name: %#v", deliver.After)
	}
	other := *sub
	other.request.Filter = &Filter{Op: "eq", Field: "stage", Value: "lost"}
	if got := eventForSubscription(event, &other); got != nil {
		t.Fatalf("eventForSubscription unrelated = %#v, want nil", got)
	}
}

func testState() *metadataState {
	return &metadataState{
		Tenant: &Tenant{ID: "00000000-0000-0000-0000-000000000001", Key: "test"},
		Object: &Object{ID: "00000000-0000-0000-0000-000000000002", TenantID: "00000000-0000-0000-0000-000000000001", NameSingular: "company", TableName: "t_test_company", SchemaVersion: 3},
		Fields: map[string]*Field{
			"name": {
				ID:         "field-name",
				Name:       "name",
				Type:       FieldText,
				IsNullable: true,
				Columns:    []PhysicalColumn{{Name: "name", SQLType: "text", Nullable: true}},
			},
			"stage": {
				ID:         "field-stage",
				Name:       "stage",
				Type:       FieldSelect,
				IsNullable: true,
				Columns:    []PhysicalColumn{{Name: "stage", SQLType: "text", Nullable: true}},
			},
			"arr": {
				ID:         "field-arr",
				Name:       "arr",
				Type:       FieldNumeric,
				IsNullable: true,
				Columns:    []PhysicalColumn{{Name: "arr", SQLType: "numeric", Nullable: true}},
			},
		},
	}
}
