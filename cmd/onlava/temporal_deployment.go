package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	temporalclient "go.temporal.io/sdk/client"

	"github.com/pbrazdil/onlava/internal/app"
	onlavaruntime "github.com/pbrazdil/onlava/runtime"
)

type temporalDeploymentOptions struct {
	AppRoot                 string
	Deployment              string
	BuildID                 string
	Percentage              float64
	PercentageSet           bool
	IgnoreMissingTaskQueues bool
	AllowNoPollers          bool
	Force                   bool
	JSON                    bool
}

type temporalDeploymentResult struct {
	OK         bool    `json:"ok"`
	Action     string  `json:"action"`
	Deployment string  `json:"deployment"`
	BuildID    string  `json:"build_id,omitempty"`
	Percentage float64 `json:"percentage,omitempty"`
	Namespace  string  `json:"namespace"`
	Address    string  `json:"address"`
}

func temporalCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: onlava temporal deployment set-current|ramp|drain [flags]")
	}
	switch args[0] {
	case "deployment":
		return temporalDeploymentCommand(args[1:], os.Stdout)
	default:
		return fmt.Errorf("unknown temporal command %q", args[0])
	}
}

func temporalDeploymentCommand(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: onlava temporal deployment set-current|ramp|drain [flags]")
	}
	action := args[0]
	switch action {
	case "set-current", "ramp", "drain":
	default:
		return fmt.Errorf("unknown temporal deployment command %q", action)
	}
	opts, err := parseTemporalDeploymentArgs(action, args[1:])
	if err != nil {
		return err
	}
	return runTemporalDeployment(context.Background(), action, opts, stdout)
}

func parseTemporalDeploymentArgs(action string, args []string) (temporalDeploymentOptions, error) {
	var opts temporalDeploymentOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--app-root":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("missing value for --app-root")
			}
			opts.AppRoot = args[i]
		case "--deployment":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("missing value for --deployment")
			}
			opts.Deployment = strings.TrimSpace(args[i])
			if opts.Deployment == "" {
				return opts, fmt.Errorf("--deployment must not be empty")
			}
		case "--build-id":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("missing value for --build-id")
			}
			opts.BuildID = strings.TrimSpace(args[i])
			if opts.BuildID == "" {
				return opts, fmt.Errorf("--build-id must not be empty")
			}
		case "--percentage":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("missing value for --percentage")
			}
			value, err := strconv.ParseFloat(args[i], 64)
			if err != nil {
				return opts, fmt.Errorf("invalid --percentage %q", args[i])
			}
			opts.Percentage = value
			opts.PercentageSet = true
		case "--ignore-missing-task-queues":
			opts.IgnoreMissingTaskQueues = true
		case "--allow-no-pollers":
			opts.AllowNoPollers = true
		case "--force":
			opts.Force = true
		case "--json":
			opts.JSON = true
		default:
			return opts, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	switch action {
	case "set-current", "ramp", "drain":
		if opts.BuildID == "" {
			return opts, fmt.Errorf("%s requires --build-id", action)
		}
	}
	if action == "ramp" {
		if !opts.PercentageSet {
			return opts, fmt.Errorf("ramp requires --percentage")
		}
		if opts.Percentage < 0 || opts.Percentage > 100 {
			return opts, fmt.Errorf("--percentage must be between 0 and 100")
		}
	} else if opts.PercentageSet {
		return opts, fmt.Errorf("--percentage is only valid with ramp")
	}
	if opts.Force && action != "drain" {
		return opts, fmt.Errorf("--force is only valid with drain")
	}
	return opts, nil
}

func runTemporalDeployment(ctx context.Context, action string, opts temporalDeploymentOptions, stdout io.Writer) error {
	root, err := resolveAppRoot(opts.AppRoot)
	if err != nil {
		return err
	}
	root, cfg, err := app.DiscoverRoot(root)
	if err != nil {
		return err
	}
	_ = root
	rtCfg := temporalRuntimeConfigFromApp(cfg.Temporal)
	if !rtCfg.Enabled {
		return fmt.Errorf("temporal deployment commands require temporal.enabled=true")
	}
	info := onlavaruntime.ResolveTemporalConfig(cfg.Name, rtCfg)
	if opts.Deployment != "" {
		info.DeploymentName = opts.Deployment
	}
	client, err := onlavaruntime.DialTemporal(ctx, info)
	if err != nil {
		return err
	}
	defer client.Close()

	handle := client.WorkerDeploymentClient().GetHandle(onlavaruntime.TemporalDeploymentName(info))
	switch action {
	case "set-current":
		_, err = handle.SetCurrentVersion(ctx, temporalclient.WorkerDeploymentSetCurrentVersionOptions{
			BuildID:                 opts.BuildID,
			Identity:                "onlava-cli",
			IgnoreMissingTaskQueues: opts.IgnoreMissingTaskQueues,
			AllowNoPollers:          opts.AllowNoPollers,
		})
	case "ramp":
		_, err = handle.SetRampingVersion(ctx, temporalclient.WorkerDeploymentSetRampingVersionOptions{
			BuildID:                 opts.BuildID,
			Percentage:              float32(opts.Percentage),
			Identity:                "onlava-cli",
			IgnoreMissingTaskQueues: opts.IgnoreMissingTaskQueues,
			AllowNoPollers:          opts.AllowNoPollers,
		})
	case "drain":
		_, err = handle.DeleteVersion(ctx, temporalclient.WorkerDeploymentDeleteVersionOptions{
			BuildID:      opts.BuildID,
			SkipDrainage: opts.Force,
			Identity:     "onlava-cli",
		})
	}
	if err != nil {
		return fmt.Errorf("temporal deployment %s %s build %s: %w", action, onlavaruntime.TemporalDeploymentName(info), opts.BuildID, err)
	}
	result := temporalDeploymentResult{
		OK:         true,
		Action:     action,
		Deployment: onlavaruntime.TemporalDeploymentName(info),
		BuildID:    opts.BuildID,
		Percentage: opts.Percentage,
		Namespace:  info.Namespace,
		Address:    info.Address,
	}
	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	if _, err := fmt.Fprintf(stdout, "temporal deployment %s applied to %s build %s\n", action, result.Deployment, result.BuildID); err != nil {
		return err
	}
	return nil
}
