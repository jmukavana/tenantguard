package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tenantguard/tenantguard/audit"

	"github.com/tenantguard/auditforge/chain"
)

// Memory is an in-memory Store for tests and local development. Events are partitioned by
// tenant, which makes cross-tenant leakage structurally impossible, and each tenant's records
// form a hash chain in arrival order. Safe for concurrent use.
type Memory struct {
	mu     sync.Mutex
	chains map[string][]chainRecord       // tenantID -> records in chain order
	seen   map[string]map[string]struct{} // tenantID -> set of eventIds
}

// NewMemory returns an empty Memory store.
func NewMemory() *Memory {
	return &Memory{
		chains: make(map[string][]chainRecord),
		seen:   make(map[string]map[string]struct{}),
	}
}

// Append stores e unless its eventId already exists for the tenant (idempotent redelivery),
// extending the tenant's hash chain when it is new.
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

	prev := chain.Genesis
	if records := m.chains[e.TenantID]; len(records) > 0 {
		prev = records[len(records)-1].hash
	}
	canonical, err := audit.CanonicalJSON(e)
	if err != nil {
		return false, fmt.Errorf("auditforge: canonicalize event %s: %w", e.EventID, err)
	}

	seen[e.EventID] = struct{}{}
	m.chains[e.TenantID] = append(m.chains[e.TenantID], chainRecord{
		event:    e,
		prevHash: prev,
		hash:     chain.Next(prev, canonical),
	})
	return true, nil
}

// Query returns a copy of the tenant's events within the time bounds, oldest first.
func (m *Memory) Query(_ context.Context, tenantID string, from, to time.Time) ([]audit.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []audit.Event
	for _, r := range m.chains[tenantID] {
		if !from.IsZero() && r.event.Timestamp.Before(from) {
			continue
		}
		if !to.IsZero() && r.event.Timestamp.After(to) {
			continue
		}
		out = append(out, r.event)
	}
	return out, nil
}

// VerifyChain replays the tenant's hash chain.
func (m *Memory) VerifyChain(_ context.Context, tenantID string) (ChainReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return replayChain(m.chains[tenantID]), nil
}
