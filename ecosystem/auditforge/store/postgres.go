package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/tenantguard/tenantguard/audit"
	tenantcontext "github.com/tenantguard/tenantguard/context"
	"github.com/tenantguard/tenantguard/isolation"
)

// Postgres is a schema-isolated Store: each tenant's events live in their own Postgres
// schema ("Prefix+tenantID"), bound per query via TenantGuard's SchemaStrategy. Isolation is
// therefore enforced by the database — an unqualified query for tenant A can never resolve
// tenant B's events table. Idempotency is a per-schema PRIMARY KEY on event_id.
//
// The caller supplies a *sql.DB with a driver registered (e.g. lib/pq). Safe for concurrent
// use.
type Postgres struct {
	db       *sql.DB
	strategy isolation.SchemaStrategy
	ensured  sync.Map // tenantID -> struct{}: schema+table provisioned this process
}

// NewPostgres returns a Postgres store mapping each tenant to the schema "prefix+tenantID".
func NewPostgres(db *sql.DB, prefix string) *Postgres {
	return &Postgres{db: db, strategy: isolation.SchemaStrategy{Prefix: prefix}}
}

// Append inserts e into its tenant's schema, or reports a duplicate if the eventId is
// already present (ON CONFLICT DO NOTHING).
func (p *Postgres) Append(ctx context.Context, e audit.Event) (bool, error) {
	tc, err := tenantContext(e.TenantID)
	if err != nil {
		return false, err
	}
	if err := p.ensureTenant(ctx, tc); err != nil {
		return false, err
	}
	payload, err := json.Marshal(e)
	if err != nil {
		return false, fmt.Errorf("auditforge: marshal event %s: %w", e.EventID, err)
	}

	conn, err := p.db.Conn(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	if err := p.strategy.Bind(ctx, conn, tc); err != nil {
		return false, err
	}

	result, err := conn.ExecContext(ctx,
		`INSERT INTO events (event_id, ts, event) VALUES ($1, $2, $3::jsonb)
		 ON CONFLICT (event_id) DO NOTHING`,
		e.EventID, e.Timestamp, string(payload))
	if err != nil {
		return false, fmt.Errorf("auditforge: insert event %s: %w", e.EventID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// Query returns the tenant's events within [from, to], oldest first. A zero bound is
// unbounded on that side.
func (p *Postgres) Query(ctx context.Context, tenantID string, from, to time.Time) ([]audit.Event, error) {
	tc, err := tenantContext(tenantID)
	if err != nil {
		return nil, err
	}
	if err := p.ensureTenant(ctx, tc); err != nil {
		return nil, err
	}

	conn, err := p.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if err := p.strategy.Bind(ctx, conn, tc); err != nil {
		return nil, err
	}

	rows, err := conn.QueryContext(ctx,
		`SELECT event FROM events
		  WHERE ($1::timestamptz IS NULL OR ts >= $1)
		    AND ($2::timestamptz IS NULL OR ts <= $2)
		  ORDER BY ts, event_id`,
		nullableTime(from), nullableTime(to))
	if err != nil {
		return nil, fmt.Errorf("auditforge: query events: %w", err)
	}
	defer rows.Close()

	var out []audit.Event
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, fmt.Errorf("auditforge: scan event: %w", err)
		}
		var e audit.Event
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, fmt.Errorf("auditforge: decode stored event: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ensureTenant provisions the tenant's schema and events table once per process. It uses
// the validated, injection-safe schema name and CREATE ... IF NOT EXISTS, so it is safe to
// call before every operation.
func (p *Postgres) ensureTenant(ctx context.Context, tc tenantcontext.TenantContext) error {
	if _, done := p.ensured.Load(tc.TenantID()); done {
		return nil
	}
	schema, err := p.strategy.SchemaName(tc)
	if err != nil {
		return err
	}
	// schema matched SchemaStrategy's [A-Za-z0-9_] allowlist, so quoting carries no injection.
	stmts := []string{
		fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %q`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %q.events (
			event_id text PRIMARY KEY,
			ts       timestamptz NOT NULL,
			event    jsonb NOT NULL
		)`, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS events_ts_idx ON %q.events (ts)`, schema),
	}
	for _, stmt := range stmts {
		if _, err := p.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("auditforge: provision schema %q: %w", schema, err)
		}
	}
	p.ensured.Store(tc.TenantID(), struct{}{})
	return nil
}

func tenantContext(tenantID string) (tenantcontext.TenantContext, error) {
	tc, err := tenantcontext.New(tenantID, "", "", nil)
	if err != nil {
		return tenantcontext.TenantContext{}, fmt.Errorf("auditforge: %w", err)
	}
	return tc, nil
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
