// Package middleware wires the request pipeline: resolve the tenant, bind an immutable
// TenantContext to the request's context, emit an audit event, and reject unidentified
// or tenant-switching requests (core/spec.md, Security Requirements). It is the Ring 3
// composition of resolver + context + audit.
package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/tenantguard/tenantguard/audit"
	tenantcontext "github.com/tenantguard/tenantguard/context"
	"github.com/tenantguard/tenantguard/policy"
	"github.com/tenantguard/tenantguard/resolver"
)

type ctxKey struct{}

// ErrTenantAlreadySet is returned by WithTenant when a request already carries a
// different tenant. Switching tenants mid-request is forbidden by the spec.
var ErrTenantAlreadySet = errors.New("tenantguard: tenant context already set; refusing to switch tenants mid-request")

// FromContext returns the TenantContext bound to ctx, if any.
func FromContext(ctx context.Context) (tenantcontext.TenantContext, bool) {
	tc, ok := ctx.Value(ctxKey{}).(tenantcontext.TenantContext)
	return tc, ok
}

// WithTenant binds tc to ctx. Re-binding the same tenant is a no-op; binding a
// different tenant returns ErrTenantAlreadySet, enforcing immutability of the
// request's tenant identity.
func WithTenant(ctx context.Context, tc tenantcontext.TenantContext) (context.Context, error) {
	if existing, ok := FromContext(ctx); ok {
		if existing.TenantID() == tc.TenantID() {
			return ctx, nil
		}
		return ctx, ErrTenantAlreadySet
	}
	return context.WithValue(ctx, ctxKey{}, tc), nil
}

// Guard is net/http middleware enforcing tenant resolution and isolation-of-identity.
type Guard struct {
	// Resolver extracts the tenant from each request. Required.
	Resolver resolver.Resolver
	// Audit receives an event per request. Optional; nil disables auditing.
	Audit audit.Hook
	// Policy, when set, gates each resolved request. A denied Decision yields 403.
	// The action passed to the hook is "METHOD path". Optional; nil allows all.
	Policy policy.Hook
}

// Handler returns middleware that guards next. Requests without a resolvable tenant get
// 401; requests attempting to switch tenants get 409. Otherwise the downstream handler
// runs with the TenantContext available via FromContext.
func (g Guard) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc, err := g.Resolver.Resolve(r)
		if err != nil {
			g.publish(r.Context(), audit.NewEvent("tenant.resolve", "", "", audit.OutcomeDenied))
			http.Error(w, "tenant not identified", http.StatusUnauthorized)
			return
		}

		ctx, err := WithTenant(r.Context(), tc)
		if err != nil {
			g.publish(r.Context(), audit.NewEvent("tenant.switch.rejected", tc.TenantID(), tc.TraceID(), audit.OutcomeDenied))
			http.Error(w, "tenant switch rejected", http.StatusConflict)
			return
		}

		if g.Policy != nil {
			if d := g.Policy.Evaluate(ctx, tc, r.Method+" "+r.URL.Path); !d.Allowed {
				e := audit.NewEvent("policy.denied", tc.TenantID(), tc.TraceID(), audit.OutcomeDenied)
				e.Resource = r.URL.Path
				e.Metadata = map[string]string{"reason": d.Reason}
				g.publish(ctx, e)
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}

		g.publish(ctx, audit.NewEvent("tenant.resolved", tc.TenantID(), tc.TraceID(), audit.OutcomeAllowed))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (g Guard) publish(ctx context.Context, e audit.Event) {
	if g.Audit != nil {
		_ = g.Audit.Publish(ctx, e)
	}
}
