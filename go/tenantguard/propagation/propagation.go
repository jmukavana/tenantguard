// Package propagation carries an immutable TenantContext across service, queue, and
// thread boundaries (core/spec.md, ContextPropagator). Tenant identity travels as W3C
// Baggage members under the "tg." namespace, so any OpenTelemetry-instrumented hop
// preserves it. Within a process, tenant context rides on context.Context and needs no
// serialization — Inject/Extract are for crossing a wire.
package propagation

import (
	"context"
	"encoding/json"
	"errors"

	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"

	tenantcontext "github.com/tenantguard/tenantguard/context"
)

// Baggage member keys. Trace id also rides in baggage so correlation is self-contained
// even before the OpenTelemetry SDK (and its traceparent handling) is wired in.
const (
	memberTenantID   = "tg.tenant.id"
	memberTenantName = "tg.tenant.name"
	memberTraceID    = "tg.trace.id"
	memberMetadata   = "tg.tenant.meta"
)

// ErrInvalidContext is returned by Inject when asked to propagate an invalid context.
var ErrInvalidContext = errors.New("tenantguard: cannot propagate an invalid tenant context")

// Carrier is the transport-agnostic key/value medium: HTTP headers, MQ headers, or a
// plain map. It is OpenTelemetry's TextMapCarrier, so propagation.HeaderCarrier and
// propagation.MapCarrier work out of the box.
type Carrier = propagation.TextMapCarrier

// Propagator serializes a TenantContext into and out of a Carrier as W3C Baggage.
// The zero value is ready to use.
type Propagator struct {
	baggage propagation.Baggage
}

// New returns a ready Propagator.
func New() Propagator { return Propagator{} }

// Inject writes tc into carrier as W3C Baggage, preserving any baggage already present
// in ctx. It returns ErrInvalidContext for a zero/invalid TenantContext.
func (p Propagator) Inject(ctx context.Context, tc tenantcontext.TenantContext, carrier Carrier) error {
	if !tc.IsValid() {
		return ErrInvalidContext
	}
	bag := baggage.FromContext(ctx)

	set := func(b baggage.Baggage, key, val string) (baggage.Baggage, error) {
		if val == "" {
			return b, nil
		}
		m, err := baggage.NewMemberRaw(key, val)
		if err != nil {
			return b, err
		}
		return b.SetMember(m)
	}

	var err error
	if bag, err = set(bag, memberTenantID, tc.TenantID()); err != nil {
		return err
	}
	if bag, err = set(bag, memberTenantName, tc.TenantName()); err != nil {
		return err
	}
	if bag, err = set(bag, memberTraceID, tc.TraceID()); err != nil {
		return err
	}
	if md := tc.Metadata(); len(md) > 0 {
		raw, err := json.Marshal(md)
		if err != nil {
			return err
		}
		if bag, err = set(bag, memberMetadata, string(raw)); err != nil {
			return err
		}
	}

	p.baggage.Inject(baggage.ContextWithBaggage(ctx, bag), carrier)
	return nil
}

// Extract reconstructs a TenantContext from carrier. ok is false (with nil error) when
// the carrier holds no tenant id. A non-nil error means the carrier held a tenant id but
// the payload was malformed (bad metadata JSON or trace id).
func (p Propagator) Extract(carrier Carrier) (tc tenantcontext.TenantContext, ok bool, err error) {
	bag := baggage.FromContext(p.baggage.Extract(context.Background(), carrier))

	id := bag.Member(memberTenantID).Value()
	if id == "" {
		return tenantcontext.TenantContext{}, false, nil
	}

	var md map[string]string
	if raw := bag.Member(memberMetadata).Value(); raw != "" {
		if err := json.Unmarshal([]byte(raw), &md); err != nil {
			return tenantcontext.TenantContext{}, false, err
		}
	}

	tc, err = tenantcontext.New(id, bag.Member(memberTenantName).Value(), bag.Member(memberTraceID).Value(), md)
	if err != nil {
		return tenantcontext.TenantContext{}, false, err
	}
	return tc, true, nil
}
