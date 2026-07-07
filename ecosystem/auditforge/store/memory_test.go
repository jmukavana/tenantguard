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
