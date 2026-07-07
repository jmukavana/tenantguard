package chain

import "testing"

// The canonical forms of the two auditforge-signing.json golden events. Anchoring the chain
// vectors to the same fixtures keeps hashing reproducible across languages.
const (
	minimalCanonical = `{"eventId":"e1","eventType":"tenant.resolved","tenantId":"acme","timestamp":"2026-01-01T00:00:00Z","outcome":"allowed"}`
	fullCanonical    = `{"eventId":"e2","eventType":"policy.denied","tenantId":"acme","timestamp":"2026-01-01T00:00:00Z","outcome":"denied","traceId":"4bf92f3577b34da6a3ce929d0e0e4736","actor":"user-7","resource":"/orders","metadata":{"reason":"tenant is frozen"}}`

	// Golden values, independently computed as SHA-256(prevHash || canonical).
	goldenHead   = "6fe386de2fd54a52521414dd205ff24248eb2509f058da62ae6191fd93590a38"
	goldenSecond = "8185c693ec6ebaca9fca7caa3ade97e5677847016894e46463dd7af437740755"
)

func TestNextMatchesGoldenVectors(t *testing.T) {
	head := Next(Genesis, []byte(minimalCanonical))
	if head != goldenHead {
		t.Fatalf("genesis hash = %s, want %s", head, goldenHead)
	}
	second := Next(head, []byte(fullCanonical))
	if second != goldenSecond {
		t.Fatalf("second hash = %s, want %s", second, goldenSecond)
	}
}

func TestNextIsDeterministic(t *testing.T) {
	a := Next("prev", []byte("canonical"))
	b := Next("prev", []byte("canonical"))
	if a != b {
		t.Fatal("Next must be deterministic")
	}
}

func TestNextDependsOnPrevHashAndContent(t *testing.T) {
	base := Next("prev", []byte("canonical"))
	if Next("other", []byte("canonical")) == base {
		t.Error("changing prevHash must change the hash")
	}
	if Next("prev", []byte("tampered")) == base {
		t.Error("changing the canonical form must change the hash")
	}
}
