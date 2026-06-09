//go:build windows

package main

import "path/filepath"

func lockNeonBranchRegistry(string) (func(), error) {
	return func() {}, nil
}

func neonBranchRegistryLockPath(root string) string {
	return filepath.Join(root, "branches.lock")
}
