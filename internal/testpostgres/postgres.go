package testpostgres

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const EnvDatabaseURL = "ONLAVA_TEST_DATABASE_URL"

type Database struct {
	URL       string
	container *postgres.PostgresContainer
}

func Start(ctx context.Context) (*Database, error) {
	if dsn := strings.TrimSpace(os.Getenv(EnvDatabaseURL)); dsn != "" {
		return &Database{URL: dsn}, nil
	}
	container, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("onlava_test"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
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
	return &Database{URL: url, container: container}, nil
}

func (db *Database) Terminate(ctx context.Context) error {
	if db == nil || db.container == nil {
		return nil
	}
	return db.container.Terminate(ctx)
}
