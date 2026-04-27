//go:build !darwin && !linux && !windows

package localproxy

import "fmt"

func installLocalCATrustOS(certPath string) error {
	return fmt.Errorf("local CA trust installation is not supported on this OS")
}
