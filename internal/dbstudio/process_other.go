//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd

package dbstudio

import (
	"os"
	"os/exec"
	"syscall"
)

func configureChildProcess(cmd *exec.Cmd) {}

func interruptProcessTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Signal(os.Interrupt)
}

func killProcessTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Signal(syscall.SIGKILL)
}

func isExpectedExit(err error) bool {
	_, ok := err.(*exec.ExitError)
	return ok
}
