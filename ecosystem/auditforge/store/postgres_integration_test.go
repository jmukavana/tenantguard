package store_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/tenantguard/tenantguard/audit"

	"github.com/tenantguard/auditforge/store"
)

// TestPostgresStore_IdempotencyAndIsolation is the Phase C gate: against a real Postgres it
// proves an event is stored once (idempotent on redelivery) and that one tenant's events are
// unreachable from another tenant's query — isolation enforced at the database via schema
// binding. Skipped unless TENANTGUARD_POSTGRES_DSN is set, e.g.
//
//	TENANTGUARD_POSTGRES_DSN='postgres://postgres:tg@localhost:55432/tenantguard?sslmode=disable' \
//	    go test ./store/ -run Postgres -v
func TestPostgresStore_IdempotencyAndIsolation(t *testing.T) {
	dsn := os.Getenv("TENANTGUARD_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set TENANTGUARD_POSTGRES_DSN to run the Postgres store test")
	}

	ctx := context.Background()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	// The store provisions schemas lazily; drop any leftovers first and clean up after.
	// Registered after `defer db.Close()`, so LIFO runs the drops before the close.
	dropSchemas(t, db)
	defer dropSchemas(t, db)

	pg := store.NewPostgres(db, "tenant_")
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// First append stores; redelivering the same eventId is an idempotent no-op.
	acme := event("acme", "a1", "tenant.resolved", base)
	if stored, err := pg.Append(ctx, acme); err != nil || !stored {
		t.Fatalf("first append: stored=%v err=%v, want stored=true", stored, err)
	}
	if stored, err := pg.Append(ctx, acme); err != nil || stored {
		t.Fatalf("redelivery: stored=%v err=%v, want stored=false", stored, err)
	}

	// A second acme event (later) and one for a different tenant.
	if _, err := pg.Append(ctx, event("acme", "a2", "policy.denied", base.Add(48*time.Hour))); err != nil {
		t.Fatalf("append a2: %v", err)
	}
	if _, err := pg.Append(ctx, event("globex", "g1", "tenant.resolved", base)); err != nil {
		t.Fatalf("append g1: %v", err)
	}

	// Isolation: acme's query returns only acme's events, never globex's.
	acmeEvents, err := pg.Query(ctx, "acme", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("query acme: %v", err)
	}
	if len(acmeEvents) != 2 {
		t.Fatalf("acme has %d events, want 2", len(acmeEvents))
	}
	for _, e := range acmeEvents {
		if e.TenantID != "acme" {
			t.Fatalf("TENANT ISOLATION BREACH: acme query returned %s's event", e.TenantID)
		}
	}

	globexEvents, err := pg.Query(ctx, "globex", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("query globex: %v", err)
	}
	if len(globexEvents) != 1 || globexEvents[0].EventID != "g1" {
		t.Fatalf("globex query = %+v, want just g1", globexEvents)
	}

	// Time bounds: only the later acme event falls after base+24h.
	recent, err := pg.Query(ctx, "acme", base.Add(24*time.Hour), time.Time{})
	if err != nil {
		t.Fatalf("query acme with lower bound: %v", err)
	}
	if len(recent) != 1 || recent[0].EventID != "a2" {
		t.Fatalf("time-bounded query = %+v, want just a2", recent)
	}
}

// TestPostgresStore_ChainTamperEvidence proves the hash chain detects an edit to a stored
// record: after appending a chain, verification is intact; mutating a row's event JSON
// without updating its hash makes verification fail at that record. Same skip rule.
func TestPostgresStore_ChainTamperEvidence(t *testing.T) {
	dsn := os.Getenv("TENANTGUARD_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set TENANTGUARD_POSTGRES_DSN to run the Postgres chain test")
	}

	ctx := context.Background()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	dropSchemas(t, db)
	defer dropSchemas(t, db)

	pg := store.NewPostgres(db, "tenant_")
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := pg.Append(ctx, event("acme", "a1", "tenant.resolved", base)); err != nil {
		t.Fatalf("append a1: %v", err)
	}
	if _, err := pg.Append(ctx, event("acme", "a2", "policy.denied", base.Add(time.Hour))); err != nil {
		t.Fatalf("append a2: %v", err)
	}

	intact, err := pg.VerifyChain(ctx, "acme")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !intact.Intact || intact.Length != 2 {
		t.Fatalf("fresh chain = %+v, want intact len 2", intact)
	}

	// Tamper: edit the stored event body of a1 but leave its hash untouched.
	if _, err := db.ExecContext(ctx,
		`UPDATE tenant_acme.events SET event = jsonb_set(event, '{actor}', '"forged"') WHERE event_id = 'a1'`); err != nil {
		t.Fatalf("tamper: %v", err)
	}

	broken, err := pg.VerifyChain(ctx, "acme")
	if err != nil {
		t.Fatalf("verify after tamper: %v", err)
	}
	if broken.Intact || broken.BrokenAt != "a1" {
		t.Fatalf("tampered chain = %+v, want broken at a1", broken)
	}
}

func event(tenantID, eventID, eventType string, at time.Time) audit.Event {
	e := audit.NewEvent(eventType, tenantID, "", audit.OutcomeAllowed)
	e.EventID = eventID
	e.Timestamp = at
	return e
}

func dropSchemas(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, schema := range []string{"tenant_acme", "tenant_globex"} {
		if _, err := db.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); err != nil {
			t.Fatalf("drop schema %s: %v", schema, err)
		}
	}
}
