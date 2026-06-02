package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	localagent "github.com/pbrazdil/onlava/internal/agent"
)

const staleSessionCleanupGrace = 2 * time.Second

func cleanupStaleDevSessionProcesses(ctx context.Context, current localagent.Session, previous []localagent.Session) error {
	if strings.TrimSpace(current.AppRoot) == "" || strings.TrimSpace(current.SessionID) == "" {
		return nil
	}
	var errs []error
	seen := map[int]bool{}
	for _, session := range previous {
		if !sameAgentSession(current, session) {
			continue
		}
		errs = append(errs, stopStaleRegisteredSessionProcesses(ctx, current, session, seen))
	}
	errs = append(errs, stopSessionCommandProcesses(ctx, current, seen))
	errs = append(errs, stopSessionEnvProcesses(ctx, current, seen))
	return errors.Join(errs...)
}

func sameAgentSession(a, b localagent.Session) bool {
	return cleanAbsPath(a.AppRoot) == cleanAbsPath(b.AppRoot) &&
		strings.TrimSpace(a.SessionID) == strings.TrimSpace(b.SessionID)
}

func stopStaleRegisteredSessionProcesses(ctx context.Context, current, previous localagent.Session, seen map[int]bool) error {
	var errs []error
	currentOwnerPID := firstPositiveInt(current.OwnerPID, current.Owner.PID)
	previousOwnerPID := firstPositiveInt(previous.OwnerPID, previous.Owner.PID)
	if previousOwnerPID > 0 && previousOwnerPID != os.Getpid() && previousOwnerPID != currentOwnerPID {
		if shouldSignalSessionOwner(previous) {
			errs = append(errs, stopSessionOwnerPID(ctx, previousOwnerPID))
			seen[previousOwnerPID] = true
		}
	}
	for _, pid := range sessionProcessPIDs(previous) {
		if pid <= 0 || pid == os.Getpid() || pid == currentOwnerPID || seen[pid] {
			continue
		}
		if err := stopStaleSessionChildPID(ctx, pid); err != nil {
			errs = append(errs, err)
		}
		seen[pid] = true
	}
	return errors.Join(errs...)
}

func shouldSignalSessionOwner(session localagent.Session) bool {
	owner := session.Owner
	effectivePID := firstPositiveInt(session.OwnerPID, owner.PID)
	if owner.PID != effectivePID {
		owner = localagent.Owner{}
	}
	if owner.PID <= 0 {
		owner.PID = session.OwnerPID
	}
	if owner.PID <= 0 {
		return false
	}
	if err := localagent.VerifyOwner(owner); err == nil {
		return true
	}
	info, ok := inspectProcess(owner.PID)
	return ok && looksLikeOnlavaDashboardProcess(info)
}

func stopSessionOwnerPID(ctx context.Context, pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}
	if err := proc.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	if waitForPIDExit(ctx, pid, staleSessionCleanupGrace) {
		return nil
	}
	if err := proc.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	if waitForPIDExit(ctx, pid, time.Second) {
		return nil
	}
	return fmt.Errorf("stale onlava dev owner process %d did not exit after SIGKILL", pid)
}

func stopStaleSessionChildPID(ctx context.Context, pid int) error {
	if err := terminateProcessIDTree(pid); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	if waitForPIDExit(ctx, pid, staleSessionCleanupGrace) {
		return nil
	}
	if err := killProcessIDTree(pid); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	if waitForPIDExit(ctx, pid, time.Second) {
		return nil
	}
	return fmt.Errorf("stale onlava session child process %d did not exit after SIGKILL", pid)
}

func waitForPIDExit(ctx context.Context, pid int, timeout time.Duration) bool {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		if info, ok := inspectProcess(pid); !ok || strings.Contains(info.stat, "Z") {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-deadline.C:
			return false
		case <-ticker.C:
		}
	}
}

func sessionProcessPIDs(session localagent.Session) []int {
	seen := map[int]bool{}
	var pids []int
	if pid := atoiPID(session.AppPID); pid > 0 {
		process := session.Processes[localagent.RouteAPI]
		process.PID = pid
		if shouldSignalSessionProcess(process) {
			seen[pid] = true
			pids = append(pids, pid)
		}
	}
	for _, process := range session.Processes {
		if process.PID > 0 && !seen[process.PID] && shouldSignalSessionProcess(process) {
			seen[process.PID] = true
			pids = append(pids, process.PID)
		}
	}
	return pids
}

func shouldSignalSessionProcess(process localagent.Process) bool {
	if process.PID <= 0 {
		return false
	}
	if process.Owner.PID <= 0 {
		info, ok := inspectProcess(process.PID)
		return ok && looksLikeOnlavaSessionChildProcess(info)
	}
	return process.Owner.PID == process.PID && localagent.VerifyOwner(process.Owner) == nil
}

func looksLikeOnlavaSessionChildProcess(info procInfo) bool {
	command := strings.ToLower(filepath.ToSlash(strings.TrimSpace(info.cmd)))
	return strings.Contains(command, "/.onlava/") ||
		strings.Contains(command, "onlava-app-") ||
		strings.Contains(command, "worker.ts")
}

func stopSessionCommandProcesses(ctx context.Context, current localagent.Session, seen map[int]bool) error {
	stateRoot := filepath.ToSlash(cleanAbsPath(current.StateRoot))
	if stateRoot == "" {
		return nil
	}
	output, err := exec.Command("ps", "-axo", "pid=,stat=,command=").Output()
	if err != nil {
		return nil
	}
	var errs []error
	for _, line := range strings.Split(string(output), "\n") {
		pid, stat, command, ok := parsePSCommandLine(line)
		if !ok || pid <= 0 || pid == os.Getpid() || strings.Contains(stat, "Z") || seen[pid] {
			continue
		}
		if !commandMatchesSessionStateRoot(command, stateRoot) {
			continue
		}
		if err := stopStaleSessionChildPID(ctx, pid); err != nil {
			errs = append(errs, err)
		}
		seen[pid] = true
	}
	return errors.Join(errs...)
}

func parsePSCommandLine(line string) (int, string, string, bool) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 3 {
		return 0, "", "", false
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, "", "", false
	}
	return pid, fields[1], strings.Join(fields[2:], " "), true
}

func commandMatchesSessionStateRoot(command, stateRoot string) bool {
	command = filepath.ToSlash(strings.TrimSpace(command))
	return strings.Contains(command, stateRoot+"/") && strings.Contains(command, "onlava-app")
}

func atoiPID(value string) int {
	pid, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return pid
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
