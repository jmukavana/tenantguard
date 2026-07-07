// Package store is AuditForge's append-only, tenant-isolated event log. The Store interface
// is the seam behind which a Postgres, schema-isolated backend (using TenantGuard's
// SchemaStrategy) will sit; Memory is the in-memory implementation for tests and local dev.
package store

import (
	"context"
	"time"

	"github.com/tenantguard/tenantguard/audit"
)

// Store persists verified audit events and answers tenant-scoped queries. Implementations
// must isolate tenants: an event stored for tenant A is never visible to a query for B.
type Store interface {
	// Append stores e for its tenant, keyed by eventId for idempotency. It reports whether
	// the event was newly stored; false means an event with that id already existed.
	Append(ctx context.Context, e audit.Event) (stored bool, err error)
	// Query returns tenantID's events whose timestamp falls within [from, to], oldest first.
	// A zero from or to is unbounded on that side.
	Query(ctx context.Context, tenantID string, from, to time.Time) ([]audit.Event, error)
}
