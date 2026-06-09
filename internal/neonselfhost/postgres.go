package neonselfhost

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/pbrazdil/onlava/internal/envpolicy"
)

var postgresReadyTimeout = 30 * time.Second

func branchPostgresReady(ctx context.Context, branch BackendBranch) (bool, string, error) {
	if ready, message := recordedComputeReady(branch); !ready {
		return false, message, nil
	}
	if ok, message, err := ensurePostgresDatabase(ctx, branch); err != nil {
		return false, "", err
	} else if !ok {
		return false, message, nil
	}
	return true, fmt.Sprintf("neon-selfhost-driver verified Postgres endpoint for %q at %s:%d", branch.Branch, branch.Host, branch.Port), nil
}

func ensurePostgresDatabase(ctx context.Context, branch BackendBranch) (bool, string, error) {
	psql, err := exec.LookPath("psql")
	if err != nil {
		return false, fmt.Sprintf("neon-selfhost-driver found a TCP listener at %s:%d, but psql is not available to verify and prepare database %q: %v", branch.Host, branch.Port, branch.Database, err), nil
	}
	deadline := time.Now().Add(postgresReadyTimeout)
	var lastErr error
	for {
		if err := runPSQL(ctx, psql, branch, "postgres", "select 1"); err == nil {
			break
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			return false, fmt.Sprintf("neon-selfhost-driver could not verify Postgres readiness at %s:%d: %v", branch.Host, branch.Port, lastErr), nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	database := firstNonEmpty(branch.Database, "postgres")
	if database == "postgres" {
		return true, "", nil
	}
	existsSQL := fmt.Sprintf("select 1 from pg_database where datname = %s", pgQuoteLiteral(database))
	output, err := runPSQLOutput(ctx, psql, branch, "postgres", existsSQL)
	if err != nil {
		return false, "", err
	}
	if strings.TrimSpace(output) == "1" {
		return true, "", nil
	}
	createSQL := "create database " + pgQuoteIdentifier(database)
	if err := runPSQL(ctx, psql, branch, "postgres", createSQL); err != nil {
		return false, "", err
	}
	return true, "", nil
}

func runPSQL(ctx context.Context, psql string, branch BackendBranch, database string, sql string) error {
	_, err := runPSQLOutput(ctx, psql, branch, database, sql)
	return err
}

func runPSQLOutput(ctx context.Context, psql string, branch BackendBranch, database string, sql string) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	args := []string{
		"-h", branch.Host,
		"-p", fmt.Sprintf("%d", branch.Port),
		"-U", firstNonEmpty(branch.Role, "cloud_admin"),
		"-d", firstNonEmpty(database, "postgres"),
		"-v", "ON_ERROR_STOP=1",
		"-tAc", sql,
	}
	cmd := exec.CommandContext(cmdCtx, psql, args...)
	cmd.Env = append(envpolicy.Environ(), "PGPASSWORD=cloud_admin")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
			return "", errors.New("psql timed out")
		}
		return "", fmt.Errorf("psql: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func pgQuoteLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func pgQuoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
