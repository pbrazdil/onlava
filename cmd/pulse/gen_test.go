package main

import "testing"

func TestParseGenClientArgsAcceptsEncoreStyleInvocation(t *testing.T) {
	opts, err := parseGenClientArgs([]string{
		"onlvnext-o5o2",
		"--lang=typescript",
		"--output=apps/pulse/src/encore-client.ts",
		"--app-root",
		"/tmp/app",
	})
	if err != nil {
		t.Fatalf("parseGenClientArgs() error = %v", err)
	}
	if opts.Target != "onlvnext-o5o2" {
		t.Fatalf("Target = %q", opts.Target)
	}
	if opts.Lang != "typescript" {
		t.Fatalf("Lang = %q", opts.Lang)
	}
	if opts.Output != "apps/pulse/src/encore-client.ts" {
		t.Fatalf("Output = %q", opts.Output)
	}
	if opts.AppRoot != "/tmp/app" {
		t.Fatalf("AppRoot = %q", opts.AppRoot)
	}
}
