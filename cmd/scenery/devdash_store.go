package main

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	localagent "scenery.sh/internal/agent"
	"scenery.sh/internal/devdash"
	"scenery.sh/internal/envpolicy"
)

func openDevdashStore() (*devdash.Store, error) {
	return devdash.OpenStore(devdashCacheRoot())
}

func devdashCacheRoot() string {
	if root := strings.TrimSpace(envpolicy.Get("SCENERY_DEV_CACHE_DIR")); root != "" {
		return root
	}
	if localagent.DisabledByEnv() {
		return ""
	}
	client, err := localagent.DefaultClient()
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		return ""
	}
	paths, err := localagent.DefaultPaths()
	if err != nil {
		return ""
	}
	return filepath.Join(paths.AgentDir, "dashboard")
}
