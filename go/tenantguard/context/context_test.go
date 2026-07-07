package tenantcontext

import (
	"errors"
	"testing"
)

func TestNew_RequiresTenantID(t *testing.T) {
	if _, err := New("", "Acme", "", nil); !errors.Is(err, ErrEmptyTenantID) {
		t.Fatalf("empty tenant id: got err %v, want ErrEmptyTenantID", err)
	}

	ctx, err := New("acme", "Acme", "", nil)
	if err != nil {
		t.Fatalf("valid tenant id: unexpected err %v", err)
	}
	if !ctx.IsValid() {
		t.Fatal("constructed context should be valid")
	}
	if got := ctx.TenantID(); got != "acme" {
		t.Fatalf("TenantID() = %q, want %q", got, "acme")
	}
}

func TestNew_RejectsMalformedTraceID(t *testing.T) {
	if _, err := New("acme", "Acme", "not-a-trace", nil); !errors.Is(err, ErrInvalidTraceID) {
		t.Fatalf("bad trace id: got err %v, want ErrInvalidTraceID", err)
	}
	valid := "4bf92f3577b34da6a3ce929d0e0e4736"
	ctx, err := New("acme", "Acme", valid, nil)
	if err != nil {
		t.Fatalf("valid trace id: unexpected err %v", err)
	}
	if ctx.TraceID() != valid {
		t.Fatalf("TraceID() = %q, want %q", ctx.TraceID(), valid)
	}
}

func TestZeroValueIsInvalid(t *testing.T) {
	var zero TenantContext
	if zero.IsValid() {
		t.Fatal("zero TenantContext must not be valid")
	}
}

// TestImmutable is the security-critical guard: a TenantContext must not change after
// construction. Mutating the caller's input map or the map returned by Metadata() must
// not alter the context.
func TestImmutable(t *testing.T) {
	input := map[string]string{"region": "ke", "tier": "gold"}
	ctx, err := New("acme", "Acme", "", input)
	if err != nil {
		t.Fatalf("unexpected err %v", err)
	}

	// Mutating the source map must not leak into the context.
	input["region"] = "tz"
	input["injected"] = "evil"
	if got := ctx.Metadata()["region"]; got != "ke" {
		t.Fatalf("context leaked source-map mutation: region = %q, want %q", got, "ke")
	}
	if _, ok := ctx.Metadata()["injected"]; ok {
		t.Fatal("context absorbed a key added to the source map after construction")
	}

	// Mutating the returned map must not affect the context's internal state.
	out := ctx.Metadata()
	out["region"] = "us"
	delete(out, "tier")
	if got := ctx.Metadata()["region"]; got != "ke" {
		t.Fatalf("context leaked returned-map mutation: region = %q, want %q", got, "ke")
	}
	if _, ok := ctx.Metadata()["tier"]; !ok {
		t.Fatal("context lost a key deleted from a returned map")
	}
}
