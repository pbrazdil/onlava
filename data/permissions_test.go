package data

import (
	"context"
	"errors"
	"strings"
	"testing"

	onlavaauth "github.com/pbrazdil/onlava/auth"
)

func TestTenantKeyFromActorUsesStandardAuthData(t *testing.T) {
	tenantKey, ok := TenantKeyFromActor(Actor{
		Data: &onlavaauth.AuthData{TenantID: onlavaauth.TenantID("tenant-a")},
	})
	if !ok || tenantKey != "tenant-a" {
		t.Fatalf("TenantKeyFromActor = %q, %v; want tenant-a, true", tenantKey, ok)
	}
}

func TestTenantKeyFromActorPrefersExplicitTenantKey(t *testing.T) {
	tenantKey, ok := TenantKeyFromActor(Actor{
		TenantKey: "explicit",
		Data:      &onlavaauth.AuthData{TenantID: onlavaauth.TenantID("auth-tenant")},
	})
	if !ok || tenantKey != "explicit" {
		t.Fatalf("TenantKeyFromActor = %q, %v; want explicit, true", tenantKey, ok)
	}
}

func TestStandardAuthPermissionsDenyCrossTenant(t *testing.T) {
	perms := StandardAuthPermissions{}
	err := perms.CanReadObject(context.Background(), Actor{TenantKey: "tenant-a"}, ObjectRef{TenantKey: "tenant-b", Name: "company"})
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("CanReadObject error = %v, want permission denied", err)
	}
	if CodeOf(wrapError("CanReadObject", err)) != ErrorPermissionDenied {
		t.Fatalf("CodeOf = %q, want %q", CodeOf(wrapError("CanReadObject", err)), ErrorPermissionDenied)
	}
}

func TestStandardAuthPermissionsDelegateAfterTenantCheck(t *testing.T) {
	perms := StandardAuthPermissions{Base: denyRowFilterPermissions{}}
	_, err := perms.RowFilter(context.Background(), Actor{TenantKey: "tenant-a"}, ObjectRef{TenantKey: "tenant-a", Name: "company"})
	if err == nil || err.Error() != "row denied" {
		t.Fatalf("RowFilter error = %v, want delegated row denied", err)
	}
}

type denyRowFilterPermissions struct {
	AllowAllPermissions
}

func (denyRowFilterPermissions) RowFilter(context.Context, Actor, ObjectRef) (*Filter, error) {
	return nil, errors.New("row denied")
}
