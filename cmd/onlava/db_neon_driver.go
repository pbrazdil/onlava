package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pbrazdil/onlava/internal/envpolicy"
)

const (
	neonSelfhostBranchDriverEnv             = "ONLAVA_DEV_NEON_SELFHOST_DRIVER"
	localPostgresBranchDriverEndpointSource = "local-postgres-branch-driver"
	neonSelfhostBranchDriverEndpointSource  = "neon-selfhost-driver"
)

type executableNeonBranchDriverResult struct {
	Status       string                  `json:"status,omitempty"`
	Message      string                  `json:"message,omitempty"`
	Diff         string                  `json:"diff,omitempty"`
	Endpoint     *neonEndpoint           `json:"endpoint,omitempty"`
	RestorePoint *neonBranchRestorePoint `json:"restore_point,omitempty"`
}

type executableNeonBranchDriver struct {
	path                  string
	name                  string
	envName               string
	defaultEndpointSource string
}

func configuredNeonBranchDriver() (executableNeonBranchDriver, bool, error) {
	if driver, ok, err := configuredNeonSelfhostBranchDriver(); ok || err != nil {
		return driver, ok, err
	}
	return configuredLocalPostgresBranchDriver()
}

func configuredNeonSelfhostBranchDriver() (executableNeonBranchDriver, bool, error) {
	return configuredExecutableNeonBranchDriver(neonSelfhostBranchDriverEnv, "neon-selfhost driver", neonSelfhostBranchDriverEndpointSource)
}

func configuredLocalPostgresBranchDriver() (executableNeonBranchDriver, bool, error) {
	return configuredExecutableNeonBranchDriver(localPostgresBranchDriverEnv, "local-postgres-branch driver", localPostgresBranchDriverEndpointSource)
}

func configuredExecutableNeonBranchDriver(envName, name, defaultEndpointSource string) (executableNeonBranchDriver, bool, error) {
	path := strings.TrimSpace(envpolicy.Get(envName))
	if path == "" {
		return executableNeonBranchDriver{}, false, nil
	}
	if !filepath.IsAbs(path) {
		return executableNeonBranchDriver{}, false, fmt.Errorf("%s must be an absolute path to a %s executable", envName, name)
	}
	info, err := os.Stat(path)
	if err != nil {
		return executableNeonBranchDriver{}, false, fmt.Errorf("stat %s: %w", envName, err)
	}
	if info.IsDir() {
		return executableNeonBranchDriver{}, false, fmt.Errorf("%s points at a directory, want executable file", envName)
	}
	if info.Mode()&0o111 == 0 {
		return executableNeonBranchDriver{}, false, fmt.Errorf("%s is not executable", envName)
	}
	return executableNeonBranchDriver{
		path:                  path,
		name:                  name,
		envName:               envName,
		defaultEndpointSource: defaultEndpointSource,
	}, true, nil
}

func (d executableNeonBranchDriver) EnsureBranch(ctx context.Context, pin worktreeDBPin) (neonBranchBackendStatus, error) {
	result, err := d.run(ctx, "ensure", pin, nil)
	if err != nil {
		return neonBranchBackendStatus{Status: "unknown", Message: err.Error()}, err
	}
	status, err := updateNeonBranchLeaseFromDriver(pin, result, d)
	if err != nil {
		return status, err
	}
	if status.Status == "ready" {
		if err := ensureInitialNeonRestorePoint(pin); err != nil {
			return neonBranchBackendStatus{Status: "unknown", Message: err.Error()}, err
		}
	}
	return status, nil
}

func (d executableNeonBranchDriver) ResetBranch(ctx context.Context, pin worktreeDBPin) error {
	result, err := d.run(ctx, "reset", pin, nil)
	if err != nil {
		return err
	}
	if _, err := updateNeonBranchLeaseFromDriver(pin, result, d); err != nil {
		return err
	}
	_, err = recordNeonRestorePoint(pin, "branch-reset", "")
	return err
}

func (d executableNeonBranchDriver) RestoreBranch(ctx context.Context, pin worktreeDBPin, at string) (neonBranchRestorePoint, error) {
	restoreFrom, _ := resolveNeonRestorePoint(pin.BranchID, at)
	result, err := d.run(ctx, "restore", pin, []string{"--at", strings.TrimSpace(at)})
	if err != nil {
		return neonBranchRestorePoint{}, err
	}
	if _, err := updateNeonBranchLeaseFromDriver(pin, result, d); err != nil {
		return neonBranchRestorePoint{}, err
	}
	if result.RestorePoint != nil {
		return *result.RestorePoint, nil
	}
	restoredFrom := restoreFrom.Ref
	point, err := recordNeonRestorePoint(pin, "branch-restore", restoredFrom)
	if err != nil {
		return neonBranchRestorePoint{}, err
	}
	if restoreFrom.Ref != "" {
		return restoreFrom, nil
	}
	return point, nil
}

func (d executableNeonBranchDriver) DiffBranch(ctx context.Context, pin worktreeDBPin, target string) (string, error) {
	result, err := d.run(ctx, "diff", pin, []string{"--target", strings.TrimSpace(target)})
	if err != nil {
		return "", err
	}
	return result.Diff, nil
}

func (d executableNeonBranchDriver) DeleteBranch(ctx context.Context, pin worktreeDBPin) error {
	_, err := d.run(ctx, "delete", pin, nil)
	return err
}

func (d executableNeonBranchDriver) run(ctx context.Context, action string, pin worktreeDBPin, extra []string) (executableNeonBranchDriverResult, error) {
	if strings.TrimSpace(action) == "" {
		return executableNeonBranchDriverResult{}, fmt.Errorf("%s action is required", d.name)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	args := []string{
		action,
		"--project", pin.Project,
		"--parent-branch", pin.ParentBranch,
		"--branch", pin.Branch,
		"--branch-id", pin.BranchID,
		"--database", pin.Database,
		"--role", pin.Role,
	}
	if strings.TrimSpace(pin.TTL) != "" {
		args = append(args, "--ttl", strings.TrimSpace(pin.TTL))
	}
	args = append(args, extra...)
	args = append(args, "--json")
	cmd := exec.CommandContext(ctx, d.path, args...)
	cmd.Env = envpolicy.Environ()
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return executableNeonBranchDriverResult{}, fmt.Errorf("%s %q timed out", d.name, action)
		}
		return executableNeonBranchDriverResult{}, fmt.Errorf("%s %q failed: %w", d.name, action, err)
	}
	var result executableNeonBranchDriverResult
	dec := json.NewDecoder(strings.NewReader(string(out)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&result); err != nil {
		return executableNeonBranchDriverResult{}, fmt.Errorf("parse %s %q JSON: %w", d.name, action, err)
	}
	return result, nil
}

func updateNeonBranchLeaseFromDriver(pin worktreeDBPin, result executableNeonBranchDriverResult, driver executableNeonBranchDriver) (neonBranchBackendStatus, error) {
	status := strings.ToLower(strings.TrimSpace(result.Status))
	if status == "" {
		if result.Endpoint != nil {
			status = "ready"
		} else {
			status = "pending"
		}
	}
	switch status {
	case "ready":
		if result.Endpoint == nil {
			return neonBranchBackendStatus{}, fmt.Errorf("%s marked %q ready without endpoint metadata", driver.name, pin.Branch)
		}
	case "pending", "missing", "expired":
	default:
		return neonBranchBackendStatus{}, fmt.Errorf("%s returned unsupported status %q for %q", driver.name, status, pin.Branch)
	}

	root, err := neonSubstrateRoot()
	if err != nil {
		return neonBranchBackendStatus{}, err
	}
	registry, err := readNeonBranchRegistry(root)
	if err != nil {
		return neonBranchBackendStatus{}, err
	}
	now := time.Now().UTC()
	nowText := now.Format(time.RFC3339)
	var endpoint *neonEndpoint
	if result.Endpoint != nil && status == "ready" {
		normalized := normalizedNeonEndpoint(*result.Endpoint, pin)
		if normalized.Source == "" {
			normalized.Source = driver.defaultEndpointSource
		}
		endpoint = &normalized
	}
	for i := range registry.Leases {
		if !sameNeonLease(registry.Leases[i].Pin, pin) && !sameNeonBranch(registry.Leases[i].Pin, pin) {
			continue
		}
		if !isOnlavaOwnedNeonLease(registry.Leases[i]) {
			return neonBranchBackendStatus{}, fmt.Errorf("refusing to update foreign local Neon branch lease %q from %s", pin.Branch, driver.name)
		}
		if registry.Leases[i].CreatedAt == "" {
			registry.Leases[i].CreatedAt = nowText
		}
		registry.Leases[i].Pin = pin
		registry.Leases[i].Status = status
		registry.Leases[i].Endpoint = endpoint
		registry.Leases[i].UpdatedAt = nowText
		registry.UpdatedAt = nowText
		if err := writeNeonBranchRegistry(root, registry); err != nil {
			return neonBranchBackendStatus{}, err
		}
		return neonBranchBackendStatus{
			Status:   status,
			Message:  firstNonEmpty(strings.TrimSpace(result.Message), driver.name+" updated the local branch lease."),
			Endpoint: endpoint,
		}, nil
	}
	registry.Leases = append(registry.Leases, neonBranchLease{
		Pin:       pin,
		Status:    status,
		Endpoint:  endpoint,
		CreatedAt: nowText,
		UpdatedAt: nowText,
		ExpiresAt: neonLeaseExpiresAt(now, pin.TTL),
	})
	registry.UpdatedAt = nowText
	if err := writeNeonBranchRegistry(root, registry); err != nil {
		return neonBranchBackendStatus{}, err
	}
	return neonBranchBackendStatus{
		Status:   status,
		Message:  firstNonEmpty(strings.TrimSpace(result.Message), driver.name+" created the local branch lease."),
		Endpoint: endpoint,
	}, nil
}
