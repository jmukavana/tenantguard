package auditforge

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tenantguard/tenantguard/audit"
)

var secret = []byte("tenantguard-shared-secret")

// noBackoff removes retry delay so tests don't sleep.
func noBackoff(int) time.Duration { return 0 }

// captured is what a fakeServer records about the request it received.
type captured struct {
	method      string
	path        string
	contentType string
	request     IngestRequest
}

// fakeServer stands in for AuditForge. handler decides the response per attempt; last holds
// the most recent decoded request for assertions.
func fakeServer(t *testing.T, handler func(w http.ResponseWriter, req IngestRequest, attempt int)) (*httptest.Server, *captured, *int32) {
	t.Helper()
	var last captured
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&calls, 1)
		body, _ := io.ReadAll(r.Body)
		var req IngestRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("server: decode request: %v", err)
		}
		last = captured{r.Method, r.URL.Path, r.Header.Get("Content-Type"), req}
		handler(w, req, int(attempt))
	}))
	t.Cleanup(srv.Close)
	return srv, &last, &calls
}

func writeAck(w http.ResponseWriter, ack IngestAck) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ack)
}

func TestPublishSignsAndAccepts(t *testing.T) {
	srv, last, calls := fakeServer(t, func(w http.ResponseWriter, req IngestRequest, _ int) {
		// The server verifies exactly as AuditForge would: recompute the HMAC over the
		// event and compare to the transported signature.
		e := req.Events[0]
		ok, err := audit.Verify(e, secret, e.Signature)
		if err != nil || !ok {
			t.Errorf("signature did not verify: ok=%v err=%v", ok, err)
		}
		writeAck(w, IngestAck{Received: 1, Accepted: []string{e.EventID}})
	})

	hook := Hook{Endpoint: srv.URL, KeyID: "k1", Secret: secret}
	e := audit.NewEvent("tenant.resolved", "acme", "", audit.OutcomeAllowed)
	if err := hook.Publish(context.Background(), e); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if *calls != 1 {
		t.Errorf("calls = %d, want 1", *calls)
	}
	if last.method != http.MethodPost || last.path != "/v1/events" {
		t.Errorf("got %s %s, want POST /v1/events", last.method, last.path)
	}
	if !strings.HasPrefix(last.contentType, "application/json") {
		t.Errorf("Content-Type = %q", last.contentType)
	}
	if last.request.KeyID != "k1" {
		t.Errorf("keyId = %q, want k1", last.request.KeyID)
	}
	if got := last.request.Events[0].Signature; got == "" {
		t.Error("transported event carries no signature")
	}
}

func TestPublishDuplicateIsSuccess(t *testing.T) {
	srv, _, _ := fakeServer(t, func(w http.ResponseWriter, req IngestRequest, _ int) {
		writeAck(w, IngestAck{Received: 1, Duplicate: []string{req.Events[0].EventID}})
	})

	hook := Hook{Endpoint: srv.URL, KeyID: "k1", Secret: secret}
	if err := hook.Publish(context.Background(), audit.NewEvent("x", "acme", "", audit.OutcomeAllowed)); err != nil {
		t.Fatalf("duplicate should be an idempotent success, got %v", err)
	}
}

func TestPublishRejectedIsError(t *testing.T) {
	srv, _, _ := fakeServer(t, func(w http.ResponseWriter, req IngestRequest, _ int) {
		writeAck(w, IngestAck{
			Received: 1,
			Rejected: []Rejection{{EventID: req.Events[0].EventID, Reason: "invalid_signature"}},
		})
	})

	hook := Hook{Endpoint: srv.URL, KeyID: "k1", Secret: secret}
	err := hook.Publish(context.Background(), audit.NewEvent("x", "acme", "", audit.OutcomeAllowed))
	if err == nil || !strings.Contains(err.Error(), "invalid_signature") {
		t.Fatalf("want rejection error mentioning the reason, got %v", err)
	}
}

func TestPublishUnacknowledgedIsError(t *testing.T) {
	srv, _, _ := fakeServer(t, func(w http.ResponseWriter, _ IngestRequest, _ int) {
		writeAck(w, IngestAck{Received: 1}) // event in no partition at all
	})

	hook := Hook{Endpoint: srv.URL, KeyID: "k1", Secret: secret}
	err := hook.Publish(context.Background(), audit.NewEvent("x", "acme", "", audit.OutcomeAllowed))
	if err == nil || !strings.Contains(err.Error(), "missing from ack") {
		t.Fatalf("want unacknowledged error, got %v", err)
	}
}

func TestPublishRetriesOn5xx(t *testing.T) {
	srv, _, calls := fakeServer(t, func(w http.ResponseWriter, req IngestRequest, attempt int) {
		if attempt == 1 {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		writeAck(w, IngestAck{Received: 1, Accepted: []string{req.Events[0].EventID}})
	})

	hook := Hook{Endpoint: srv.URL, KeyID: "k1", Secret: secret, MaxRetries: 2, Backoff: noBackoff}
	if err := hook.Publish(context.Background(), audit.NewEvent("x", "acme", "", audit.OutcomeAllowed)); err != nil {
		t.Fatalf("Publish should succeed after retry: %v", err)
	}
	if *calls != 2 {
		t.Errorf("calls = %d, want 2 (one 503, one 200)", *calls)
	}
}

func TestPublishDoesNotRetry4xx(t *testing.T) {
	srv, _, calls := fakeServer(t, func(w http.ResponseWriter, _ IngestRequest, _ int) {
		http.Error(w, "unknown key", http.StatusUnauthorized)
	})

	hook := Hook{Endpoint: srv.URL, KeyID: "bad", Secret: secret, MaxRetries: 3, Backoff: noBackoff}
	err := hook.Publish(context.Background(), audit.NewEvent("x", "acme", "", audit.OutcomeAllowed))
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("want non-retryable 401 error, got %v", err)
	}
	if *calls != 1 {
		t.Errorf("calls = %d, want 1 (4xx must not retry)", *calls)
	}
}

func TestPublishExhaustsRetries(t *testing.T) {
	srv, _, calls := fakeServer(t, func(w http.ResponseWriter, _ IngestRequest, _ int) {
		http.Error(w, "down", http.StatusBadGateway)
	})

	hook := Hook{Endpoint: srv.URL, KeyID: "k1", Secret: secret, MaxRetries: 2, Backoff: noBackoff}
	err := hook.Publish(context.Background(), audit.NewEvent("x", "acme", "", audit.OutcomeAllowed))
	if err == nil || !strings.Contains(err.Error(), "giving up after 3 attempts") {
		t.Fatalf("want exhausted-retries error, got %v", err)
	}
	if *calls != 3 {
		t.Errorf("calls = %d, want 3 (initial + 2 retries)", *calls)
	}
}

// Hook must satisfy audit.Hook so it drops into the middleware pipeline.
var _ audit.Hook = Hook{}
