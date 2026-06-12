//go:build windows

package main

import "os"

func validateEdgeTargetOwner(_ string, _ os.FileInfo, _, _ int) error {
	return nil
}
