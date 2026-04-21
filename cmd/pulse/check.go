package main

import (
	"context"
	"fmt"
	"os"

	"pulse.dev/internal/app"
	"pulse.dev/internal/build"
	"pulse.dev/internal/parse"
)

func checkCommand(args []string) error {
	return runPulseCheck(context.Background(), args)
}

func runPulseCheck(ctx context.Context, args []string) error {
	appRootFlag, err := parseCheckArgs(args)
	if err != nil {
		return err
	}

	start, err := resolveAppRoot(appRootFlag)
	if err != nil {
		return err
	}
	appRoot, cfg, err := app.DiscoverRoot(start)
	if err != nil {
		return err
	}

	model, err := parse.App(appRoot, cfg.Name)
	if err != nil {
		return err
	}
	result, err := build.Prepare(appRoot, model, cfg, build.PrepareOptions{})
	if err != nil {
		return err
	}
	if err := build.CompileContext(ctx, result); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(os.Stdout, "pulse: check ok")
	return nil
}

func parseCheckArgs(args []string) (string, error) {
	appRoot := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--app-root":
			i++
			if i >= len(args) {
				return "", fmt.Errorf("missing value for --app-root")
			}
			appRoot = args[i]
		default:
			return "", fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return appRoot, nil
}
