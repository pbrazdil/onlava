package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	localagent "github.com/pbrazdil/onlava/internal/agent"
	"github.com/pbrazdil/onlava/internal/envpolicy"
	"github.com/pbrazdil/onlava/internal/toolchain"
)

const neonSelfhostDriverToolchainArtifact = "neon-selfhost-driver"

func ensureNeonSelfhostDriverToolchain(ctx context.Context) neonCellDriver {
	storeDir, err := neonSelfhostDriverToolchainStoreDir()
	if err != nil {
		return neonCellDriver{
			Kind:    "toolchain",
			Tool:    neonSelfhostDriverToolchainArtifact,
			Status:  "missing",
			Message: err.Error(),
		}
	}
	status, err := syncManagedToolchainArtifactInDir(ctx, storeDir, neonSelfhostDriverToolchainArtifact)
	return neonCellDriverFromArtifactStatus(status, err)
}

func inspectNeonSelfhostDriverToolchain() neonCellDriver {
	storeDir, err := neonSelfhostDriverToolchainStoreDir()
	if err != nil {
		return neonCellDriver{
			Kind:    "toolchain",
			Tool:    neonSelfhostDriverToolchainArtifact,
			Status:  "missing",
			Message: err.Error(),
		}
	}
	status, err := managedToolchainArtifactStatusInDir(storeDir, neonSelfhostDriverToolchainArtifact)
	return neonCellDriverFromArtifactStatus(status, err)
}

func neonCellDriverFromArtifactStatus(status toolchain.ArtifactStatus, err error) neonCellDriver {
	driver := neonCellDriver{
		Kind:   "toolchain",
		Tool:   neonSelfhostDriverToolchainArtifact,
		Status: "missing",
	}
	if err != nil {
		driver.Message = err.Error()
		return driver
	}
	driver.Path = status.ManagedPath
	driver.Version = status.Version
	driver.Status = status.Status
	driver.Message = status.Message
	if driver.Status == "" {
		driver.Status = "missing"
	}
	return driver
}

func neonSelfhostDriverToolchainStoreDir() (string, error) {
	if strings.TrimSpace(envpolicy.Get("ONLAVA_TOOLCHAIN_DIR")) != "" {
		return toolchain.DefaultStoreDir(""), nil
	}
	paths, err := localagent.DefaultPaths()
	if err != nil {
		return "", err
	}
	return filepath.Join(paths.Home, "toolchain"), nil
}

func configuredManagedNeonSelfhostBranchDriver() (executableNeonBranchDriver, bool, error) {
	root, err := neonSubstrateRoot()
	if err != nil {
		return executableNeonBranchDriver{}, false, err
	}
	state, ok, err := readNeonCellState(root)
	if err != nil {
		return executableNeonBranchDriver{}, false, err
	}
	if ok && state.Driver != nil && strings.TrimSpace(state.Driver.Path) != "" {
		driver, err := executableNeonBranchDriverFromPath(state.Driver.Path, "neon-selfhost driver", "cell.json driver.path", neonSelfhostBranchDriverEndpointSource)
		return driver, err == nil, err
	}
	status := inspectNeonSelfhostDriverToolchain()
	if strings.TrimSpace(status.Path) == "" || status.Status != "installed" {
		return executableNeonBranchDriver{}, false, nil
	}
	driver, err := executableNeonBranchDriverFromPath(status.Path, "neon-selfhost driver", fmt.Sprintf("toolchain artifact %s", neonSelfhostDriverToolchainArtifact), neonSelfhostBranchDriverEndpointSource)
	return driver, err == nil, err
}
