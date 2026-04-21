package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"

	"pulse.dev/internal/app"
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

func runWithWatch(addr string, verbose bool, appRoot string) error {
	start, err := resolveAppRoot(appRoot)
	if err != nil {
		return err
	}
	root, cfg, err := app.DiscoverRoot(start)
	if err != nil {
		return err
	}

	sigCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		stopSignals()
		cancel()
	}()
	go func() {
		select {
		case <-sigCtx.Done():
			stopSignals()
			cancel()
		case <-ctx.Done():
		}
	}()
	stopParentMonitor := startParentMonitor(ctx, cancel)
	defer stopParentMonitor()

	snapshot, err := scanWatchedFiles(root)
	if err != nil {
		return err
	}

	supervisor, err := newDevSupervisor(ctx, root, cfg, addr, verbose)
	if err != nil {
		return err
	}
	defer supervisor.Close()
	if err := supervisor.Start(ctx); err != nil {
		return err
	}

	if err := supervisor.RebuildAndRestart(ctx, true, snapshot, nil); err != nil {
		supervisor.console.InitialBuildFailed(err)
	}

	watcher, err := newFileChangeWatcher(root)
	if err != nil {
		if verbose {
			supervisor.console.printf(supervisor.console.err, "  %s\n\n", err.Error())
		}
	}
	if watcher != nil {
		defer watcher.Close()
	}

	for {
		nextSnapshot, err := waitForStableChange(ctx, root, snapshot, watcher)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
		paths := changedPaths(snapshot, nextSnapshot)
		snapshot = nextSnapshot
		supervisor.announceRebuild(paths)
		if err := supervisor.RebuildAndRestart(ctx, false, snapshot, paths); err != nil {
			supervisor.console.RebuildFailed(err)
		}
	}
}

func waitForStableChange(ctx context.Context, root string, current fileSnapshot, watcher *fileChangeWatcher) (fileSnapshot, error) {
	if watcher != nil {
		return waitForStableChangeEvents(ctx, root, current, watcher.Events())
	}
	return waitForStableChangePolling(ctx, root, current)
}

func waitForStableChangePolling(ctx context.Context, root string, current fileSnapshot) (fileSnapshot, error) {
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
		return waitForSnapshotToSettlePolling(ctx, root, next)
	}
}

func waitForStableChangeEvents(ctx context.Context, root string, current fileSnapshot, events <-chan struct{}) (fileSnapshot, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case _, ok := <-events:
			if !ok {
				return waitForStableChangePolling(ctx, root, current)
			}
		}

		next, err := waitForSnapshotToSettleEvents(ctx, root, events)
		if err != nil {
			return nil, err
		}
		if snapshotsEqual(current, next) {
			continue
		}
		return next, nil
	}
}

func waitForSnapshotToSettlePolling(ctx context.Context, root string, current fileSnapshot) (fileSnapshot, error) {
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

func waitForSnapshotToSettleEvents(ctx context.Context, root string, events <-chan struct{}) (fileSnapshot, error) {
	timer := time.NewTimer(watchSettleDelay)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return scanWatchedFiles(root)
		case _, ok := <-events:
			if !ok {
				return scanWatchedFiles(root)
			}
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

func shouldIgnoreWatchPath(rel string) bool {
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." || rel == "" {
		return false
	}
	for _, part := range strings.Split(rel, "/") {
		if part == "" || part == "." {
			continue
		}
		if strings.HasPrefix(part, ".") {
			return true
		}
		switch part {
		case "node_modules", "pulse_internal_main":
			return true
		}
	}
	return false
}

func isWatchedFile(rel string) bool {
	if filepath.Base(rel) == "pulse.app" {
		return true
	}
	if filepath.Base(rel) == "encore.gen.go" {
		return false
	}
	switch filepath.Ext(rel) {
	case ".go", ".cpp", ".h":
		return true
	default:
		return false
	}
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

func changedPaths(before, after fileSnapshot) []string {
	seen := make(map[string]struct{}, len(before)+len(after))
	paths := make([]string, 0, len(before)+len(after))
	for path, stamp := range before {
		seen[path] = struct{}{}
		if other, ok := after[path]; !ok || other != stamp {
			paths = append(paths, path)
		}
	}
	for path := range after {
		if _, ok := seen[path]; ok {
			continue
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func snapshotFingerprint(snapshot fileSnapshot) string {
	paths := make([]string, 0, len(snapshot))
	for path := range snapshot {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, path := range paths {
		stamp := snapshot[path]
		_, _ = h.Write([]byte(path))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(stamp.modTime.Format(time.RFC3339Nano)))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(fmt.Sprintf("%d", stamp.size)))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

type fileChangeWatcher struct {
	events  chan struct{}
	watcher *fsnotify.Watcher
	root    string
	done    chan struct{}
}

func newFileChangeWatcher(root string) (*fileChangeWatcher, error) {
	underlying, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	fw := &fileChangeWatcher{
		events:  make(chan struct{}, 1),
		watcher: underlying,
		root:    root,
		done:    make(chan struct{}),
	}
	if err := fw.addTree(root); err != nil {
		_ = underlying.Close()
		return nil, err
	}
	go fw.run()
	return fw, nil
}

func (fw *fileChangeWatcher) Events() <-chan struct{} {
	if fw == nil {
		return nil
	}
	return fw.events
}

func (fw *fileChangeWatcher) Close() error {
	if fw == nil {
		return nil
	}
	err := fw.watcher.Close()
	<-fw.done
	return err
}

func (fw *fileChangeWatcher) run() {
	defer close(fw.done)
	defer close(fw.events)
	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			fw.handleEvent(event)
		case _, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			fw.signal()
		}
	}
}

func (fw *fileChangeWatcher) handleEvent(event fsnotify.Event) {
	path := filepath.Clean(event.Name)
	rel, err := filepath.Rel(fw.root, path)
	if err != nil {
		fw.signal()
		return
	}
	if rel == "." {
		return
	}
	if shouldIgnoreWatchPath(rel) {
		return
	}
	if event.Has(fsnotify.Create) {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			_ = fw.addTree(path)
		}
	}
	fw.signal()
}

func (fw *fileChangeWatcher) signal() {
	select {
	case fw.events <- struct{}{}:
	default:
	}
}

func (fw *fileChangeWatcher) addTree(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(fw.root, path)
		if err != nil {
			return err
		}
		if rel != "." && shouldIgnoreWatchPath(rel) {
			return filepath.SkipDir
		}
		return fw.watcher.Add(path)
	})
}
