// Package auditforge provides Hook, an audit.Hook that signs each TenantEvent and ships it
// to an AuditForge server over POST /v1/events (ecosystem/auditforge/spec.md). It is the
// production replacement for audit.LoggingHook: every tenant-aware operation's event is
// HMAC-signed with a shared key and appended to the tamper-evident audit log.
//
// This lives in its own package so the core audit package stays free of net/http.
package auditforge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/tenantguard/tenantguard/audit"
)

// eventsPath is the ingest endpoint appended to the configured base Endpoint.
const eventsPath = "/v1/events"

// defaultClient is used when a Hook leaves Client nil. The timeout bounds a single attempt.
var defaultClient = &http.Client{Timeout: 10 * time.Second}

// IngestRequest is the POST /v1/events body (ecosystem/auditforge/schemas/IngestRequest).
type IngestRequest struct {
	KeyID  string        `json:"keyId"`
	Events []audit.Event `json:"events"`
}

// IngestAck is the POST /v1/events response (ecosystem/auditforge/schemas/IngestAck). It
// partitions the submitted batch; a rejection carries the reason it was not stored.
type IngestAck struct {
	Received  int         `json:"received"`
	Accepted  []string    `json:"accepted"`
	Duplicate []string    `json:"duplicate"`
	Rejected  []Rejection `json:"rejected"`
}

// Rejection is one entry of IngestAck.Rejected: an eventId and why it was not stored.
type Rejection struct {
	EventID string `json:"eventId"`
	Reason  string `json:"reason"`
}

// Hook signs each event and POSTs it to AuditForge. The zero value is not usable; set at
// least Endpoint, KeyID, and Secret.
type Hook struct {
	// Endpoint is the AuditForge base URL, e.g. "https://auditforge.internal". The hook
	// POSTs to Endpoint + "/v1/events".
	Endpoint string
	// KeyID identifies the shared signing key; sent as keyId in every IngestRequest so the
	// server knows which secret to verify against.
	KeyID string
	// Secret is the shared HMAC-SHA256 key used to sign each event.
	Secret []byte
	// Client is the HTTP client to use; nil uses a shared client with a 10s timeout.
	Client *http.Client
	// MaxRetries is how many extra attempts follow the first on a retryable failure (a
	// network error or a 5xx). Zero means a single attempt.
	MaxRetries int
	// Backoff returns the delay before retry attempt n (1-based); nil uses exponential
	// backoff starting at 100ms.
	Backoff func(attempt int) time.Duration
}

// Publish signs e, submits it as a one-event IngestRequest, and reports whether AuditForge
// stored it. A duplicate (same eventId already present) is an idempotent success. It errors
// if the event is rejected, if the batch comes back unacknowledged, or if the request cannot
// be delivered within the retry budget.
func (h Hook) Publish(ctx context.Context, e audit.Event) error {
	signature, err := audit.Sign(e, h.Secret)
	if err != nil {
		return fmt.Errorf("auditforge: sign event %s: %w", e.EventID, err)
	}
	e.Signature = signature

	body, err := json.Marshal(IngestRequest{KeyID: h.KeyID, Events: []audit.Event{e}})
	if err != nil {
		return fmt.Errorf("auditforge: marshal request: %w", err)
	}

	ack, err := h.post(ctx, body)
	if err != nil {
		return err
	}
	return ackError(e.EventID, ack)
}

// post delivers body to the ingest endpoint, retrying retryable failures up to MaxRetries.
func (h Hook) post(ctx context.Context, body []byte) (IngestAck, error) {
	url := strings.TrimRight(h.Endpoint, "/") + eventsPath
	var lastErr error
	for attempt := 0; attempt <= h.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return IngestAck{}, ctx.Err()
			case <-time.After(h.backoff(attempt)):
			}
		}
		ack, retryable, err := h.attempt(ctx, url, body)
		if err == nil {
			return ack, nil
		}
		lastErr = err
		if !retryable {
			return IngestAck{}, err
		}
	}
	return IngestAck{}, fmt.Errorf("auditforge: giving up after %d attempts: %w", h.MaxRetries+1, lastErr)
}

// attempt makes one POST. The bool reports whether a failure is worth retrying.
func (h Hook) attempt(ctx context.Context, url string, body []byte) (IngestAck, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return IngestAck{}, false, fmt.Errorf("auditforge: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client().Do(req)
	if err != nil {
		return IngestAck{}, true, fmt.Errorf("auditforge: POST %s: %w", url, err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return IngestAck{}, true, fmt.Errorf("auditforge: read response: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusOK:
		var ack IngestAck
		if err := json.Unmarshal(payload, &ack); err != nil {
			return IngestAck{}, false, fmt.Errorf("auditforge: decode ack: %w", err)
		}
		return ack, false, nil
	case resp.StatusCode >= 500:
		return IngestAck{}, true, fmt.Errorf("auditforge: server error %d: %s", resp.StatusCode, payload)
	default:
		// 4xx is a transport/auth failure (bad key, malformed batch); retrying won't help.
		return IngestAck{}, false, fmt.Errorf("auditforge: request rejected %d: %s", resp.StatusCode, payload)
	}
}

func (h Hook) client() *http.Client {
	if h.Client != nil {
		return h.Client
	}
	return defaultClient
}

func (h Hook) backoff(attempt int) time.Duration {
	if h.Backoff != nil {
		return h.Backoff(attempt)
	}
	return time.Duration(100*(1<<(attempt-1))) * time.Millisecond
}

// ackError inspects an IngestAck for a single submitted event. A rejection is an error; an
// event absent from every partition is a protocol violation and also an error, so a silent
// drop can never masquerade as success in a compliance log.
func ackError(eventID string, ack IngestAck) error {
	for _, r := range ack.Rejected {
		if r.EventID == eventID {
			return fmt.Errorf("auditforge: event %s rejected: %s", eventID, r.Reason)
		}
	}
	if slices.Contains(ack.Accepted, eventID) || slices.Contains(ack.Duplicate, eventID) {
		return nil
	}
	return fmt.Errorf("auditforge: event %s missing from ack (received=%d)", eventID, ack.Received)
}
