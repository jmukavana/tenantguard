package audit

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestNewEvent(t *testing.T) {
	before := time.Now().UTC()
	e := NewEvent("tenant.resolved", "acme", "4bf92f3577b34da6a3ce929d0e0e4736", OutcomeAllowed)

	if e.EventID == "" {
		t.Fatal("EventID should be populated")
	}
	if e.EventType != "tenant.resolved" || e.TenantID != "acme" || e.Outcome != OutcomeAllowed {
		t.Fatalf("unexpected event fields: %+v", e)
	}
	if e.Timestamp.Before(before) {
		t.Fatalf("timestamp %v predates event creation %v", e.Timestamp, before)
	}

	// Event ids must be unique per event (idempotency key for AuditForge).
	if NewEvent("x", "acme", "", OutcomeAllowed).EventID == e.EventID {
		t.Fatal("EventID should differ between events")
	}
}

func TestEventJSONKeys(t *testing.T) {
	e := NewEvent("db.access", "acme", "", OutcomeDenied)
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, k := range []string{"eventId", "eventType", "tenantId", "timestamp", "outcome"} {
		if _, ok := m[k]; !ok {
			t.Errorf("required key %q missing from JSON: %s", k, b)
		}
	}
	// Empty optional fields must be omitted, matching the schema's optional properties.
	if _, ok := m["traceId"]; ok {
		t.Errorf("empty traceId should be omitted: %s", b)
	}
}

func TestSignIgnoresExistingSignature(t *testing.T) {
	e := NewEvent("tenant.resolved", "acme", "", OutcomeAllowed)
	secret := []byte("s3cret")

	unsigned, err := Sign(e, secret)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	// A stale signature on the event must not feed back into the canonical form.
	e.Signature = "stale-signature"
	resigned, err := Sign(e, secret)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if resigned != unsigned {
		t.Errorf("signature depends on the Signature field: %s != %s", resigned, unsigned)
	}
}

func TestVerifyDetectsTampering(t *testing.T) {
	e := NewEvent("db.access", "acme", "", OutcomeAllowed)
	secret := []byte("s3cret")

	sig, err := Sign(e, secret)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if ok, _ := Verify(e, secret, sig); !ok {
		t.Fatal("Verify rejected a valid signature")
	}

	tampered := e
	tampered.TenantID = "evil"
	if ok, _ := Verify(tampered, secret, sig); ok {
		t.Error("Verify accepted a signature after the tenant id was changed")
	}
}

func TestLoggingHookPublish(t *testing.T) {
	if err := (LoggingHook{}).Publish(context.Background(), NewEvent("x", "acme", "", OutcomeAllowed)); err != nil {
		t.Fatalf("LoggingHook.Publish returned error: %v", err)
	}
}
