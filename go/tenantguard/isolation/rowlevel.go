package isolation

import (
	"context"
	"fmt"

	tenantcontext "github.com/tenantguard/tenantguard/context"
)

// DefaultRLSSetting is the session parameter RowLevelStrategy sets by default. Postgres
// requires custom parameters to be namespaced (contain a dot).
const DefaultRLSSetting = "app.tenant_id"

// RowLevelStrategy implements Row-Level Security isolation: instead of separate schemas,
// all tenants share tables carrying a tenant_id column, and Postgres RLS policies filter
// rows using a session parameter. Bind sets that parameter for the current transaction.
//
// The tenant id is passed as a bound parameter to set_config (not string-interpolated),
// so it is injection-safe for any tenant id value.
type RowLevelStrategy struct {
	// Setting is the session parameter RLS policies read via current_setting.
	// Empty means DefaultRLSSetting.
	Setting string
}

// setting returns the effective session parameter name.
func (s RowLevelStrategy) setting() string {
	if s.Setting == "" {
		return DefaultRLSSetting
	}
	return s.Setting
}

// Bind sets the tenant session parameter for the current transaction (is_local = true),
// so RLS policies of the form `USING (tenant_id = current_setting('app.tenant_id', true))`
// scope every subsequent statement to tc's tenant. Run it inside the tenant transaction.
func (s RowLevelStrategy) Bind(ctx context.Context, e Execer, tc tenantcontext.TenantContext) error {
	if !tc.IsValid() {
		return fmt.Errorf("%w: empty tenant id", ErrUnsafeTenantID)
	}
	if _, err := e.ExecContext(ctx, "SELECT set_config($1, $2, true)", s.setting(), tc.TenantID()); err != nil {
		return fmt.Errorf("tenantguard: set %s: %w", s.setting(), err)
	}
	return nil
}
