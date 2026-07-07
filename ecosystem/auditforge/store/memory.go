package store

import (
	"context"
	"sync"
	"time"

	"github.com/tenantguard/tenantguard/audit"
)

// Memory is an in-memory Store for tests and local development. Events are partitioned by
// tenant, which makes cross-tenant leakage structurally impossible. Safe for concurrent use.
type Memory struct {
	mu     sync.Mutex
	events map[string][]audit.Event       // tenantID -> events in arrival order
	seen   map[string]map[string]struct{} // tenantID -> set of eventIds
}

// NewMemory returns an empty Memory store.
func NewMemory() *Memory {
	return &Memory{
		events: make(map[string][]audit.Event),
		seen:   make(map[string]map[string]struct{}),
	}
}

// Append stores e unless its eventId already exists for the tenant (idempotent redelivery).
func (m *Memory) Append(_ context.Context, e audit.Event) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	seen := m.seen[e.TenantID]
	if seen == nil {
		seen = make(map[string]struct{})
		m.seen[e.TenantID] = seen
	}
	if _, duplicate := seen[e.EventID]; duplicate {
		return false, nil
	}
	seen[e.EventID] = struct{}{}
	m.events[e.TenantID] = append(m.events[e.TenantID], e)
	return true, nil
}

// Query returns a copy of the tenant's events within the time bounds, oldest first.
func (m *Memory) Query(_ context.Context, tenantID string, from, to time.Time) ([]audit.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []audit.Event
	for _, e := range m.events[tenantID] {
		if !from.IsZero() && e.Timestamp.Before(from) {
			continue
		}
		if !to.IsZero() && e.Timestamp.After(to) {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}
