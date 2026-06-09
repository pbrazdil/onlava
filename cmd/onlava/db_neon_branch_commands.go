package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	appcfg "github.com/pbrazdil/onlava/internal/app"
)

func dbBranchCommand(args []string) error {
	return runDBBranchCommand(context.Background(), os.Stdout, args)
}

func runDBBranchCommand(ctx context.Context, stdout io.Writer, args []string) error {
	opts, err := parseDBBranchArgs(args)
	if err != nil {
		return err
	}
	switch opts.Command {
	case "status":
		return runDBBranchStatus(ctx, stdout, opts)
	case "list":
		return runDBBranchList(ctx, stdout, opts)
	case "checkout":
		return runDBBranchCheckout(ctx, stdout, opts)
	case "reset":
		return runDBBranchReset(ctx, stdout, opts)
	case "delete":
		return runDBBranchDelete(ctx, stdout, opts)
	case "restore":
		return runDBBranchRestore(ctx, stdout, opts)
	case "diff":
		return runDBBranchDiff(ctx, stdout, opts)
	case "expire":
		return runDBBranchExpire(ctx, stdout, opts)
	case "prune":
		return runDBBranchPrune(ctx, stdout, opts)
	default:
		return fmt.Errorf("db branch %s is not implemented yet", opts.Command)
	}
}

func parseDBBranchArgs(args []string) (dbBranchOptions, error) {
	if len(args) == 0 {
		return dbBranchOptions{}, fmt.Errorf("usage: onlava db branch status|list|checkout|reset|delete|restore|diff|expire|prune [--json] [--app-root <path>]")
	}
	opts := dbBranchOptions{Command: args[0]}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--app-root":
			i++
			if i >= len(args) {
				return dbBranchOptions{}, fmt.Errorf("missing value for --app-root")
			}
			opts.AppRoot = args[i]
		case "--json":
			opts.JSON = true
		case "--yes":
			opts.Yes = true
		case "--force":
			opts.Force = true
		case "--at":
			i++
			if i >= len(args) {
				return dbBranchOptions{}, fmt.Errorf("missing value for --at")
			}
			opts.At = args[i]
		case "--after":
			i++
			if i >= len(args) {
				return dbBranchOptions{}, fmt.Errorf("missing value for --after")
			}
			opts.After = args[i]
		case "--older-than":
			i++
			if i >= len(args) {
				return dbBranchOptions{}, fmt.Errorf("missing value for --older-than")
			}
			opts.Older = args[i]
		default:
			if strings.HasPrefix(args[i], "-") {
				return dbBranchOptions{}, fmt.Errorf("unknown flag %q", args[i])
			}
			if opts.Branch == "" {
				opts.Branch = args[i]
			} else if opts.Target == "" {
				opts.Target = args[i]
			} else {
				return dbBranchOptions{}, fmt.Errorf("unexpected argument %q", args[i])
			}
		}
	}
	switch opts.Command {
	case "status", "list", "checkout", "reset", "delete", "prune", "restore", "diff", "expire":
	default:
		return dbBranchOptions{}, fmt.Errorf("unknown db branch command %q", opts.Command)
	}
	return opts, nil
}

func runDBBranchStatus(ctx context.Context, stdout io.Writer, opts dbBranchOptions) error {
	appRoot, cfg, err := discoverConfiguredApp(opts.AppRoot)
	if err != nil {
		return err
	}
	result, err := buildDBBranchStatus(ctx, appRoot, cfg)
	if err != nil {
		return err
	}
	if opts.JSON {
		return writeInspectJSON(stdout, result)
	}
	if result.Pin != nil {
		fmt.Fprintf(stdout, "db branch %s (%s)\n", result.Pin.Branch, result.Pin.BranchID)
		return nil
	}
	fmt.Fprintf(stdout, "db branch %s\n", result.Status)
	return nil
}

func runDBBranchList(ctx context.Context, stdout io.Writer, opts dbBranchOptions) error {
	appRoot, cfg, err := discoverConfiguredApp(opts.AppRoot)
	if err != nil {
		return err
	}
	status, err := buildDBBranchStatus(ctx, appRoot, cfg)
	if err != nil {
		return err
	}
	registry, registryPath, err := readNeonBranchRegistryForDefaultRoot()
	if err != nil {
		return err
	}
	result := dbBranchListResult{
		SchemaVersion: dbBranchListSchemaVersion,
		OK:            true,
		App:           status.App,
		Provider:      status.Provider,
		Branches:      []worktreeDBPin{},
		RegistryPath:  registryPath,
	}
	provider := neonBranchProviderForConfig(cfg)
	seen := map[string]bool{}
	for _, lease := range registry.Leases {
		if !isOnlavaOwnedNeonLease(lease) {
			continue
		}
		if lease.Pin.Project != sanitizeNeonBranchSegment(firstNonEmpty(neonPostgresService(cfg).Project, cfg.AppID(), "app")) {
			continue
		}
		result.Branches = append(result.Branches, lease.Pin)
		result.Leases = append(result.Leases, dbBranchListLeaseFromRegistryLease(ctx, provider, lease))
		seen[lease.Pin.BranchID] = true
	}
	if status.Pin != nil && !seen[status.Pin.BranchID] {
		result.Branches = append(result.Branches, *status.Pin)
		result.Leases = append(result.Leases, dbBranchListLease{
			Pin:      *status.Pin,
			Status:   firstNonEmpty(status.BackendStatus, "missing"),
			Endpoint: cloneNeonEndpoint(status.Connection),
		})
	}
	if len(result.Branches) == 0 {
		result.Message = "No Onlava-owned Neon branch leases exist for this app."
	}
	if opts.JSON {
		return writeInspectJSON(stdout, result)
	}
	for _, branch := range result.Branches {
		fmt.Fprintf(stdout, "%s %s\n", branch.Branch, branch.BranchID)
	}
	return nil
}

func runDBBranchCheckout(ctx context.Context, stdout io.Writer, opts dbBranchOptions) error {
	appRoot, cfg, err := discoverConfiguredApp(opts.AppRoot)
	if err != nil {
		return err
	}
	branch := strings.TrimSpace(opts.Branch)
	if branch == "" {
		return fmt.Errorf("usage: onlava db branch checkout <name> [--app-root <path>] [--json]")
	}
	pin, err := buildWorktreeDBPin(appRoot, cfg, branch)
	if err != nil {
		return err
	}
	if err := writeWorktreeDBPin(appRoot, pin); err != nil {
		return err
	}
	if _, err := neonBranchProviderForConfig(cfg).EnsureBranch(ctx, pin); err != nil {
		return err
	}
	result, err := buildDBBranchStatus(ctx, appRoot, cfg)
	if err != nil {
		return err
	}
	result.Message = "Current worktree database branch pin updated. Neon branch provider ensure ran; connection becomes usable when backend_status is ready."
	if opts.JSON {
		return writeInspectJSON(stdout, result)
	}
	fmt.Fprintf(stdout, "checked out db branch %s (%s)\n", pin.Branch, pin.BranchID)
	return nil
}

func runDBBranchExpire(ctx context.Context, stdout io.Writer, opts dbBranchOptions) error {
	appRoot, cfg, err := discoverConfiguredApp(opts.AppRoot)
	if err != nil {
		return err
	}
	if strings.TrimSpace(opts.After) == "" {
		return fmt.Errorf("onlava db branch expire requires --after <duration>")
	}
	after, err := time.ParseDuration(strings.TrimSpace(opts.After))
	if err != nil {
		return fmt.Errorf("parse --after: %w", err)
	}
	target, err := resolveBranchCommandTarget(appRoot, cfg, opts)
	if err != nil {
		return err
	}
	if err := expireNeonBranchLease(target, time.Now().UTC().Add(after)); err != nil {
		return err
	}
	result, err := buildDBBranchStatus(ctx, appRoot, cfg)
	if err != nil {
		return err
	}
	result.Message = fmt.Sprintf("Local Neon branch lease %q expiration updated. Backend expiration is not implemented yet.", target.Branch)
	if opts.JSON {
		return writeInspectJSON(stdout, result)
	}
	fmt.Fprintf(stdout, "updated db branch lease expiration for %s\n", target.Branch)
	return nil
}

func runDBBranchPrune(ctx context.Context, stdout io.Writer, opts dbBranchOptions) error {
	appRoot, cfg, err := discoverConfiguredApp(opts.AppRoot)
	if err != nil {
		return err
	}
	var olderThan time.Duration
	if strings.TrimSpace(opts.Older) != "" {
		olderThan, err = time.ParseDuration(strings.TrimSpace(opts.Older))
		if err != nil {
			return fmt.Errorf("parse --older-than: %w", err)
		}
	}
	current, _, err := readWorktreeDBPin(worktreeDBPinPath(appRoot))
	if err != nil {
		return err
	}
	project := sanitizeNeonBranchSegment(firstNonEmpty(neonPostgresService(cfg).Project, cfg.AppID(), "app"))
	pruned, err := pruneExpiredNeonBranchLeases(project, current.BranchID, olderThan)
	if err != nil {
		return err
	}
	status, err := buildDBBranchStatus(ctx, appRoot, cfg)
	if err != nil {
		return err
	}
	registry, registryPath, err := readNeonBranchRegistryForDefaultRoot()
	if err != nil {
		return err
	}
	result := dbBranchListResult{
		SchemaVersion: dbBranchListSchemaVersion,
		OK:            true,
		App:           status.App,
		Provider:      status.Provider,
		Branches:      registryPins(registry, cfg),
		Leases:        registryListLeases(ctx, registry, cfg),
		RegistryPath:  registryPath,
		Message:       fmt.Sprintf("Pruned %d expired local Neon branch lease(s). Backend branch deletion is not implemented yet.", pruned),
	}
	if opts.JSON {
		return writeInspectJSON(stdout, result)
	}
	fmt.Fprintf(stdout, "pruned %d expired db branch lease(s)\n", pruned)
	return nil
}

func runDBBranchReset(ctx context.Context, _ io.Writer, opts dbBranchOptions) error {
	appRoot, cfg, err := discoverConfiguredApp(opts.AppRoot)
	if err != nil {
		return err
	}
	pin, ok, err := readWorktreeDBPin(worktreeDBPinPath(appRoot))
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no worktree database branch pin exists; run `onlava db branch checkout <name>` first")
	}
	if isProtectedNeonParentBranch(pin) {
		return fmt.Errorf("refusing to reset protected parent branch %q", pin.Branch)
	}
	if !opts.Yes {
		return fmt.Errorf("onlava db branch reset requires --yes")
	}
	return neonBranchProviderForConfig(cfg).ResetBranch(ctx, pin, opts)
}

func runDBBranchDelete(ctx context.Context, _ io.Writer, opts dbBranchOptions) error {
	appRoot, cfg, err := discoverConfiguredApp(opts.AppRoot)
	if err != nil {
		return err
	}
	opts.AppRoot = appRoot
	branch := normalizeNeonBranchName(opts.Branch)
	if branch == "" {
		return fmt.Errorf("usage: onlava db branch delete <name> [--app-root <path>] [--force]")
	}
	pin, ok, err := readWorktreeDBPin(worktreeDBPinPath(appRoot))
	if err != nil {
		return err
	}
	targetPin := pin
	if !ok {
		targetPin, err = buildWorktreeDBPin(appRoot, cfg, branch)
		if err != nil {
			return err
		}
	}
	if branch == targetPin.ParentBranch {
		return fmt.Errorf("refusing to delete protected parent branch %q", branch)
	}
	if ok && branch == pin.Branch && !opts.Force {
		return fmt.Errorf("refusing to delete current branch %q without --force", branch)
	}
	return neonBranchProviderForConfig(cfg).DeleteBranch(ctx, targetPin, branch, opts)
}

func buildDBBranchStatus(ctx context.Context, appRoot string, cfg appcfg.Config) (dbBranchStatusResult, error) {
	pinPath := worktreeDBPinPath(appRoot)
	pin, ok, err := readWorktreeDBPin(pinPath)
	if err != nil {
		return dbBranchStatusResult{}, err
	}
	status := "unpinned"
	var pinPtr *worktreeDBPin
	backendStatus := neonBranchBackendStatus{Status: "none"}
	if ok {
		status = "pinned"
		pinPtr = &pin
		backendStatus = neonBranchProviderForConfig(cfg).InspectBranch(ctx, pin)
	}
	return dbBranchStatusResult{
		SchemaVersion:  dbBranchStatusSchemaVersion,
		OK:             true,
		App:            inspectAppRef(appRoot, cfg),
		Provider:       neonSelfhostProvider,
		Status:         status,
		BackendStatus:  backendStatus.Status,
		BackendMessage: backendStatus.Message,
		Connection:     backendStatus.Endpoint,
		PinPath:        pinPath,
		Pin:            pinPtr,
		DatabaseURLEnv: neonDatabaseURLEnv(cfg),
		PSQLCommand:    "onlava db psql",
		ResetCommand:   "onlava db branch reset",
		Message:        dbBranchStatusMessage(ok),
	}, nil
}

func dbBranchStatusMessage(pinned bool) string {
	if pinned {
		return "Current worktree database branch pin is present."
	}
	return "No worktree database branch pin exists yet; run `onlava db branch checkout <name>` to pin this worktree."
}
