//go:build windows

package main

import "path/filepath"

func lockDBBranchRegistry(string) (func(), error) {
	return func() {}, nil
}

func dbBranchRegistryLockPath(root string) string {
	return filepath.Join(root, "branches.lock")
}
