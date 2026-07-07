package propagation

import (
	"context"
	"net/http"
	"testing"

	"go.opentelemetry.io/otel/baggage"
	otelprop "go.opentelemetry.io/otel/propagation"

	tenantcontext "github.com/tenantguard/tenantguard/context"
)

func newTenant(t *testing.T, id, name, trace string, md map[string]string) tenantcontext.TenantContext {
	t.Helper()
	tc, err := tenantcontext.New(id, name, trace, md)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return tc
}

func TestRoundTrip(t *testing.T) {
	p := New()
	want := newTenant(t, "acme", "Acme Ltd", "4bf92f3577b34da6a3ce929d0e0e4736",
		map[string]string{"region": "ke", "tier": "gold"})

	carrier := otelprop.MapCarrier{}
	if err := p.Inject(context.Background(), want, carrier); err != nil {
		t.Fatalf("Inject: %v", err)
	}

	got, ok, err := p.Extract(carrier)
	if err != nil || !ok {
		t.Fatalf("Extract: ok=%v err=%v", ok, err)
	}
	if got.TenantID() != "acme" || got.TenantName() != "Acme Ltd" || got.TraceID() != want.TraceID() {
		t.Fatalf("scalar mismatch: %+v", got)
	}
	md := got.Metadata()
	if md["region"] != "ke" || md["tier"] != "gold" || len(md) != 2 {
		t.Fatalf("metadata mismatch: %v", md)
	}
}

func TestExtract_NoTenant(t *testing.T) {
	_, ok, err := New().Extract(otelprop.MapCarrier{})
	if ok || err != nil {
		t.Fatalf("empty carrier: ok=%v err=%v, want false,nil", ok, err)
	}
}

func TestInject_InvalidContext(t *testing.T) {
	var zero tenantcontext.TenantContext
	if err := New().Inject(context.Background(), zero, otelprop.MapCarrier{}); err != ErrInvalidContext {
		t.Fatalf("zero context: got %v, want ErrInvalidContext", err)
	}
}

func TestRoundTrip_HTTPHeaders(t *testing.T) {
	p := New()
	want := newTenant(t, "beta", "", "", nil)

	h := http.Header{}
	if err := p.Inject(context.Background(), want, otelprop.HeaderCarrier(h)); err != nil {
		t.Fatalf("Inject: %v", err)
	}
	if h.Get("baggage") == "" {
		t.Fatal("expected a W3C baggage header to be written")
	}

	got, ok, err := p.Extract(otelprop.HeaderCarrier(h))
	if err != nil || !ok || got.TenantID() != "beta" {
		t.Fatalf("extract from headers: tc=%q ok=%v err=%v", got.TenantID(), ok, err)
	}
}

// TestInject_PreservesExistingBaggage ensures we add to, not replace, non-tenant baggage
// that an upstream hop already set.
func TestInject_PreservesExistingBaggage(t *testing.T) {
	member, err := baggage.NewMemberRaw("app.feature", "on")
	if err != nil {
		t.Fatalf("member: %v", err)
	}
	bag, err := baggage.New(member)
	if err != nil {
		t.Fatalf("baggage: %v", err)
	}
	ctx := baggage.ContextWithBaggage(context.Background(), bag)

	carrier := otelprop.MapCarrier{}
	if err := New().Inject(ctx, newTenant(t, "acme", "", "", nil), carrier); err != nil {
		t.Fatalf("Inject: %v", err)
	}

	// Re-read the baggage from the carrier and confirm both members survived.
	out := baggage.FromContext((otelprop.Baggage{}).Extract(context.Background(), carrier))
	if out.Member("app.feature").Value() != "on" {
		t.Fatalf("pre-existing baggage lost: %q", out.String())
	}
	if out.Member(memberTenantID).Value() != "acme" {
		t.Fatalf("tenant baggage missing: %q", out.String())
	}
}
