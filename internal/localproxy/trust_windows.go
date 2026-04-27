//go:build windows

package localproxy

import (
	"fmt"
	"os/exec"
)

func installLocalCATrustOS(certPath string) error {
	cmd := exec.Command("certutil", "-user", "-addstore", "Root", certPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("certutil -user -addstore Root: %w: %s", err, out)
	}
	return nil
}
