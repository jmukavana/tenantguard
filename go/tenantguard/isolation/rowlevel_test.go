package isolation

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

// argRecordingExecer captures both the SQL and its bound arguments.
type argRecordingExecer struct {
	query string
	args  []any
}

func (r *argRecordingExecer) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	r.query = query
	r.args = args
	return nil, nil
}

func TestRowLevel_Bind_UsesBoundParams(t *testing.T) {
	rec := &argRecordingExecer{}
	if err := (RowLevelStrategy{}).Bind(context.Background(), rec, mustCtx(t, "acme")); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if rec.query != "SELECT set_config($1, $2, true)" {
		t.Fatalf("query = %q", rec.query)
	}
	if len(rec.args) != 2 || rec.args[0] != DefaultRLSSetting || rec.args[1] != "acme" {
		t.Fatalf("args = %#v, want [%q acme]", rec.args, DefaultRLSSetting)
	}
}

func TestRowLevel_Bind_CustomSetting(t *testing.T) {
	rec := &argRecordingExecer{}
	if err := (RowLevelStrategy{Setting: "app.org"}).Bind(context.Background(), rec, mustCtx(t, "acme")); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if rec.args[0] != "app.org" {
		t.Fatalf("setting arg = %v, want app.org", rec.args[0])
	}
}

// A malicious tenant id is passed as data, never interpolated, so it can only ever be a
// harmless bound value — Bind still succeeds and the driver escapes it.
func TestRowLevel_Bind_InjectionIsInert(t *testing.T) {
	rec := &argRecordingExecer{}
	evil := "acme'); DROP TABLE users; --"
	tc := mustCtx(t, evil)
	if err := (RowLevelStrategy{}).Bind(context.Background(), rec, tc); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if rec.args[1] != evil {
		t.Fatalf("payload should be an untouched bound arg, got %v", rec.args[1])
	}
}

func TestRowLevel_Bind_RejectsEmptyTenant(t *testing.T) {
	rec := &argRecordingExecer{}
	if err := (RowLevelStrategy{}).Bind(context.Background(), rec, mustCtx2(t, "")); !errors.Is(err, ErrUnsafeTenantID) {
		t.Fatalf("empty tenant: got %v, want ErrUnsafeTenantID", err)
	}
	if rec.query != "" {
		t.Fatal("no SQL should run for an empty tenant id")
	}
}
