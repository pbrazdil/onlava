package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const onlavaLocalStateGitignore = `# Managed by onlava. Local runtime state may include downloaded binaries,
# databases, logs, generated build outputs, and other machine-local files.
*
!.gitignore
`

func ensureOnlavaLocalStateIgnored(appRoot string) error {
	appRoot = strings.TrimSpace(appRoot)
	if appRoot == "" {
		return nil
	}
	return ensureLocalStateDirIgnored(filepath.Join(appRoot, ".onlava"))
}

func ensureLocalStateDirIgnored(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil
	}
	path := filepath.Join(dir, ".gitignore")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	current, err := os.ReadFile(path)
	if err == nil {
		if localStateGitignoreCovers(current) {
			return nil
		}
		next := strings.TrimRight(string(current), "\n")
		if next != "" {
			next += "\n\n"
		}
		next += onlavaLocalStateGitignore
		return atomicWriteFile(path, []byte(next), 0o644)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return atomicWriteFile(path, []byte(onlavaLocalStateGitignore), 0o644)
}

func localStateGitignoreCovers(data []byte) bool {
	hasAll := false
	hasSelfException := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "*" {
			hasAll = true
		}
		if line == "!.gitignore" {
			hasSelfException = true
		}
	}
	return hasAll && hasSelfException
}
