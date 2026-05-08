package data

import "testing"

func TestFilterHelpers(t *testing.T) {
	filter := And(
		Contains("name", "acme"),
		Or(EQ("stage", "lead"), EQ("stage", "won")),
		Not(IsNull("arr")),
	)
	if filter == nil || filter.Op != "and" || len(filter.Filters) != 3 {
		t.Fatalf("And filter = %#v", filter)
	}
	if filter.Filters[0].Op != "contains" || filter.Filters[0].Field != "name" || filter.Filters[0].Value != "acme" {
		t.Fatalf("contains filter = %#v", filter.Filters[0])
	}
	if filter.Filters[1].Op != "or" || len(filter.Filters[1].Filters) != 2 {
		t.Fatalf("or filter = %#v", filter.Filters[1])
	}
	if filter.Filters[2].Op != "not" || len(filter.Filters[2].Filters) != 1 || filter.Filters[2].Filters[0].Value != true {
		t.Fatalf("not filter = %#v", filter.Filters[2])
	}
}

func TestFilterHelpersCollapseEmptyAndSingleLogicalFilters(t *testing.T) {
	if got := And(nil, nil); got != nil {
		t.Fatalf("And(nil) = %#v, want nil", got)
	}
	one := EQ("stage", "won")
	if got := And(nil, one); got == nil || got.Op != "eq" || got.Field != "stage" {
		t.Fatalf("And(single) = %#v, want single filter", got)
	}
}

func TestSortHelpers(t *testing.T) {
	if got := Asc("name"); got.Field != "name" || got.Desc {
		t.Fatalf("Asc = %#v", got)
	}
	if got := Desc("arr"); got.Field != "arr" || !got.Desc {
		t.Fatalf("Desc = %#v", got)
	}
}
