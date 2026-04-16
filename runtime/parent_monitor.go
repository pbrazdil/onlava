package runtime

import (
	"context"
	"os"
	"time"
)

var (
	supervisorParentCheckInterval = time.Second
	supervisorParentPID           = os.Getppid
)

func startSupervisorParentMonitor(cancel context.CancelFunc) func() {
	if !launchedBySupervisor() {
		return func() {}
	}

	initial := supervisorParentPID()
	if initial <= 1 {
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
				if supervisorParentMonitorShouldCancel(initial, supervisorParentPID()) {
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

func supervisorParentMonitorShouldCancel(initial, current int) bool {
	return initial > 1 && current > 0 && current != initial
}
