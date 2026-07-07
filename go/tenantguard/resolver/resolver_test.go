package resolver

import (
	"errors"
	"net/http/httptest"
	"testing"
)

func TestHeaderResolver_Default(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set(DefaultHeader, "acme")

	tc, err := HeaderResolver{}.Resolve(r)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if tc.TenantID() != "acme" {
		t.Fatalf("TenantID() = %q, want acme", tc.TenantID())
	}
}

func TestHeaderResolver_TrimsAndRejectsMissing(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set(DefaultHeader, "   acme  ")
	if tc, _ := (HeaderResolver{}).Resolve(r); tc.TenantID() != "acme" {
		t.Fatalf("expected trimmed id, got %q", tc.TenantID())
	}

	empty := httptest.NewRequest("GET", "/", nil)
	if _, err := (HeaderResolver{}).Resolve(empty); !errors.Is(err, ErrNoTenant) {
		t.Fatalf("missing header: got %v, want ErrNoTenant", err)
	}

	blank := httptest.NewRequest("GET", "/", nil)
	blank.Header.Set(DefaultHeader, "   ")
	if _, err := (HeaderResolver{}).Resolve(blank); !errors.Is(err, ErrNoTenant) {
		t.Fatalf("whitespace header: got %v, want ErrNoTenant", err)
	}
}

func TestHeaderResolver_CustomHeader(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Org", "beta")
	tc, err := HeaderResolver{Header: "X-Org"}.Resolve(r)
	if err != nil || tc.TenantID() != "beta" {
		t.Fatalf("custom header: tc=%q err=%v", tc.TenantID(), err)
	}
}

func TestHeaderResolver_TraceParent(t *testing.T) {
	const traceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set(DefaultHeader, "acme")
	r.Header.Set("traceparent", "00-"+traceID+"-00f067aa0ba902b7-01")
	tc, err := HeaderResolver{}.Resolve(r)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if tc.TraceID() != traceID {
		t.Fatalf("TraceID() = %q, want %q", tc.TraceID(), traceID)
	}

	// A malformed traceparent must not fail an otherwise valid request.
	bad := httptest.NewRequest("GET", "/", nil)
	bad.Header.Set(DefaultHeader, "acme")
	bad.Header.Set("traceparent", "garbage")
	tc, err = HeaderResolver{}.Resolve(bad)
	if err != nil {
		t.Fatalf("malformed traceparent should be ignored, got err: %v", err)
	}
	if tc.TraceID() != "" {
		t.Fatalf("expected empty trace id, got %q", tc.TraceID())
	}
}
