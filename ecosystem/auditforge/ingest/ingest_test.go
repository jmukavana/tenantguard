package ingest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tenantguard/tenantguard/audit"

	"github.com/tenantguard/auditforge/keyring"
	"github.com/tenantguard/auditforge/store"
)

// goldenEvents loads the shared signing golden vectors and returns the secret plus the
// events carrying their real signatures. Verifying these proves AuditForge recomputes the
// canonical form byte-identically to the emitters.
func goldenEvents(t *testing.T) ([]byte, []audit.Event) {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testkit", "compliance-tests", "fixtures", "auditforge-signing.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture struct {
		Secret string `json:"secret"`
		Cases  []struct {
			Event     audit.Event `json:"event"`
			Signature string      `json:"signature"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(b, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	events := make([]audit.Event, 0, len(fixture.Cases))
	for _, c := range fixture.Cases {
		e := c.Event
		e.Signature = c.Signature
		events = append(events, e)
	}
	return []byte(fixture.Secret), events
}

func service(secret []byte) Service {
	return Service{Keys: keyring.Map{"k1": secret}, Store: store.NewMemory()}
}

func TestIngestAcceptsGoldenVectors(t *testing.T) {
	secret, events := goldenEvents(t)
	svc := service(secret)

	ack, err := svc.Ingest(context.Background(), IngestRequest{KeyID: "k1", Events: events})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if len(ack.Accepted) != len(events) || len(ack.Rejected) != 0 || len(ack.Duplicate) != 0 {
		t.Fatalf("expected every golden vector accepted, got %+v", ack)
	}
}

func TestIngestIsIdempotentOnRedelivery(t *testing.T) {
	secret, events := goldenEvents(t)
	svc := service(secret)
	req := IngestRequest{KeyID: "k1", Events: events}

	if _, err := svc.Ingest(context.Background(), req); err != nil {
		t.Fatalf("first Ingest: %v", err)
	}
	ack, err := svc.Ingest(context.Background(), req)
	if err != nil {
		t.Fatalf("second Ingest: %v", err)
	}
	if len(ack.Duplicate) != len(events) || len(ack.Accepted) != 0 {
		t.Fatalf("redelivery should be all duplicate, got %+v", ack)
	}
}

func TestIngestRejectsTamperedEvent(t *testing.T) {
	secret, events := goldenEvents(t)
	svc := service(secret)

	tampered := events[0]
	tampered.TenantID = "attacker" // invalidates the signature

	ack, err := svc.Ingest(context.Background(), IngestRequest{KeyID: "k1", Events: []audit.Event{tampered}})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if len(ack.Rejected) != 1 || ack.Rejected[0].Reason != reasonInvalidSignature {
		t.Fatalf("tampered event should be rejected invalid_signature, got %+v", ack)
	}
	if stored, _ := svc.Store.Query(context.Background(), "attacker", time.Time{}, time.Time{}); len(stored) != 0 {
		t.Fatal("a tampered event must never be stored")
	}
}

func TestIngestRejectsUnknownKey(t *testing.T) {
	_, events := goldenEvents(t)
	svc := Service{Keys: keyring.Map{}, Store: store.NewMemory()}

	ack, err := svc.Ingest(context.Background(), IngestRequest{KeyID: "unregistered", Events: events})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if len(ack.Rejected) != len(events) {
		t.Fatalf("all events under an unknown key should be rejected, got %+v", ack)
	}
	for _, r := range ack.Rejected {
		if r.Reason != reasonUnknownKey {
			t.Errorf("reason = %q, want %q", r.Reason, reasonUnknownKey)
		}
	}
}

func TestIngestRejectsMalformedEvent(t *testing.T) {
	secret, _ := goldenEvents(t)
	svc := service(secret)

	malformed := audit.NewEvent("tenant.resolved", "", "", audit.OutcomeAllowed) // empty tenantId
	malformed.EventID = "m1"

	ack, err := svc.Ingest(context.Background(), IngestRequest{KeyID: "k1", Events: []audit.Event{malformed}})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if len(ack.Rejected) != 1 || ack.Rejected[0].Reason != reasonMalformedEvent {
		t.Fatalf("empty tenantId should be malformed_event, got %+v", ack)
	}
}
