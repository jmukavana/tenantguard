// Package isolation enforces tenant data boundaries (core/spec.md, IsolationStrategy).
// SchemaStrategy implements schema-based isolation — the recommended default for
// regulated systems — by scoping each tenant's queries to its own Postgres schema via
// search_path. Row-Level and Database-per-Tenant strategies are follow-ups.
package isolation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"

	tenantcontext "github.com/tenantguard/tenantguard/context"
)

// ErrUnsafeTenantID is returned when a tenant id cannot be used as a schema identifier.
// A schema name is interpolated into a SET statement (SET does not accept placeholders),
// so ids are restricted to a strict allowlist to make SQL injection impossible.
var ErrUnsafeTenantID = errors.New("tenantguard: tenant id is not a safe schema identifier")

var safeIdent = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// Execer is the subset of *sql.DB, *sql.Tx and *sql.Conn needed to bind a schema.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// SchemaStrategy maps each tenant to the Postgres schema "Prefix+tenantID".
type SchemaStrategy struct {
	// Prefix is prepended to the tenant id to form the schema name, e.g. "tenant_".
	Prefix string
}

// SchemaName returns the validated, injection-safe schema name for tc, or
// ErrUnsafeTenantID if the tenant id contains anything outside [A-Za-z0-9_].
func (s SchemaStrategy) SchemaName(tc tenantcontext.TenantContext) (string, error) {
	id := tc.TenantID()
	if !safeIdent.MatchString(id) {
		return "", fmt.Errorf("%w: %q", ErrUnsafeTenantID, id)
	}
	return s.Prefix + id, nil
}

// Bind scopes subsequent queries on e to the tenant's schema by setting search_path.
// Call it at the start of each tenant-scoped transaction or connection; afterwards an
// unqualified `SELECT ... FROM widgets` resolves only within the tenant's schema.
func (s SchemaStrategy) Bind(ctx context.Context, e Execer, tc tenantcontext.TenantContext) error {
	schema, err := s.SchemaName(tc)
	if err != nil {
		return err
	}
	// schema matched safeIdent, so double-quoting it can carry no injection.
	if _, err := e.ExecContext(ctx, fmt.Sprintf("SET search_path TO %q", schema)); err != nil {
		return fmt.Errorf("tenantguard: bind schema %q: %w", schema, err)
	}
	return nil
}
