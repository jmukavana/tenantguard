// Package audit defines the tenant audit event and the Hook that publishes it. Every
// tenant-aware operation must emit an event (core/spec.md, Observability & Audit). In
// production these flow to AuditForge over an authenticated channel; that client is
// Ring 5. This package provides the contract plus a development LoggingHook.
package audit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"
)

// Outcome is the result of the audited operation.
type Outcome string

const (
	OutcomeAllowed Outcome = "allowed"
	OutcomeDenied  Outcome = "denied"
	OutcomeError   Outcome = "error"
)

// Event mirrors core/schemas/TenantEvent.schema.json. Its JSON encoding is the
// cross-language wire contract.
type Event struct {
	EventID   string            `json:"eventId"`
	EventType string            `json:"eventType"`
	TenantID  string            `json:"tenantId"`
	Timestamp time.Time         `json:"timestamp"`
	Outcome   Outcome           `json:"outcome"`
	TraceID   string            `json:"traceId,omitempty"`
	Actor     string            `json:"actor,omitempty"`
	Resource  string            `json:"resource,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Signature string            `json:"signature,omitempty"`
}

// Hook publishes tenant audit events to a sink.
type Hook interface {
	Publish(ctx context.Context, e Event) error
}

// NewEvent builds an Event stamped with a fresh id and the current UTC time.
func NewEvent(eventType, tenantID, traceID string, outcome Outcome) Event {
	return Event{
		EventID:   newEventID(),
		EventType: eventType,
		TenantID:  tenantID,
		Timestamp: time.Now().UTC(),
		Outcome:   outcome,
		TraceID:   traceID,
	}
}

func newEventID() string {
	var b [16]byte
	// crypto/rand.Read never returns an error on supported platforms.
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// LoggingHook is a development Hook that writes events as structured logs. It never
// errors. Replace with the AuditForge client in production.
type LoggingHook struct {
	Logger *slog.Logger
}

// Publish writes e to the hook's logger (or slog.Default when nil).
func (h LoggingHook) Publish(ctx context.Context, e Event) error {
	logger := h.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.InfoContext(ctx, "tenant.audit",
		"eventId", e.EventID,
		"eventType", e.EventType,
		"tenantId", e.TenantID,
		"outcome", string(e.Outcome),
		"traceId", e.TraceID,
	)
	return nil
}
