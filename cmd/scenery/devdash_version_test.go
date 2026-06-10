package main

import "testing"

func TestSceneryDashboardCompatVersion(t *testing.T) {
	t.Parallel()

	if sceneryDashboardCompatVersion == "" {
		t.Fatal("scenery dashboard compat version must not be empty")
	}
	if sceneryDashboardCompatChannel != "ga" {
		t.Fatalf("unexpected dashboard compat channel: %q", sceneryDashboardCompatChannel)
	}
}
