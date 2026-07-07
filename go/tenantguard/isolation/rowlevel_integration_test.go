package isolation_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"

	tenantcontext "github.com/tenantguard/tenantguard/context"
	"github.com/tenantguard/tenantguard/isolation"
)

// TestRowLevelIsolation_CrossTenant proves against real Postgres that RowLevelStrategy
// scopes a shared table to one tenant via RLS. Because superusers bypass RLS, the query
// runs under a dedicated NOSUPERUSER role set for the transaction. Skipped unless
// TENANTGUARD_POSTGRES_DSN is set.
func TestRowLevelIsolation_CrossTenant(t *testing.T) {
	dsn := os.Getenv("TENANTGUARD_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set TENANTGUARD_POSTGRES_DSN to run the Postgres RLS test")
	}

	ctx := context.Background()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	// dropRole removes the role's privileges (DROP OWNED BY) before dropping it, so a
	// lingering schema/table grant cannot block cleanup. Guarded for non-existence.
	const dropRole = `DO $$ BEGIN
	IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'tg_rls_role') THEN
		DROP OWNED BY tg_rls_role;
		DROP ROLE tg_rls_role;
	END IF;
END $$`

	setup := []string{
		"DROP TABLE IF EXISTS rls_notes",
		dropRole,
		"CREATE TABLE rls_notes (tenant_id text NOT NULL, body text NOT NULL)",
		"ALTER TABLE rls_notes ENABLE ROW LEVEL SECURITY",
		"DROP POLICY IF EXISTS tenant_isolation ON rls_notes",
		"CREATE POLICY tenant_isolation ON rls_notes USING (tenant_id = current_setting('app.tenant_id', true))",
		"INSERT INTO rls_notes (tenant_id, body) VALUES ('acme', 'acme-note'), ('beta', 'beta-note')",
		"CREATE ROLE tg_rls_role NOSUPERUSER",
		"GRANT USAGE ON SCHEMA public TO tg_rls_role",
		"GRANT SELECT ON rls_notes TO tg_rls_role",
	}
	for _, q := range setup {
		exec(t, db, q)
	}
	defer func() {
		exec(t, db, dropRole)
		exec(t, db, "DROP TABLE IF EXISTS rls_notes")
	}()

	strategy := isolation.RowLevelStrategy{}

	if got := rlsNotesFor(t, ctx, db, strategy, "acme"); len(got) != 1 || got[0] != "acme-note" {
		t.Fatalf("tenant acme saw %v, want [acme-note]", got)
	}
	if got := rlsNotesFor(t, ctx, db, strategy, "beta"); len(got) != 1 || got[0] != "beta-note" {
		t.Fatalf("tenant beta saw %v, want [beta-note]", got)
	}
}

// rlsNotesFor runs, in one transaction under the non-superuser role, Bind + an
// unfiltered SELECT so the visible rows depend entirely on the RLS session parameter.
func rlsNotesFor(t *testing.T, ctx context.Context, db *sql.DB, s isolation.RowLevelStrategy, tenantID string) []string {
	t.Helper()
	tc, err := tenantcontext.New(tenantID, "", "", nil)
	if err != nil {
		t.Fatalf("New(%q): %v", tenantID, err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "SET LOCAL ROLE tg_rls_role"); err != nil {
		t.Fatalf("set role: %v", err)
	}
	if err := s.Bind(ctx, tx, tc); err != nil {
		t.Fatalf("bind %q: %v", tenantID, err)
	}

	rows, err := tx.QueryContext(ctx, "SELECT body FROM rls_notes ORDER BY body")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var b string
		if err := rows.Scan(&b); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return out
}
