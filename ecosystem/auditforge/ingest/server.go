package ingest

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/tenantguard/tenantguard/audit"
	"github.com/tenantguard/tenantguard/resolver"
)

// Handler returns an http.Handler serving the AuditForge ingest and query API.
func (s Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/events", s.handleIngest)
	mux.HandleFunc("GET /v1/events", s.handleQuery)
	return mux
}

// handleIngest verifies and stores a batch. A batch with per-event rejections still returns
// 200 (the ack carries them); only a storage failure or unreadable body is an error status.
func (s Service) handleIngest(w http.ResponseWriter, r *http.Request) {
	var req IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "malformed request body", http.StatusBadRequest)
		return
	}
	ack, err := s.Ingest(r.Context(), req)
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, ack)
}

// handleQuery answers a tenant-scoped compliance query. The tenant is resolved from the
// request by TenantGuard (dogfooding), so results are isolated to the caller's tenant
// regardless of any query parameter.
func (s Service) handleQuery(w http.ResponseWriter, r *http.Request) {
	tc, err := resolver.HeaderResolver{}.Resolve(r)
	if err != nil {
		http.Error(w, "missing tenant", http.StatusBadRequest)
		return
	}
	from, err := parseBound(r.URL.Query().Get("from"))
	if err != nil {
		http.Error(w, "invalid 'from' timestamp", http.StatusBadRequest)
		return
	}
	to, err := parseBound(r.URL.Query().Get("to"))
	if err != nil {
		http.Error(w, "invalid 'to' timestamp", http.StatusBadRequest)
		return
	}

	events, err := s.Store.Query(r.Context(), tc.TenantID(), from, to)
	if err != nil {
		http.Error(w, "query error", http.StatusInternalServerError)
		return
	}
	if events == nil {
		events = []audit.Event{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

// parseBound parses an RFC 3339 timestamp, treating an empty value as unbounded (zero time).
func parseBound(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, value)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
