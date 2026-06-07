package main

import (
	"strings"
	"testing"
)

func TestCanonicalTopLevelUsage(t *testing.T) {
	usage := usageError().Error()
	for _, want := range []string{
		"onlava up ",
		"onlava ps ",
		"onlava logs ",
		"onlava console ",
		"onlava down ",
		"onlava prune ",
		"onlava serve ",
		"onlava worker ",
		"onlava build ",
		"onlava check ",
		"onlava test ",
		"onlava inspect ",
		"onlava generate ",
		"onlava db ",
		"onlava task ",
		"onlava traces ",
		"onlava metrics ",
		"onlava doctor ",
		"onlava version ",
		"onlava system ",
		"onlava harness ",
	} {
		if !strings.Contains(usage, want) {
			t.Fatalf("usage missing %q:\n%s", want, usage)
		}
	}
	for _, removed := range []string{
		"onlava dev ",
		"onlava attach ",
		"onlava status ",
		"onlava agent ",
		"onlava edge ",
		"onlava toolchain ",
		"onlava temporal ",
		"onlava admin ",
		"onlava psql ",
		"onlava gen ",
		"onlava run ",
		"onlava script ",
		"onlava db sync ",
		"onlava inspect traces ",
		"onlava inspect metrics ",
	} {
		if strings.Contains(usage, removed) {
			t.Fatalf("usage contains removed spelling %q:\n%s", removed, usage)
		}
	}
}

func TestRemovedTopLevelCommandsFail(t *testing.T) {
	for _, command := range []string{
		"dev",
		"attach",
		"status",
		"agent",
		"edge",
		"toolchain",
		"temporal",
		"admin",
		"psql",
		"gen",
		"run",
		"script",
	} {
		err := run([]string{command})
		if err == nil || err.Error() != `unknown command "`+command+`"` {
			t.Fatalf("run(%q) error = %v", command, err)
		}
	}
}

func TestCanonicalCommandParsers(t *testing.T) {
	if _, err := parseDevArgs([]string{"--app-root", "/tmp/app", "--detach"}); err != nil {
		t.Fatalf("parse up args: %v", err)
	}
	if _, err := parseStatusArgs([]string{"--json", "--app-root", "/tmp/app", "--session", "current"}); err != nil {
		t.Fatalf("parse ps args: %v", err)
	}
	if err := workerCommand([]string{"deployment"}); err == nil || !strings.Contains(err.Error(), "onlava worker deployment") {
		t.Fatalf("worker deployment usage error = %v", err)
	}
	if err := dbCommand([]string{"sync"}); err == nil || err.Error() != `unknown db command "sync"` {
		t.Fatalf("db sync error = %v", err)
	}
	if _, err := parseInspectArgs([]string{"traces", "--json"}); err == nil || !strings.Contains(err.Error(), "use `onlava traces list`") {
		t.Fatalf("inspect traces error = %v", err)
	}
	if _, err := parseInspectArgs([]string{"metrics", "--json"}); err == nil || !strings.Contains(err.Error(), "use `onlava metrics list`") {
		t.Fatalf("inspect metrics error = %v", err)
	}
}
