package testpostgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/jackc/pgx/v5/pgxpool"
)

const EnvDatabaseURL = "ONLAVA_TEST_DATABASE_URL"

type Database struct {
	URL       string
	container *postgres.PostgresContainer
	reusable  bool
}

func Start(ctx context.Context) (*Database, error) {
	if dsn := strings.TrimSpace(os.Getenv(EnvDatabaseURL)); dsn != "" {
		return &Database{URL: dsn}, nil
	}
	if adminURL, ok := readCachedReusablePostgresURL(); ok {
		url, err := ensurePackageDatabase(ctx, adminURL)
		if err == nil {
			return &Database{URL: url, reusable: true}, nil
		}
		_ = os.Remove(reusablePostgresURLCachePath())
	}
	unlock, err := lockReusablePostgres(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if adminURL, ok := readCachedReusablePostgresURL(); ok {
		url, err := ensurePackageDatabase(ctx, adminURL)
		if err == nil {
			return &Database{URL: url, reusable: true}, nil
		}
		_ = os.Remove(reusablePostgresURLCachePath())
	}
	if err := os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true"); err != nil {
		return nil, err
	}
	container, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("onlava_test"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithReuseByName(reusablePostgresContainerName()),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(90*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("start PostgreSQL testcontainer: %w", err)
	}
	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("read PostgreSQL testcontainer connection string: %w", err)
	}
	if err := writeCachedReusablePostgresURL(url); err != nil {
		_ = container.Terminate(ctx)
		return nil, err
	}
	url, err = ensurePackageDatabase(ctx, url)
	if err != nil {
		return nil, err
	}
	return &Database{URL: url, container: container, reusable: true}, nil
}

func (db *Database) Terminate(ctx context.Context) error {
	if db == nil || db.container == nil || db.reusable {
		return nil
	}
	return db.container.Terminate(ctx)
}

func reusablePostgresContainerName() string {
	sum := sha256.Sum256([]byte(repoRootForContainerName()))
	return "onlava-test-postgres-" + hex.EncodeToString(sum[:6])
}

func reusablePostgresCacheDir() (string, error) {
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheRoot, "onlava", "test-postgres"), nil
}

func reusablePostgresURLCachePath() string {
	dir, err := reusablePostgresCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, reusablePostgresContainerName()+".url")
}

func readCachedReusablePostgresURL() (string, bool) {
	path := reusablePostgresURLCachePath()
	if path == "" {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	adminURL := strings.TrimSpace(string(data))
	if adminURL == "" {
		return "", false
	}
	return adminURL, true
}

func writeCachedReusablePostgresURL(adminURL string) error {
	dir, err := reusablePostgresCacheDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, reusablePostgresContainerName()+".url")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(adminURL+"\n"), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func repoRootForContainerName() string {
	wd, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	for {
		data, err := os.ReadFile(filepath.Join(wd, "go.mod"))
		if err == nil && strings.Contains(string(data), "module github.com/pbrazdil/onlava") {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return wd
		}
		wd = parent
	}
}

func lockReusablePostgres(ctx context.Context) (func(), error) {
	cacheRoot, err := reusablePostgresCacheDir()
	if err != nil {
		return nil, err
	}
	lockDir := filepath.Join(cacheRoot, reusablePostgresContainerName()+".lock")
	if err := os.MkdirAll(filepath.Dir(lockDir), 0o755); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(2 * time.Minute)
	for {
		err := os.Mkdir(lockDir, 0o755)
		if err == nil {
			return func() { _ = os.Remove(lockDir) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		if info, statErr := os.Stat(lockDir); statErr == nil && time.Since(info.ModTime()) > 2*time.Minute {
			_ = os.Remove(lockDir)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for reusable PostgreSQL test lock %s", lockDir)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func ensurePackageDatabase(ctx context.Context, adminURL string) (string, error) {
	dbName := packageDatabaseName()
	pool, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		return "", fmt.Errorf("connect PostgreSQL admin database: %w", err)
	}
	defer pool.Close()
	var exists bool
	if err := pool.QueryRow(ctx, `select exists (select 1 from pg_database where datname = $1)`, dbName).Scan(&exists); err != nil {
		return "", fmt.Errorf("inspect PostgreSQL package database: %w", err)
	}
	if !exists {
		if _, err := pool.Exec(ctx, `create database `+quoteIdent(dbName)); err != nil {
			if inspectErr := pool.QueryRow(ctx, `select exists (select 1 from pg_database where datname = $1)`, dbName).Scan(&exists); inspectErr != nil {
				return "", fmt.Errorf("create PostgreSQL package database %s: %w", dbName, err)
			}
			if !exists {
				return "", fmt.Errorf("create PostgreSQL package database %s: %w", dbName, err)
			}
		}
	}
	parsed, err := url.Parse(adminURL)
	if err != nil {
		return "", err
	}
	parsed.Path = "/" + dbName
	query := parsed.Query()
	if query.Get("pool_max_conns") == "" {
		query.Set("pool_max_conns", "4")
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func packageDatabaseName() string {
	wd, err := os.Getwd()
	if err != nil {
		wd = "unknown"
	}
	root := repoRootForContainerName()
	rel, err := filepath.Rel(root, wd)
	if err != nil || strings.HasPrefix(rel, "..") {
		rel = wd
	}
	sum := sha256.Sum256([]byte(rel))
	return "onlava_test_" + hex.EncodeToString(sum[:6])
}

func quoteIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
