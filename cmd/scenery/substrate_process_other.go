//go:build !(aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris)

package main

import "os"

func processExitSignal(state *os.ProcessState) string {
	return ""
}
