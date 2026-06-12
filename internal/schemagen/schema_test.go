package schemagen

import (
	"strings"
	"testing"

	"scenery.sh/internal/model"
)

func TestBuildUsesAppOwnedSchemaForGeneratedModelArtifacts(t *testing.T) {
	t.Parallel()

	app := &model.App{Entities: []*model.Entity{testTaskEntity()}}

	schemas, err := Build("", app)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(schemas) != 1 {
		t.Fatalf("schemas = %+v", schemas)
	}
	hcl := schemas[0].HCL
	for _, want := range []string{
		`schema "tasks" {}`,
		`schema = schema.tasks`,
		`table "tasks"`,
	} {
		if !strings.Contains(hcl, want) {
			t.Fatalf("generated HCL missing %q:\n%s", want, hcl)
		}
	}
	for _, unwanted := range []string{`schema "public"`, `schema.public`} {
		if strings.Contains(hcl, unwanted) {
			t.Fatalf("generated HCL should not contain %q:\n%s", unwanted, hcl)
		}
	}

	seeds, err := BuildSeeds("", app)
	if err != nil {
		t.Fatalf("BuildSeeds() error = %v", err)
	}
	if len(seeds) != 1 {
		t.Fatalf("seeds = %+v", seeds)
	}
	if !strings.Contains(seeds[0].SQL, `insert into "tasks"."tasks"`) {
		t.Fatalf("generated seed SQL should use schema-qualified table:\n%s", seeds[0].SQL)
	}
}

func testTaskEntity() *model.Entity {
	return &model.Entity{
		Package: &model.Package{RelDir: "tasks"},
		Name:    "Task",
		Table:   "tasks",
		Fields: []model.EntityField{
			{Name: "ID", TypeExpr: "string", Kind: model.EntityFieldStored, Column: "id"},
			{Name: "Status", TypeExpr: "string", Kind: model.EntityFieldStored, Column: "status", EnumValues: []string{"todo", "done"}},
		},
		Seeds: []model.EntitySeedRow{{
			Values: []model.EntitySeedValue{
				{Field: "ID", Kind: model.EntitySeedString, Value: "seed-task-1"},
				{Field: "Status", Kind: model.EntitySeedString, Value: "todo"},
			},
		}},
	}
}
