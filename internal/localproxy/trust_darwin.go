//go:build darwin

package localproxy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func localCATrustedOS(certPath string) (bool, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return false, err
	}
	cert, err := parseCertificatePEM(certPEM)
	if err != nil {
		return false, err
	}
	sum := sha256.Sum256(cert.Raw)
	fingerprint := strings.ToUpper(hex.EncodeToString(sum[:]))
	home, err := os.UserHomeDir()
	if err != nil {
		return false, err
	}
	keychain := filepath.Join(home, "Library", "Keychains", "login.keychain-db")
	cmd := exec.Command("security", "find-certificate", "-a", "-c", cert.Subject.CommonName, "-Z", "-p", keychain)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, nil
	}
	return certificateOutputHasSHA256(out, fingerprint), nil
}

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
