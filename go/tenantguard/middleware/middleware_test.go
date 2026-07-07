package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tenantguard/tenantguard/audit"
	tenantcontext "github.com/tenantguard/tenantguard/context"
	"github.com/tenantguard/tenantguard/resolver"
)

// recordingHook captures published events.
type recordingHook struct{ events []audit.Event }

func (h *recordingHook) Publish(_ context.Context, e audit.Event) error {
	h.events = append(h.events, e)
	return nil
}

func newTenant(t *testing.T, id string) tenantcontext.TenantContext {
	t.Helper()
	tc, err := tenantcontext.New(id, "", "", nil)
	if err != nil {
		t.Fatalf("New(%q): %v", id, err)
	}
	return tc
}

func TestGuard_ValidRequest(t *testing.T) {
	hook := &recordingHook{}
	var seen tenantcontext.TenantContext
	var reached bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		seen, _ = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(resolver.DefaultHeader, "acme")
	rec := httptest.NewRecorder()

	Guard{Resolver: resolver.HeaderResolver{}, Audit: hook}.Handler(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !reached {
		t.Fatal("downstream handler was not reached")
	}
	if seen.TenantID() != "acme" {
		t.Fatalf("FromContext tenant = %q, want acme", seen.TenantID())
	}
	if len(hook.events) != 1 || hook.events[0].Outcome != audit.OutcomeAllowed {
		t.Fatalf("expected one allowed event, got %+v", hook.events)
	}
}

func TestGuard_MissingTenant(t *testing.T) {
	hook := &recordingHook{}
	reached := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true })

	rec := httptest.NewRecorder()
	Guard{Resolver: resolver.HeaderResolver{}, Audit: hook}.Handler(next).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if reached {
		t.Fatal("downstream handler must not run without a tenant")
	}
	if len(hook.events) != 1 || hook.events[0].Outcome != audit.OutcomeDenied {
		t.Fatalf("expected one denied event, got %+v", hook.events)
	}
}

func TestGuard_RejectsTenantSwitch(t *testing.T) {
	hook := &recordingHook{}
	reached := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true })

	// Request already carries tenant "acme"; resolver reports "beta" from the header.
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(resolver.DefaultHeader, "beta")
	ctx, err := WithTenant(req.Context(), newTenant(t, "acme"))
	if err != nil {
		t.Fatalf("seed WithTenant: %v", err)
	}
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	Guard{Resolver: resolver.HeaderResolver{}, Audit: hook}.Handler(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	if reached {
		t.Fatal("downstream handler must not run on a rejected tenant switch")
	}
	if len(hook.events) != 1 || hook.events[0].EventType != "tenant.switch.rejected" {
		t.Fatalf("expected switch.rejected event, got %+v", hook.events)
	}
}

func TestWithTenant_Immutability(t *testing.T) {
	base := context.Background()
	ctx, err := WithTenant(base, newTenant(t, "acme"))
	if err != nil {
		t.Fatalf("first bind: %v", err)
	}
	// Re-binding the same tenant is a no-op.
	if _, err := WithTenant(ctx, newTenant(t, "acme")); err != nil {
		t.Fatalf("same-tenant rebind should succeed, got %v", err)
	}
	// Binding a different tenant is rejected.
	if _, err := WithTenant(ctx, newTenant(t, "beta")); !errors.Is(err, ErrTenantAlreadySet) {
		t.Fatalf("switch: got %v, want ErrTenantAlreadySet", err)
	}
}
