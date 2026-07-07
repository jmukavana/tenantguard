package isolation

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	tenantcontext "github.com/tenantguard/tenantguard/context"
)

// recordingExecer captures the SQL that Bind would run, without a database.
type recordingExecer struct{ queries []string }

func (r *recordingExecer) ExecContext(_ context.Context, query string, _ ...any) (sql.Result, error) {
	r.queries = append(r.queries, query)
	return nil, nil
}

func mustCtx(t *testing.T, id string) tenantcontext.TenantContext {
	t.Helper()
	tc, err := tenantcontext.New(id, "", "", nil)
	if err != nil {
		t.Fatalf("New(%q): %v", id, err)
	}
	return tc
}

func TestSchemaName(t *testing.T) {
	s := SchemaStrategy{Prefix: "tenant_"}
	got, err := s.SchemaName(mustCtx(t, "acme"))
	if err != nil || got != "tenant_acme" {
		t.Fatalf("SchemaName = %q, %v; want tenant_acme, nil", got, err)
	}
}

func TestSchemaName_RejectsInjection(t *testing.T) {
	s := SchemaStrategy{Prefix: "tenant_"}
	for _, bad := range []string{
		`acme"; DROP SCHEMA public CASCADE; --`,
		"acme public",
		"acme-1",
		"acme;",
		"",
	} {
		if _, err := s.SchemaName(mustCtx2(t, bad)); !errors.Is(err, ErrUnsafeTenantID) {
			t.Errorf("SchemaName(%q): got %v, want ErrUnsafeTenantID", bad, err)
		}
	}
}

// mustCtx2 builds a context for ids that New accepts (non-empty) and, for the empty
// case, exercises SchemaName's own guard against a zero context.
func mustCtx2(t *testing.T, id string) tenantcontext.TenantContext {
	t.Helper()
	if id == "" {
		return tenantcontext.TenantContext{} // zero value: TenantID() == ""
	}
	tc, err := tenantcontext.New(id, "", "", nil)
	if err != nil {
		t.Fatalf("New(%q): %v", id, err)
	}
	return tc
}

func TestBind_SetsSearchPath(t *testing.T) {
	rec := &recordingExecer{}
	if err := (SchemaStrategy{Prefix: "tenant_"}).Bind(context.Background(), rec, mustCtx(t, "acme")); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if len(rec.queries) != 1 || rec.queries[0] != `SET search_path TO "tenant_acme"` {
		t.Fatalf("unexpected queries: %#v", rec.queries)
	}
}

func TestBind_RejectsUnsafeBeforeExec(t *testing.T) {
	rec := &recordingExecer{}
	err := (SchemaStrategy{Prefix: "tenant_"}).Bind(context.Background(), rec, mustCtx2(t, "a b"))
	if !errors.Is(err, ErrUnsafeTenantID) {
		t.Fatalf("Bind unsafe: got %v, want ErrUnsafeTenantID", err)
	}
	if len(rec.queries) != 0 {
		t.Fatalf("no SQL should run for an unsafe tenant id, got %#v", rec.queries)
	}
}
