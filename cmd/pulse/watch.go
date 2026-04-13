package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"pulse.dev/internal/app"
	"pulse.dev/internal/build"
)

const (
	watchPollInterval = 250 * time.Millisecond
	watchSettleDelay  = 500 * time.Millisecond
	stopTimeout       = 5 * time.Second
)

type fileStamp struct {
	modTime time.Time
	size    int64
}

type fileSnapshot map[string]fileStamp

type runningApp struct {
	cmd      *exec.Cmd
	done     chan error
	buildDir string
}

type appRunner struct {
	root string
	addr string
	app  *runningApp
}

func runWithWatch(addr string) error {
	root, _, err := app.DiscoverRoot(".")
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	snapshot, err := scanWatchedFiles(root)
	if err != nil {
		return err
	}

	runner := &appRunner{root: root, addr: addr}
	defer runner.close()

	if err := runner.rebuildAndRestart(); err != nil {
		fmt.Fprintf(os.Stderr, "pulse: initial build failed:\n%v\n", err)
	}

	for {
		nextSnapshot, err := waitForStableChange(ctx, root, snapshot)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
		snapshot = nextSnapshot
		fmt.Fprintln(os.Stdout, "pulse: change detected, rebuilding...")
		if err := runner.rebuildAndRestart(); err != nil {
			fmt.Fprintf(os.Stderr, "pulse: rebuild failed:\n%v\n", err)
		}
	}
}

func (r *appRunner) rebuildAndRestart() error {
	_, cfg, err := app.DiscoverRoot(r.root)
	if err != nil {
		return err
	}

	result, err := build.App(r.root, cfg.Name)
	if err != nil {
		return err
	}

	previous := r.app
	if previous != nil {
		if err := previous.stop(); err != nil {
			return err
		}
	}

	current, err := startApp(r.root, r.addr, result)
	if err != nil {
		_ = os.RemoveAll(result.Dir)
		return err
	}
	r.app = current
	return nil
}

func (r *appRunner) close() error {
	if r.app == nil {
		return nil
	}
	err := r.app.stop()
	r.app = nil
	return err
}

func startApp(root, addr string, result *build.Result) (*runningApp, error) {
	cmd := exec.Command(result.Binary)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PULSE_LISTEN_ADDR="+addr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
		close(done)
	}()

	return &runningApp{
		cmd:      cmd,
		done:     done,
		buildDir: result.Dir,
	}, nil
}

func (r *runningApp) stop() error {
	if r == nil {
		return nil
	}
	defer func() {
		if r.buildDir != "" {
			_ = os.RemoveAll(r.buildDir)
		}
	}()

	if r.cmd == nil || r.cmd.Process == nil {
		return nil
	}

	_ = r.cmd.Process.Signal(os.Interrupt)

	select {
	case err := <-r.done:
		if err == nil || isExpectedExit(err) {
			return nil
		}
		return err
	case <-time.After(stopTimeout):
		_ = r.cmd.Process.Kill()
		err := <-r.done
		if err == nil || isExpectedExit(err) {
			return nil
		}
		return err
	}
}

func isExpectedExit(err error) bool {
	if err == nil {
		return true
	}
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr)
}

func waitForStableChange(ctx context.Context, root string, current fileSnapshot) (fileSnapshot, error) {
	ticker := time.NewTicker(watchPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}

		next, err := scanWatchedFiles(root)
		if err != nil {
			return nil, err
		}
		if snapshotsEqual(current, next) {
			continue
		}
		return waitForSnapshotToSettle(ctx, root, next)
	}
}

func waitForSnapshotToSettle(ctx context.Context, root string, current fileSnapshot) (fileSnapshot, error) {
	timer := time.NewTimer(watchSettleDelay)
	defer timer.Stop()
	ticker := time.NewTicker(watchPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return current, nil
		case <-ticker.C:
			next, err := scanWatchedFiles(root)
			if err != nil {
				return nil, err
			}
			if snapshotsEqual(current, next) {
				continue
			}
			current = next
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(watchSettleDelay)
		}
	}
}

func scanWatchedFiles(root string) (fileSnapshot, error) {
	snapshot := make(fileSnapshot)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		if d.IsDir() {
			if shouldSkipWatchDir(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !isWatchedFile(rel) {
			return nil
		}

		info, err := d.Info()
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		snapshot[filepath.ToSlash(rel)] = fileStamp{
			modTime: info.ModTime().UTC().Round(0),
			size:    info.Size(),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

func shouldSkipWatchDir(rel string) bool {
	base := filepath.Base(rel)
	if strings.HasPrefix(base, ".") {
		return true
	}
	switch base {
	case "node_modules", "pulse_internal_main":
		return true
	default:
		return false
	}
}

func isWatchedFile(rel string) bool {
	switch filepath.Base(rel) {
	case "pulse.app", "go.mod", "go.sum", "go.work", "go.work.sum", ".env", ".env.local":
		return true
	}
	return filepath.Ext(rel) == ".go"
}

func snapshotsEqual(a, b fileSnapshot) bool {
	if len(a) != len(b) {
		return false
	}
	for path, stamp := range a {
		if other, ok := b[path]; !ok || other != stamp {
			return false
		}
	}
	return true
}
