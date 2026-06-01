package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRunOnlavaInspectDocs(t *testing.T) {
	t.Parallel()

	root := writeHarnessSelfRepo(t, `{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object"}`)

	var out bytes.Buffer
	if err := runOnlavaInspect([]string{"docs", "--repo-root", root, "--json"}, &out); err != nil {
		t.Fatalf("inspect docs: %v\n%s", err, out.String())
	}

	var payload inspectDocsResponse
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v\n%s", err, out.String())
	}
	if payload.SchemaVersion != inspectDocsSchema {
		t.Fatalf("schema = %q", payload.SchemaVersion)
	}
	if payload.Repo.Root != root || payload.Repo.ModulePath != "github.com/pbrazdil/onlava" {
		t.Fatalf("repo = %+v", payload.Repo)
	}
	if payload.Summary.DocumentCount == 0 || payload.Summary.MissingCount != 0 {
		t.Fatalf("summary = %+v", payload.Summary)
	}
	if len(payload.Documents) == 0 || !payload.Documents[0].Exists {
		t.Fatalf("documents = %+v", payload.Documents)
	}
	if !payload.Plans.Active.Exists || !payload.Plans.Completed.Exists || !payload.TechDebt.Exists {
		t.Fatalf("plans/debt = %+v %+v", payload.Plans, payload.TechDebt)
	}
}
