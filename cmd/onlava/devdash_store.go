package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	localagent "github.com/pbrazdil/onlava/internal/agent"
	"github.com/pbrazdil/onlava/internal/devdash"
)

func openDevdashStore() (*devdash.Store, error) {
	return devdash.OpenStore(devdashCacheRoot())
}

func devdashCacheRoot() string {
	if root := strings.TrimSpace(os.Getenv("ONLAVA_DEV_CACHE_DIR")); root != "" {
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
