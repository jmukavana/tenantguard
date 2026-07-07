package ingest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tenantguard/tenantguard/audit"

	"github.com/tenantguard/auditforge/keyring"
	"github.com/tenantguard/auditforge/store"
)

func agedEvent(eventID string, ts time.Time) audit.Event {
	e := audit.NewEvent("tenant.resolved", "acme", "", audit.OutcomeAllowed)
	e.EventID = eventID
	e.Timestamp = ts
	return e
}

func TestServicePurgeDisabledByDefault(t *testing.T) {
	svc := Service{Keys: keyring.Map{}, Store: store.NewMemory(checkpointSecret)}
	if _, err := svc.Purge(context.Background(), "acme"); !errors.Is(err, ErrRetentionDisabled) {
		t.Fatalf("want ErrRetentionDisabled, got %v", err)
	}
}

func TestServicePurgeEnforcesWindow(t *testing.T) {
	st := store.NewMemory(checkpointSecret)
	svc := Service{Keys: keyring.Map{}, Store: st, Retention: time.Hour}
	ctx := context.Background()

	// One event older than the window, one within it.
	st.Append(ctx, agedEvent("old", time.Now().Add(-2*time.Hour)))
	st.Append(ctx, agedEvent("recent", time.Now()))

	res, err := svc.Purge(ctx, "acme")
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if res.Purged != 1 {
		t.Fatalf("purged %d, want 1 (only the event older than the 1h window)", res.Purged)
	}

	got, _ := st.Query(ctx, "acme", time.Time{}, time.Time{})
	if len(got) != 1 || got[0].EventID != "recent" {
		t.Fatalf("query after purge = %+v, want [recent]", got)
	}
}
