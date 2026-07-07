// Package store is AuditForge's append-only, tenant-isolated event log. The Store interface
// is the seam behind which a Postgres, schema-isolated backend (using TenantGuard's
// SchemaStrategy) will sit; Memory is the in-memory implementation for tests and local dev.
package store

import (
	"context"
	"time"

	"github.com/tenantguard/tenantguard/audit"

	"github.com/tenantguard/auditforge/chain"
)

// Store persists verified audit events and answers tenant-scoped queries. Implementations
// must isolate tenants: an event stored for tenant A is never visible to a query for B.
// Each newly stored event extends the tenant's tamper-evident hash chain (see chain).
type Store interface {
	// Append stores e for its tenant, keyed by eventId for idempotency. It reports whether
	// the event was newly stored; false means an event with that id already existed.
	Append(ctx context.Context, e audit.Event) (stored bool, err error)
	// Query returns tenantID's events whose timestamp falls within [from, to], oldest first.
	// A zero from or to is unbounded on that side.
	Query(ctx context.Context, tenantID string, from, to time.Time) ([]audit.Event, error)
	// VerifyChain replays the tenant's hash chain (from its retention checkpoint, if any)
	// and reports whether it is intact.
	VerifyChain(ctx context.Context, tenantID string) (ChainReport, error)
	// Purge removes the leading run of the tenant's records at or older than olderThan and
	// records a signed checkpoint so the surviving tail still verifies. Records newer than
	// olderThan are never removed, so a retention window is honored.
	Purge(ctx context.Context, tenantID string, olderThan time.Time) (PurgeResult, error)
}

// ChainReport is the outcome of replaying a tenant's tamper-evident chain.
type ChainReport struct {
	// Length is the total number of records the chain has held, including any purged prefix.
	Length int `json:"length"`
	// Head is the hash of the last record, the checkpoint hash for a fully purged chain, or
	// chain.Genesis for an empty chain.
	Head string `json:"head"`
	// Intact is true when the checkpoint (if any) verifies and every surviving record's
	// prevHash and recomputed hash line up.
	Intact bool `json:"intact"`
	// Purged is the number of records represented by the retention checkpoint.
	Purged int `json:"purged"`
	// BrokenAt is the eventId of the first inconsistent record when Intact is false, or
	// "checkpoint" when the retention checkpoint itself fails to verify.
	BrokenAt string `json:"brokenAt,omitempty"`
}

// PurgeResult reports the outcome of a retention purge.
type PurgeResult struct {
	// Purged is the number of records removed by this call.
	Purged int `json:"purged"`
	// Checkpoint is the signed checkpoint now in effect (zero-valued if none exists).
	Checkpoint chain.Checkpoint `json:"checkpoint"`
}
