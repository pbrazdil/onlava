package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/pbrazdil/onlava/internal/build"
)

func useFakeBuildGoRunner(t *testing.T) {
	t.Helper()
	restore := build.SetGoRunnerForTesting(func(_ context.Context, _ string, args ...string) error {
		if len(args) >= 2 && args[0] == "mod" && args[1] == "tidy" {
			return nil
		}
		if len(args) >= 4 && args[0] == "build" && args[1] == "-buildvcs=false" && args[2] == "-o" {
			out := args[3]
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return err
			}
			return os.WriteFile(out, []byte("#!/bin/sh\nexit 0\n"), 0o755)
		}
		return fmt.Errorf("unexpected fake go command: %v", args)
	})
	t.Cleanup(restore)
}
