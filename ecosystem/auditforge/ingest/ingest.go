// Package ingest is AuditForge's ingestion and query service: it verifies each event's
// signature against a keyring, enforces idempotency, and persists to a tenant-isolated Store
// (ecosystem/auditforge/spec.md, "Ingestion API"). Signature verification reuses TenantGuard's
// audit.Verify so AuditForge computes the canonical form byte-identically to every emitter.
package ingest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tenantguard/tenantguard/audit"

	"github.com/tenantguard/auditforge/keyring"
	"github.com/tenantguard/auditforge/store"
)

// Rejection reasons (ecosystem/auditforge/schemas/IngestAck.schema.json).
const (
	reasonUnknownKey       = "unknown_key"
	reasonMalformedEvent   = "malformed_event"
	reasonInvalidSignature = "invalid_signature"
)

// IngestRequest is the POST /v1/events body (ecosystem/auditforge/schemas/IngestRequest).
// AuditForge owns this type independently of the emitter's client, sharing only the JSON
// contract fixed by the schema.
type IngestRequest struct {
	KeyID  string        `json:"keyId"`
	Events []audit.Event `json:"events"`
}

// Rejection is one entry of IngestAck.Rejected: an eventId and why it was not stored.
type Rejection struct {
	EventID string `json:"eventId"`
	Reason  string `json:"reason"`
}

// IngestAck partitions a submitted batch (ecosystem/auditforge/schemas/IngestAck).
type IngestAck struct {
	Received  int         `json:"received"`
	Accepted  []string    `json:"accepted"`
	Duplicate []string    `json:"duplicate"`
	Rejected  []Rejection `json:"rejected"`
}

// ErrRetentionDisabled is returned by Purge when no retention window is configured.
var ErrRetentionDisabled = errors.New("auditforge: retention purge is disabled")

// Service verifies and stores audit events.
type Service struct {
	Keys  keyring.Keyring
	Store store.Store
	// Retention is the minimum time events are kept. Purge removes only records older than
	// this window; zero disables purging entirely. Deletion is never exposed over HTTP.
	Retention time.Duration
}

// Purge enforces the retention window for a tenant: it removes records older than Retention
// and returns the resulting signed checkpoint. It is an operator/scheduler entry point, not
// part of the public API, so AuditForge never deletes in response to a request.
func (s Service) Purge(ctx context.Context, tenantID string) (store.PurgeResult, error) {
	if s.Retention <= 0 {
		return store.PurgeResult{}, ErrRetentionDisabled
	}
	return s.Store.Purge(ctx, tenantID, time.Now().Add(-s.Retention))
}

// Ingest verifies each event and stores the valid, new ones, returning an ack partitioning
// the batch. Per-event faults (unknown key, malformed, bad signature) are recorded as
// rejections and never stored. A non-nil error signals a storage failure that aborted the
// batch — the caller should surface it as a 5xx so the sender retries (idempotency makes
// redelivery safe).
func (s Service) Ingest(ctx context.Context, req IngestRequest) (IngestAck, error) {
	secret, known := s.Keys.Secret(req.KeyID)
	ack := IngestAck{
		Received:  len(req.Events),
		Accepted:  []string{},
		Duplicate: []string{},
		Rejected:  []Rejection{},
	}

	for _, e := range req.Events {
		switch {
		case !known:
			ack.Rejected = append(ack.Rejected, Rejection{e.EventID, reasonUnknownKey})
		case e.EventID == "" || e.TenantID == "" || e.EventType == "":
			ack.Rejected = append(ack.Rejected, Rejection{e.EventID, reasonMalformedEvent})
		default:
			if ok, _ := audit.Verify(e, secret, e.Signature); !ok {
				ack.Rejected = append(ack.Rejected, Rejection{e.EventID, reasonInvalidSignature})
				continue
			}
			stored, err := s.Store.Append(ctx, e)
			if err != nil {
				return IngestAck{}, fmt.Errorf("auditforge: store event %s: %w", e.EventID, err)
			}
			if stored {
				ack.Accepted = append(ack.Accepted, e.EventID)
			} else {
				ack.Duplicate = append(ack.Duplicate, e.EventID)
			}
		}
	}
	return ack, nil
}
