package database

import (
	"context"
	"errors"
	"testing"

	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
)

// A context without a tenant (and not marked as system work) is refused
// before the connection is touched: the hook never reaches the nil conn.
func TestTenantHookFailsClosed(t *testing.T) {
	prepare, _ := TenantHooks()
	ok, err := prepare(context.Background(), nil)
	if !errors.Is(err, tenant.ErrNoTenant) {
		t.Fatalf("err = %v, want ErrNoTenant", err)
	}
	if !ok {
		t.Fatal("an untouched connection must go back to the pool, not be destroyed")
	}
}
