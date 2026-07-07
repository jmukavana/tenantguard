// Package tenantcontext defines TenantContext, the immutable keystone type of
// TenantGuard. Every tenant-aware operation is performed on behalf of exactly one
// TenantContext, and the context cannot change once created — this is what prevents
// tenant hopping (see core/spec.md and ADR-004).
//
// The package is deliberately named tenantcontext rather than context to avoid
// shadowing the standard library's context package in callers such as middleware.
package tenantcontext

import (
	"errors"
	"regexp"
)

// ErrEmptyTenantID is returned by New when the tenant id is empty. A context
// without a tenant id has no isolation boundary and must never be constructed.
var ErrEmptyTenantID = errors.New("tenantguard: tenant id must not be empty")

// ErrInvalidTraceID is returned by New when a non-empty trace id is not a valid
// W3C trace-id (32 lowercase hex characters).
var ErrInvalidTraceID = errors.New("tenantguard: trace id must be 32 lowercase hex characters")

var traceIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// TenantContext is an immutable value describing the tenant an operation runs for.
// Its zero value is invalid; construct one with New. Fields are unexported and there
// are no setters, so a TenantContext cannot be mutated after construction. It is safe
// to copy and to share across goroutines.
//
// The shape mirrors core/schemas/TenantContext.schema.json.
type TenantContext struct {
	tenantID   string
	tenantName string
	traceID    string
	metadata   map[string]string
}

// New constructs an immutable TenantContext. tenantID is required. traceID, when
// non-empty, must be a valid W3C trace-id. The metadata map is defensively copied so
// later mutation of the caller's map cannot affect the returned context.
func New(tenantID, tenantName, traceID string, metadata map[string]string) (TenantContext, error) {
	if tenantID == "" {
		return TenantContext{}, ErrEmptyTenantID
	}
	if traceID != "" && !traceIDPattern.MatchString(traceID) {
		return TenantContext{}, ErrInvalidTraceID
	}
	return TenantContext{
		tenantID:   tenantID,
		tenantName: tenantName,
		traceID:    traceID,
		metadata:   cloneMetadata(metadata),
	}, nil
}

// TenantID returns the tenant's stable identifier — the isolation boundary key.
func (c TenantContext) TenantID() string { return c.tenantID }

// TenantName returns the human-readable tenant name. Informational only.
func (c TenantContext) TenantName() string { return c.tenantName }

// TraceID returns the W3C trace-id correlating this context, or "" if unset.
func (c TenantContext) TraceID() string { return c.traceID }

// Metadata returns a copy of the tenant metadata. Mutating the returned map does not
// affect the context, preserving immutability.
func (c TenantContext) Metadata() map[string]string { return cloneMetadata(c.metadata) }

// IsValid reports whether the context carries a tenant id. The zero TenantContext is
// not valid; anything built by New is.
func (c TenantContext) IsValid() bool { return c.tenantID != "" }

// cloneMetadata returns an independent copy of m (nil for nil/empty input).
func cloneMetadata(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
