package runtime

import (
	"context"
	"os"
	"strconv"
	"time"
)

var (
	supervisorParentCheckInterval = time.Second
	supervisorParentPID           = os.Getppid
	supervisorProcessExists       = processExists
)

func startSupervisorParentMonitor(cancel context.CancelFunc) func() {
	if !launchedBySupervisor() {
		return func() {}
	}

	supervisorPID := supervisorPIDFromEnv()
	initial := supervisorParentPID()
	if supervisorPID <= 1 && initial <= 1 {
		return func() {}
	}

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(supervisorParentCheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if supervisorParentMonitorShouldCancel(supervisorPID, supervisorProcessExists(supervisorPID), initial, supervisorParentPID()) {
					cancel()
					return
				}
			}
		}
	}()
	return func() {
		close(done)
	}
}

func supervisorPIDFromEnv() int {
	value := os.Getenv("PULSE_DEV_SUPERVISOR_PID")
	if value == "" {
		return 0
	}
	pid, err := strconv.Atoi(value)
	if err != nil || pid <= 0 {
		return 0
	}
	return pid
}

func supervisorParentMonitorShouldCancel(supervisorPID int, supervisorAlive bool, initial, current int) bool {
	if supervisorPID > 1 {
		return !supervisorAlive
	}
	return initial > 1 && current > 0 && current != initial
}
