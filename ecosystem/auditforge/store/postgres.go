package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/tenantguard/tenantguard/audit"
	tenantcontext "github.com/tenantguard/tenantguard/context"
	"github.com/tenantguard/tenantguard/isolation"

	"github.com/tenantguard/auditforge/chain"
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
	secret   []byte   // signs retention checkpoints
	ensured  sync.Map // tenantID -> struct{}: schema+tables provisioned this process
}

// NewPostgres returns a Postgres store mapping each tenant to the schema "prefix+tenantID".
// checkpointSecret signs retention checkpoints.
func NewPostgres(db *sql.DB, prefix string, checkpointSecret []byte) *Postgres {
	return &Postgres{db: db, strategy: isolation.SchemaStrategy{Prefix: prefix}, secret: checkpointSecret}
}

// rowQueryer is the QueryRowContext method shared by *sql.Conn and *sql.Tx.
type rowQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// loadCheckpoint reads the tenant's retention checkpoint (zero-valued when none exists). q
// must already be bound to the tenant's schema.
func loadCheckpoint(ctx context.Context, q rowQueryer) (chain.Checkpoint, error) {
	var cp chain.Checkpoint
	var length int64
	err := q.QueryRowContext(ctx,
		`SELECT length, hash, through, signature FROM chain_checkpoint LIMIT 1`).
		Scan(&length, &cp.Hash, &cp.Through, &cp.Signature)
	if errors.Is(err, sql.ErrNoRows) {
		return chain.Checkpoint{}, nil
	}
	if err != nil {
		return chain.Checkpoint{}, fmt.Errorf("auditforge: read checkpoint: %w", err)
	}
	cp.Length = int(length)
	return cp, nil
}

// Append inserts e into its tenant's schema, extending the tenant's hash chain, or reports a
// duplicate if the eventId is already present (ON CONFLICT DO NOTHING). The chain head is
// read and the row inserted under an EXCLUSIVE table lock so concurrent appends cannot fork
// the chain; that lock still admits concurrent reads (ACCESS SHARE).
func (p *Postgres) Append(ctx context.Context, e audit.Event) (bool, error) {
	tc, err := tenantContext(e.TenantID)
	if err != nil {
		return false, err
	}
	if err := p.ensureTenant(ctx, tc); err != nil {
		return false, err
	}
	canonical, err := audit.CanonicalJSON(e)
	if err != nil {
		return false, fmt.Errorf("auditforge: canonicalize event %s: %w", e.EventID, err)
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

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	if _, err := tx.ExecContext(ctx, `LOCK TABLE events IN EXCLUSIVE MODE`); err != nil {
		return false, fmt.Errorf("auditforge: lock chain: %w", err)
	}

	prev := chain.Genesis
	err = tx.QueryRowContext(ctx, `SELECT hash FROM events ORDER BY seq DESC LIMIT 1`).Scan(&prev)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("auditforge: read chain head: %w", err)
	}

	result, err := tx.ExecContext(ctx,
		`INSERT INTO events (event_id, ts, event, prev_hash, hash) VALUES ($1, $2, $3::jsonb, $4, $5)
		 ON CONFLICT (event_id) DO NOTHING`,
		e.EventID, e.Timestamp, string(payload), prev, chain.Next(prev, canonical))
	if err != nil {
		return false, fmt.Errorf("auditforge: insert event %s: %w", e.EventID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("auditforge: commit: %w", err)
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

// VerifyChain replays the tenant's hash chain in insertion (seq) order, starting from its
// retention checkpoint when one exists.
func (p *Postgres) VerifyChain(ctx context.Context, tenantID string) (ChainReport, error) {
	tc, err := tenantContext(tenantID)
	if err != nil {
		return ChainReport{}, err
	}
	if err := p.ensureTenant(ctx, tc); err != nil {
		return ChainReport{}, err
	}

	conn, err := p.db.Conn(ctx)
	if err != nil {
		return ChainReport{}, err
	}
	defer conn.Close()
	if err := p.strategy.Bind(ctx, conn, tc); err != nil {
		return ChainReport{}, err
	}

	cp, err := loadCheckpoint(ctx, conn)
	if err != nil {
		return ChainReport{}, err
	}
	records, err := p.readChain(ctx, conn)
	if err != nil {
		return ChainReport{}, err
	}
	if cp.Hash != "" && !chain.VerifyCheckpoint(cp, p.secret) {
		return brokenCheckpoint(cp, records), nil
	}
	return replayFrom(checkpointStart(cp), cp.Length, records), nil
}

// readChain loads the surviving records in seq order. conn must be bound to the tenant schema.
func (p *Postgres) readChain(ctx context.Context, conn *sql.Conn) ([]chainRecord, error) {
	rows, err := conn.QueryContext(ctx, `SELECT event, prev_hash, hash FROM events ORDER BY seq`)
	if err != nil {
		return nil, fmt.Errorf("auditforge: read chain: %w", err)
	}
	defer rows.Close()

	var records []chainRecord
	for rows.Next() {
		var payload []byte
		var prev, hash string
		if err := rows.Scan(&payload, &prev, &hash); err != nil {
			return nil, fmt.Errorf("auditforge: scan chain record: %w", err)
		}
		var e audit.Event
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, fmt.Errorf("auditforge: decode chain record: %w", err)
		}
		records = append(records, chainRecord{event: e, prevHash: prev, hash: hash})
	}
	return records, rows.Err()
}

// Purge removes the leading run of records at or older than olderThan and records a signed
// checkpoint, all under an EXCLUSIVE table lock so it cannot race an Append. Records newer
// than olderThan are never removed. Because records share a seq order, every row before the
// first record newer than the cutoff is at or older than it, so the purged set is a prefix
// and the surviving tail stays a contiguous, verifiable suffix.
func (p *Postgres) Purge(ctx context.Context, tenantID string, olderThan time.Time) (PurgeResult, error) {
	tc, err := tenantContext(tenantID)
	if err != nil {
		return PurgeResult{}, err
	}
	if err := p.ensureTenant(ctx, tc); err != nil {
		return PurgeResult{}, err
	}
	cutoff := olderThan.Truncate(time.Microsecond)

	conn, err := p.db.Conn(ctx)
	if err != nil {
		return PurgeResult{}, err
	}
	defer conn.Close()
	if err := p.strategy.Bind(ctx, conn, tc); err != nil {
		return PurgeResult{}, err
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return PurgeResult{}, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	if _, err := tx.ExecContext(ctx, `LOCK TABLE events IN EXCLUSIVE MODE`); err != nil {
		return PurgeResult{}, fmt.Errorf("auditforge: lock chain: %w", err)
	}

	// The purge set is every record before the first one newer than the cutoff.
	var boundary sql.NullInt64
	if err := tx.QueryRowContext(ctx,
		`SELECT MIN(seq) FROM events WHERE ts > $1`, cutoff).Scan(&boundary); err != nil {
		return PurgeResult{}, fmt.Errorf("auditforge: find retention boundary: %w", err)
	}

	// "$1 OR seq < $2" selects every row when there is no newer record (boundary invalid),
	// otherwise the prefix strictly before the boundary.
	var count int
	var lastSeq sql.NullInt64
	if err := tx.QueryRowContext(ctx,
		`SELECT count(*), max(seq) FROM events WHERE $1 OR seq < $2`,
		!boundary.Valid, boundary.Int64).Scan(&count, &lastSeq); err != nil {
		return PurgeResult{}, fmt.Errorf("auditforge: measure purge set: %w", err)
	}
	if count == 0 {
		cp, err := loadCheckpoint(ctx, tx)
		if err != nil {
			return PurgeResult{}, err
		}
		return PurgeResult{Checkpoint: cp}, tx.Commit()
	}

	var lastHash string
	if err := tx.QueryRowContext(ctx,
		`SELECT hash FROM events WHERE seq = $1`, lastSeq.Int64).Scan(&lastHash); err != nil {
		return PurgeResult{}, fmt.Errorf("auditforge: read purge head: %w", err)
	}

	prev, err := loadCheckpoint(ctx, tx)
	if err != nil {
		return PurgeResult{}, err
	}
	cp := chain.Checkpoint{Length: prev.Length + count, Hash: lastHash, Through: cutoff}
	cp.Signature = chain.SignCheckpoint(cp, p.secret)

	if _, err := tx.ExecContext(ctx, `DELETE FROM events WHERE seq <= $1`, lastSeq.Int64); err != nil {
		return PurgeResult{}, fmt.Errorf("auditforge: delete purged events: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO chain_checkpoint (id, length, hash, through, signature) VALUES (true, $1, $2, $3, $4)
		 ON CONFLICT (id) DO UPDATE SET length = EXCLUDED.length, hash = EXCLUDED.hash,
		   through = EXCLUDED.through, signature = EXCLUDED.signature`,
		cp.Length, cp.Hash, cp.Through, cp.Signature); err != nil {
		return PurgeResult{}, fmt.Errorf("auditforge: write checkpoint: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return PurgeResult{}, fmt.Errorf("auditforge: commit purge: %w", err)
	}
	return PurgeResult{Purged: count, Checkpoint: cp}, nil
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
			seq       bigserial PRIMARY KEY,
			event_id  text UNIQUE NOT NULL,
			ts        timestamptz NOT NULL,
			event     jsonb NOT NULL,
			prev_hash text NOT NULL,
			hash      text NOT NULL
		)`, schema),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS events_ts_idx ON %q.events (ts)`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %q.chain_checkpoint (
			id        boolean PRIMARY KEY DEFAULT true,
			length    bigint NOT NULL,
			hash      text NOT NULL,
			through   timestamptz NOT NULL,
			signature text NOT NULL,
			CONSTRAINT chain_checkpoint_single_row CHECK (id)
		)`, schema),
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
