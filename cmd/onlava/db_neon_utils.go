package main

import (
	"strings"

	appcfg "github.com/pbrazdil/onlava/internal/app"
)

func dockerHealthFromStatus(status string) string {
	for _, health := range []string{"healthy", "unhealthy"} {
		if strings.Contains(status, "("+health+")") {
			return health
		}
	}
	start := strings.Index(status, "(health: ")
	if start == -1 {
		return ""
	}
	rest := status[start+len("(health: "):]
	end := strings.Index(rest, ")")
	if end == -1 {
		return ""
	}
	return rest[:end]
}

func neonProjectForConfig(cfg appcfg.Config) string {
	project := sanitizeNeonBranchSegment(firstNonEmpty(neonPostgresService(cfg).Project, cfg.AppID(), "app"))
	if project == "" {
		return "app"
	}
	return project
}

func neonDatabaseURLEnv(cfg appcfg.Config) string {
	return firstNonEmpty(neonPostgresService(cfg).DatabaseURLEnv, appDatabaseURLEnv)
}
