// Package policy provides a minimal tenant policy hook (core/spec.md, PolicyHook): a
// pluggable allow/deny decision consulted after a tenant is resolved. This is the seed
// of the policy engine on the roadmap; today it is just the contract plus small
// building blocks. Decisions must fail closed — an unsure hook should Deny.
package policy

import (
	"context"

	tenantcontext "github.com/tenantguard/tenantguard/context"
)

// Decision is the outcome of a policy evaluation.
type Decision struct {
	Allowed bool
	// Reason explains a denial (surfaced to audit); empty when allowed.
	Reason string
}

// Allow returns an allowing Decision.
func Allow() Decision { return Decision{Allowed: true} }

// Deny returns a denying Decision carrying reason.
func Deny(reason string) Decision { return Decision{Allowed: false, Reason: reason} }

// Hook decides whether a tenant may perform an action (e.g. an HTTP method+path, a
// message type, or a database operation).
type Hook interface {
	Evaluate(ctx context.Context, tc tenantcontext.TenantContext, action string) Decision
}

// HookFunc adapts a function to a Hook.
type HookFunc func(ctx context.Context, tc tenantcontext.TenantContext, action string) Decision

// Evaluate calls f.
func (f HookFunc) Evaluate(ctx context.Context, tc tenantcontext.TenantContext, action string) Decision {
	return f(ctx, tc, action)
}

// AllowAll permits every action. Suitable as an explicit default, not for production
// gating.
var AllowAll Hook = HookFunc(func(context.Context, tenantcontext.TenantContext, string) Decision {
	return Allow()
})
