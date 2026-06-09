package neonselfhost

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"time"
)

const (
	DefaultBranchPortBase  = 55440
	DefaultBranchPortRange = 1000
)

type BackendState struct {
	SchemaVersion    string                   `json:"schema_version"`
	Provider         string                   `json:"provider"`
	TenantID         string                   `json:"tenant_id"`
	DefaultPGVersion int                      `json:"default_pg_version"`
	Branches         map[string]BackendBranch `json:"branches"`
	UpdatedAt        string                   `json:"updated_at,omitempty"`
}

type BackendBranch struct {
	Project          string `json:"project"`
	Branch           string `json:"branch"`
	TimelineID       string `json:"timeline_id"`
	ParentTimelineID string `json:"parent_timeline_id,omitempty"`
	EndpointID       string `json:"endpoint_id"`
	ComputeContainer string `json:"compute_container"`
	Host             string `json:"host"`
	Port             int    `json:"port"`
	Database         string `json:"database"`
	Role             string `json:"role"`
	Status           string `json:"status"`
	UpdatedAt        string `json:"updated_at,omitempty"`
}

func NewBackendState(tenantID string, pgVersion int) BackendState {
	return BackendState{
		SchemaVersion:    BackendSchemaVersion,
		Provider:         "neon-selfhost",
		TenantID:         tenantID,
		DefaultPGVersion: pgVersion,
		Branches:         map[string]BackendBranch{},
	}
}

func ReadBackendState(path string) (BackendState, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return BackendState{}, false, nil
	}
	if err != nil {
		return BackendState{}, false, err
	}
	var state BackendState
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&state); err != nil {
		return BackendState{}, false, fmt.Errorf("parse %s: %w", path, err)
	}
	if state.SchemaVersion != BackendSchemaVersion {
		return BackendState{}, false, fmt.Errorf("%s has unsupported schema_version %q", path, state.SchemaVersion)
	}
	if state.Provider != "neon-selfhost" {
		return BackendState{}, false, fmt.Errorf("%s has unsupported provider %q", path, state.Provider)
	}
	if state.Branches == nil {
		state.Branches = map[string]BackendBranch{}
	}
	return state, true, nil
}

func WriteBackendState(path string, state BackendState) error {
	if state.SchemaVersion == "" {
		state.SchemaVersion = BackendSchemaVersion
	}
	if state.Provider == "" {
		state.Provider = "neon-selfhost"
	}
	if state.Branches == nil {
		state.Branches = map[string]BackendBranch{}
	}
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return atomicWriteFile(path, data, 0o644)
}

func WithBackendStateLock(root string, fn func() error) error {
	unlock, err := lockBackendState(root)
	if err != nil {
		return err
	}
	defer unlock()
	return fn()
}

func AllocateBranchPort(state BackendState, branchID string) int {
	if branch, ok := state.Branches[branchID]; ok && branch.Port > 0 {
		return branch.Port
	}
	used := map[int]bool{}
	for _, branch := range state.Branches {
		if branch.Port > 0 {
			used[branch.Port] = true
		}
	}
	start := DefaultBranchPortBase + int(hashString(branchID)%DefaultBranchPortRange)
	for offset := 0; offset < DefaultBranchPortRange; offset++ {
		port := DefaultBranchPortBase + ((start - DefaultBranchPortBase + offset) % DefaultBranchPortRange)
		if !used[port] {
			return port
		}
	}
	return start
}

func hashString(value string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(value))
	return h.Sum32()
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp")
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
