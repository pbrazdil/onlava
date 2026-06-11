package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"scenery.sh/internal/build"
)

func useFakeBuildGoRunner(t *testing.T) {
	t.Helper()
	restore := build.SetGoRunnerForTesting(func(_ context.Context, _ string, args ...string) error {
		if len(args) >= 2 && args[0] == "mod" && args[1] == "tidy" {
			return nil
		}
		if out, ok := fakeBuildOutputArg(args); ok {
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return err
			}
			return os.WriteFile(out, []byte("#!/bin/sh\nexit 0\n"), 0o755)
		}
		return fmt.Errorf("unexpected fake go command: %v", args)
	})
	t.Cleanup(restore)
}

func fakeBuildOutputArg(args []string) (string, bool) {
	if len(args) < 5 || args[0] != "build" || args[len(args)-1] != "./scenery_internal_main" {
		return "", false
	}
	for i := 1; i < len(args)-2; i++ {
		if args[i] == "-buildvcs=false" && args[i+1] == "-o" {
			return args[i+2], true
		}
	}
	return "", false
}
