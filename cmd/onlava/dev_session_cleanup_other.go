//go:build !linux

package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"

	localagent "github.com/pbrazdil/onlava/internal/agent"
)

func stopSessionEnvProcesses(ctx context.Context, current localagent.Session, seen map[int]bool) error {
	output, err := exec.Command("ps", "eww", "-axo", "pid=,stat=,command=").Output()
	if err != nil {
		return nil
	}
	var errs []error
	for _, line := range strings.Split(string(output), "\n") {
		pid, stat, env, ok := parsePSEnvProcessLine(line)
		if !ok || pid <= 0 || pid == os.Getpid() || strings.Contains(stat, "Z") || seen[pid] {
			continue
		}
		if !envMatchesSession(env, current) || !onlavaOwnedSessionEnv(env) {
			continue
		}
		if err := stopStaleSessionChildPID(ctx, pid); err != nil {
			errs = append(errs, err)
		}
		seen[pid] = true
	}
	return errors.Join(errs...)
}

func parsePSEnvProcessLine(line string) (int, string, map[string]string, bool) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 3 {
		return 0, "", nil, false
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, "", nil, false
	}
	env := map[string]string{}
	for _, field := range fields[2:] {
		name, value, ok := strings.Cut(field, "=")
		if !ok || name == "" {
			continue
		}
		env[name] = value
	}
	return pid, fields[1], env, true
}

func envMatchesSession(env map[string]string, session localagent.Session) bool {
	return cleanAbsPath(env["ONLAVA_APP_ROOT"]) == cleanAbsPath(session.AppRoot) &&
		strings.TrimSpace(env["ONLAVA_SESSION_ID"]) == strings.TrimSpace(session.SessionID)
}

func onlavaOwnedSessionEnv(env map[string]string) bool {
	return strings.TrimSpace(env["ONLAVA_DEV_SUPERVISOR"]) == "1"
}
