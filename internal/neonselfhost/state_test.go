package neonselfhost

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackendStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backend.json")
	state := NewBackendState("tenant-test", 16)
	state.Branches["br-local-a"] = BackendBranch{
		Project:          "onlv",
		Branch:           "feature/a",
		TimelineID:       "timeline-a",
		ParentTimelineID: "timeline-main",
		EndpointID:       "feature-a",
		ComputeContainer: "onlava-neon-compute-feature-a",
		Host:             "127.0.0.1",
		Port:             55441,
		Database:         "onlv",
		Role:             "cloud_admin",
		Status:           "ready",
	}
	if err := WriteBackendState(path, state); err != nil {
		t.Fatalf("WriteBackendState returned error: %v", err)
	}
	got, ok, err := ReadBackendState(path)
	if err != nil {
		t.Fatalf("ReadBackendState returned error: %v", err)
	}
	if !ok {
		t.Fatal("ReadBackendState ok=false")
	}
	if got.SchemaVersion != BackendSchemaVersion || got.Provider != "neon-selfhost" || got.TenantID != "tenant-test" {
		t.Fatalf("state = %+v", got)
	}
	if got.Branches["br-local-a"].Port != 55441 {
		t.Fatalf("branch = %+v", got.Branches["br-local-a"])
	}
	if got.UpdatedAt == "" {
		t.Fatalf("updated_at was not set: %+v", got)
	}
}

func TestReadBackendStateRejectsBadState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backend.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":"wrong","provider":"neon-selfhost","tenant_id":"t","default_pg_version":16,"branches":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := ReadBackendState(path)
	if err == nil || !strings.Contains(err.Error(), "unsupported schema_version") {
		t.Fatalf("schema error = %v", err)
	}

	if err := os.WriteFile(path, []byte(`{"schema_version":"onlava.db.neon.selfhost.backend.v1","provider":"other","tenant_id":"t","default_pg_version":16,"branches":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err = ReadBackendState(path)
	if err == nil || !strings.Contains(err.Error(), "unsupported provider") {
		t.Fatalf("provider error = %v", err)
	}
}

func TestAllocateBranchPortStableAndCollisionAware(t *testing.T) {
	state := NewBackendState("tenant-test", 16)
	first := AllocateBranchPort(state, "br-local-a")
	if first < DefaultBranchPortBase || first >= DefaultBranchPortBase+DefaultBranchPortRange {
		t.Fatalf("first port out of range: %d", first)
	}
	state.Branches["br-local-a"] = BackendBranch{Port: first}
	if got := AllocateBranchPort(state, "br-local-a"); got != first {
		t.Fatalf("existing branch port = %d, want %d", got, first)
	}
	collidingBranchID := "br-local-collision"
	collidingStart := DefaultBranchPortBase + int(hashString(collidingBranchID)%DefaultBranchPortRange)
	state.Branches["br-local-b"] = BackendBranch{Port: collidingStart}
	next := AllocateBranchPort(state, collidingBranchID)
	if next == collidingStart {
		t.Fatalf("allocated colliding port %d", next)
	}
	if next < DefaultBranchPortBase || next >= DefaultBranchPortBase+DefaultBranchPortRange {
		t.Fatalf("next port out of range: %d", next)
	}
}
