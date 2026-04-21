//go:build windows

package runtime

func processExists(pid int) bool {
	return pid > 0
}
