package store

import (
	"context"
	"testing"
	"time"

	"github.com/tenantguard/tenantguard/audit"
)

func event(tenantID, eventID string, at time.Time) audit.Event {
	e := audit.NewEvent("tenant.resolved", tenantID, "", audit.OutcomeAllowed)
	e.EventID = eventID
	e.Timestamp = at
	return e
}

func TestAppendIsIdempotentPerTenant(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	now := time.Now().UTC()

	stored, _ := m.Append(ctx, event("acme", "e1", now))
	if !stored {
		t.Fatal("first append should store")
	}
	stored, _ = m.Append(ctx, event("acme", "e1", now))
	if stored {
		t.Fatal("redelivering the same eventId must be a duplicate no-op")
	}

	// The same eventId under a different tenant is a distinct event.
	if stored, _ := m.Append(ctx, event("globex", "e1", now)); !stored {
		t.Fatal("same eventId for another tenant should store")
	}
}

func TestQueryIsolatesTenants(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	now := time.Now().UTC()
	_, _ = m.Append(ctx, event("acme", "a1", now))
	_, _ = m.Append(ctx, event("globex", "g1", now))

	acme, _ := m.Query(ctx, "acme", time.Time{}, time.Time{})
	if len(acme) != 1 || acme[0].TenantID != "acme" {
		t.Fatalf("acme query leaked or missed events: %+v", acme)
	}
}

func TestMemoryChainIsIntactAndAdvances(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	now := time.Now().UTC()

	empty, _ := m.VerifyChain(ctx, "acme")
	if !empty.Intact || empty.Length != 0 || empty.Head != "" {
		t.Fatalf("empty chain report = %+v, want intact len 0 head \"\"", empty)
	}

	_, _ = m.Append(ctx, event("acme", "e1", now))
	after1, _ := m.VerifyChain(ctx, "acme")
	_, _ = m.Append(ctx, event("acme", "e2", now))
	after2, _ := m.VerifyChain(ctx, "acme")

	if !after2.Intact || after2.Length != 2 {
		t.Fatalf("chain after two appends = %+v, want intact len 2", after2)
	}
	if after2.Head == after1.Head {
		t.Error("appending an event should advance the chain head")
	}
	// A duplicate must not extend the chain.
	_, _ = m.Append(ctx, event("acme", "e2", now))
	if again, _ := m.VerifyChain(ctx, "acme"); again.Length != 2 || again.Head != after2.Head {
		t.Errorf("duplicate changed the chain: %+v", again)
	}
}

func TestQueryFiltersByTimeBounds(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_, _ = m.Append(ctx, event("acme", "old", base))
	_, _ = m.Append(ctx, event("acme", "new", base.Add(48*time.Hour)))

	got, _ := m.Query(ctx, "acme", base.Add(24*time.Hour), time.Time{})
	if len(got) != 1 || got[0].EventID != "new" {
		t.Fatalf("time filter wrong: %+v", got)
	}
}
