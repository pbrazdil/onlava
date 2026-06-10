package auth

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestClarifyStandardAuthTenantError(t *testing.T) {
	t.Parallel()

	original := &pgconn.PgError{
		SchemaName: "scenery_auth",
		TableName:  "tenants",
		Message:    `relation "scenery_auth"."tenants" does not exist`,
	}

	err := clarifyStandardAuthTenantError(original)
	if !errors.Is(err, original) {
		t.Fatalf("wrapped error does not preserve original: %v", err)
	}
	got := err.Error()
	for _, want := range []string{"standard auth owns framework tenant state", "scenery_auth.tenants", "not an app-local tenants service"} {
		if !strings.Contains(got, want) {
			t.Fatalf("error %q does not contain %q", got, want)
		}
	}
}

func TestClarifyAppDomainTenantError(t *testing.T) {
	t.Parallel()

	original := &pgconn.PgError{
		TableName: "tenants",
		Message:   `relation "tenants" does not exist`,
	}

	err := clarifyStandardAuthTenantError(original)
	if !errors.Is(err, original) {
		t.Fatalf("wrapped error does not preserve original: %v", err)
	}
	got := err.Error()
	for _, want := range []string{"app-domain tenants relation", "standard auth tenant state lives in scenery_auth.tenants", "does not require an app-local tenants service"} {
		if !strings.Contains(got, want) {
			t.Fatalf("error %q does not contain %q", got, want)
		}
	}
}

func TestClarifyStandardAuthTenantErrorIgnoresUnrelatedErrors(t *testing.T) {
	t.Parallel()

	original := errors.New("plain runtime error")
	if got := clarifyStandardAuthTenantError(original); got != original {
		t.Fatalf("error = %v, want original", got)
	}
}

func TestClarifyStandardAuthTenantErrorIgnoresNonTenantAuthSchemaErrors(t *testing.T) {
	t.Parallel()

	original := &pgconn.PgError{
		SchemaName: "scenery_auth",
		TableName:  "users",
		Message:    `relation "scenery_auth"."users" does not exist`,
	}
	if got := clarifyStandardAuthTenantError(original); got != original {
		t.Fatalf("error = %v, want original", got)
	}
}
