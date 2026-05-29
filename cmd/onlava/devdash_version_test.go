package main

import "testing"

func TestOnlavaDashboardCompatVersion(t *testing.T) {
	t.Parallel()

	if onlavaDashboardCompatVersion == "" {
		t.Fatal("onlava dashboard compat version must not be empty")
	}
	if onlavaDashboardCompatChannel != "ga" {
		t.Fatalf("unexpected dashboard compat channel: %q", onlavaDashboardCompatChannel)
	}
}
