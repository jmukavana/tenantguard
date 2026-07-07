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

// TestSchemaIsolation_CrossTenant is the Phase 1 gate: it proves against a real Postgres
// that binding tenant A's schema makes tenant B's rows unreachable through an
// unqualified query. It is skipped unless TENANTGUARD_POSTGRES_DSN is set, e.g.
//
//	TENANTGUARD_POSTGRES_DSN='postgres://postgres:tg@localhost:55432/tenantguard?sslmode=disable' \
//	    go test ./isolation/ -run CrossTenant -v
func TestSchemaIsolation_CrossTenant(t *testing.T) {
	dsn := os.Getenv("TENANTGUARD_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set TENANTGUARD_POSTGRES_DSN to run the Postgres isolation test")
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

	// Fresh per-tenant schemas each holding a widgets table with distinct data.
	for _, tenant := range []struct{ schema, widget string }{
		{"tenant_acme", "acme-widget"},
		{"tenant_beta", "beta-widget"},
	} {
		exec(t, db, "DROP SCHEMA IF EXISTS "+tenant.schema+" CASCADE")
		exec(t, db, "CREATE SCHEMA "+tenant.schema)
		exec(t, db, "CREATE TABLE "+tenant.schema+".widgets (name text)")
		exec(t, db, "INSERT INTO "+tenant.schema+".widgets (name) VALUES ($1)", tenant.widget)
	}
	// Registered after `defer db.Close()`, so LIFO runs these drops before the close.
	defer func() {
		exec(t, db, "DROP SCHEMA IF EXISTS tenant_acme CASCADE")
		exec(t, db, "DROP SCHEMA IF EXISTS tenant_beta CASCADE")
	}()

	strategy := isolation.SchemaStrategy{Prefix: "tenant_"}

	// A connection bound to tenant "acme" must see only acme's widget via an
	// unqualified query — and never beta's.
	names := widgetsFor(t, ctx, db, strategy, "acme")
	if len(names) != 1 || names[0] != "acme-widget" {
		t.Fatalf("tenant acme saw %v, want [acme-widget]", names)
	}
	for _, n := range names {
		if n == "beta-widget" {
			t.Fatal("TENANT ISOLATION BREACH: acme connection read beta's data")
		}
	}

	// The same query bound to "beta" sees only beta's widget.
	if names := widgetsFor(t, ctx, db, strategy, "beta"); len(names) != 1 || names[0] != "beta-widget" {
		t.Fatalf("tenant beta saw %v, want [beta-widget]", names)
	}
}

// widgetsFor binds a dedicated connection to the tenant's schema, then runs an
// unqualified SELECT so the result depends entirely on search_path.
func widgetsFor(t *testing.T, ctx context.Context, db *sql.DB, s isolation.SchemaStrategy, tenantID string) []string {
	t.Helper()
	tc, err := tenantcontext.New(tenantID, "", "", nil)
	if err != nil {
		t.Fatalf("New(%q): %v", tenantID, err)
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer conn.Close()
	if err := s.Bind(ctx, conn, tc); err != nil {
		t.Fatalf("bind %q: %v", tenantID, err)
	}
	rows, err := conn.QueryContext(ctx, "SELECT name FROM widgets ORDER BY name")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		names = append(names, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return names
}

func exec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}
