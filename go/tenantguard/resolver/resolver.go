// Package resolver extracts tenant identity from an inbound request and produces an
// immutable TenantContext (core/spec.md, TenantResolver). This package provides the
// HTTP header strategy; JWT-claim and subdomain resolvers are Ring 1 follow-ups.
package resolver

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	tenantcontext "github.com/tenantguard/tenantguard/context"
)

// DefaultHeader is the conventional tenant header (core/spec.md, TenantResolver).
const DefaultHeader = "X-Tenant-ID"

// ErrNoTenant is returned when the request carries no tenant identifier.
var ErrNoTenant = errors.New("tenantguard: no tenant identifier in request")

var traceIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Resolver turns a request into a TenantContext.
type Resolver interface {
	Resolve(r *http.Request) (tenantcontext.TenantContext, error)
}

// HeaderResolver resolves the tenant from an HTTP header. When present, the W3C
// traceparent header seeds the context's trace id so audit events correlate.
type HeaderResolver struct {
	// Header names the tenant header; empty means DefaultHeader.
	Header string
}

// Resolve reads the tenant id from the configured header, returning ErrNoTenant when
// absent. It never trusts request-supplied names beyond the tenant id itself.
func (hr HeaderResolver) Resolve(r *http.Request) (tenantcontext.TenantContext, error) {
	name := hr.Header
	if name == "" {
		name = DefaultHeader
	}
	id := strings.TrimSpace(r.Header.Get(name))
	if id == "" {
		return tenantcontext.TenantContext{}, ErrNoTenant
	}
	return tenantcontext.New(id, "", traceIDFromRequest(r), nil)
}

// traceIDFromRequest best-effort extracts the W3C trace-id from a traceparent header
// (version-traceid-spanid-flags). It returns "" for missing or malformed values so a
// bad trace header never fails an otherwise valid request.
func traceIDFromRequest(r *http.Request) string {
	parts := strings.Split(r.Header.Get("traceparent"), "-")
	if len(parts) < 4 {
		return ""
	}
	traceID := strings.ToLower(parts[1])
	if !traceIDPattern.MatchString(traceID) {
		return ""
	}
	return traceID
}
