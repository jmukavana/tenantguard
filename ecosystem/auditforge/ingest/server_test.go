package ingest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tenantguard/tenantguard/audit"

	"github.com/tenantguard/auditforge/keyring"
	"github.com/tenantguard/auditforge/store"
)

var serverSecret = []byte("tenantguard-shared-secret")

func signedEvent(t *testing.T, tenantID, eventID string) audit.Event {
	t.Helper()
	e := audit.NewEvent("tenant.resolved", tenantID, "", audit.OutcomeAllowed)
	e.EventID = eventID
	sig, err := audit.Sign(e, serverSecret)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	e.Signature = sig
	return e
}

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	svc := Service{Keys: keyring.Map{"k1": serverSecret}, Store: store.NewMemory(checkpointSecret)}
	srv := httptest.NewServer(svc.Handler())
	t.Cleanup(srv.Close)
	return srv
}

func post(t *testing.T, base string, req IngestRequest) IngestAck {
	t.Helper()
	body, _ := json.Marshal(req)
	resp, err := http.Post(base+"/v1/events", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST status = %d, want 200", resp.StatusCode)
	}
	var ack IngestAck
	if err := json.NewDecoder(resp.Body).Decode(&ack); err != nil {
		t.Fatalf("decode ack: %v", err)
	}
	return ack
}

func query(t *testing.T, base, tenantID string) []audit.Event {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, base+"/v1/events", nil)
	if tenantID != "" {
		req.Header.Set("X-Tenant-ID", tenantID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Events []audit.Event `json:"events"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode query: %v", err)
	}
	return body.Events
}

func TestServerIngestThenTenantIsolatedQuery(t *testing.T) {
	srv := newServer(t)

	ack := post(t, srv.URL, IngestRequest{KeyID: "k1", Events: []audit.Event{
		signedEvent(t, "acme", "a1"),
		signedEvent(t, "globex", "g1"),
	}})
	if len(ack.Accepted) != 2 {
		t.Fatalf("expected 2 accepted, got %+v", ack)
	}

	acme := query(t, srv.URL, "acme")
	if len(acme) != 1 || acme[0].TenantID != "acme" {
		t.Fatalf("acme query must return only acme events, got %+v", acme)
	}
	globex := query(t, srv.URL, "globex")
	if len(globex) != 1 || globex[0].TenantID != "globex" {
		t.Fatalf("globex query must return only globex events, got %+v", globex)
	}
}

func TestServerChainEndpointReportsIntactChain(t *testing.T) {
	srv := newServer(t)

	post(t, srv.URL, IngestRequest{KeyID: "k1", Events: []audit.Event{
		signedEvent(t, "acme", "a1"),
		signedEvent(t, "acme", "a2"),
	}})

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/events/chain", nil)
	req.Header.Set("X-Tenant-ID", "acme")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET chain: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("chain status = %d, want 200", resp.StatusCode)
	}

	var report store.ChainReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if !report.Intact || report.Length != 2 || report.Head == "" {
		t.Fatalf("chain report = %+v, want intact len 2 with a head", report)
	}
}

func TestServerRejectedEventStillReturns200(t *testing.T) {
	srv := newServer(t)

	tampered := signedEvent(t, "acme", "t1")
	tampered.Actor = "forged-after-signing" // invalidates the signature

	ack := post(t, srv.URL, IngestRequest{KeyID: "k1", Events: []audit.Event{tampered}})
	if len(ack.Rejected) != 1 || ack.Rejected[0].Reason != reasonInvalidSignature {
		t.Fatalf("want one invalid_signature rejection, got %+v", ack)
	}
}

func TestServerRejectsMissingTenantOnQuery(t *testing.T) {
	srv := newServer(t)

	resp, err := http.Get(srv.URL + "/v1/events")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("query without tenant should be 400, got %d", resp.StatusCode)
	}
}

func TestServerRejectsMalformedBody(t *testing.T) {
	srv := newServer(t)

	resp, err := http.Post(srv.URL+"/v1/events", "application/json", strings.NewReader("{not json"))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed body should be 400, got %d", resp.StatusCode)
	}
}
