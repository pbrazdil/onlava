//go:build darwin

package localproxy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func installLocalCATrustOS(certPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	keychain := filepath.Join(home, "Library", "Keychains", "login.keychain-db")
	cmd := exec.Command("security", "add-trusted-cert", "-r", "trustRoot", "-p", "ssl", "-p", "basic", "-k", keychain, certPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("security add-trusted-cert: %w: %s", err, out)
	}
	return nil
}
