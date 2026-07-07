package policy

import (
	"context"
	"testing"

	tenantcontext "github.com/tenantguard/tenantguard/context"
)

func tenant(t *testing.T, id string) tenantcontext.TenantContext {
	t.Helper()
	tc, err := tenantcontext.New(id, "", "", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return tc
}

func TestAllowDeny(t *testing.T) {
	if d := Allow(); !d.Allowed || d.Reason != "" {
		t.Fatalf("Allow() = %+v", d)
	}
	if d := Deny("blocked"); d.Allowed || d.Reason != "blocked" {
		t.Fatalf("Deny() = %+v", d)
	}
}

func TestAllowAll(t *testing.T) {
	if d := AllowAll.Evaluate(context.Background(), tenant(t, "acme"), "GET /x"); !d.Allowed {
		t.Fatalf("AllowAll denied: %+v", d)
	}
}

func TestHookFunc(t *testing.T) {
	// A hook that denies a specific tenant for write actions.
	h := HookFunc(func(_ context.Context, tc tenantcontext.TenantContext, action string) Decision {
		if tc.TenantID() == "frozen" && action == "POST /orders" {
			return Deny("tenant is frozen")
		}
		return Allow()
	})

	if d := h.Evaluate(context.Background(), tenant(t, "frozen"), "POST /orders"); d.Allowed {
		t.Fatal("expected frozen tenant to be denied")
	}
	if d := h.Evaluate(context.Background(), tenant(t, "acme"), "POST /orders"); !d.Allowed {
		t.Fatalf("expected acme allowed: %+v", d)
	}
	if d := h.Evaluate(context.Background(), tenant(t, "frozen"), "GET /orders"); !d.Allowed {
		t.Fatalf("expected read allowed for frozen: %+v", d)
	}
}
