// Package conformance runs the language-neutral compliance fixtures
// (testkit/compliance-tests/fixtures) against the Go reference implementation. It is the
// executable definition of "conformant" and the template every other language mirrors.
package conformance

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"go.opentelemetry.io/otel/baggage"
	otelprop "go.opentelemetry.io/otel/propagation"

	"github.com/tenantguard/tenantguard/audit"
	tenantcontext "github.com/tenantguard/tenantguard/context"
	"github.com/tenantguard/tenantguard/isolation"
	"github.com/tenantguard/tenantguard/propagation"
	"github.com/tenantguard/tenantguard/resolver"
)

// fixturesDir is relative to this package's directory (go/tenantguard/conformance).
const fixturesDir = "../../../testkit/compliance-tests/fixtures"

func loadFixture(t *testing.T, name string, cases any) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fixturesDir, name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var file struct {
		Cases json.RawMessage `json:"cases"`
	}
	if err := json.Unmarshal(b, &file); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	if err := json.Unmarshal(file.Cases, cases); err != nil {
		t.Fatalf("parse cases in %s: %v", name, err)
	}
}

// wantErr asserts err matches the fixture's symbolic error code.
func wantErr(t *testing.T, code string, err error) {
	t.Helper()
	var target error
	switch code {
	case "empty_tenant_id":
		target = tenantcontext.ErrEmptyTenantID
	case "invalid_trace_id":
		target = tenantcontext.ErrInvalidTraceID
	case "no_tenant":
		target = resolver.ErrNoTenant
	case "unsafe_tenant_id":
		target = isolation.ErrUnsafeTenantID
	default:
		t.Fatalf("fixture uses unknown error code %q", code)
	}
	if !errors.Is(err, target) {
		t.Fatalf("expected error %v, got %v", target, err)
	}
}

func sameMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestTenantContextFixtures(t *testing.T) {
	var cases []struct {
		Name  string `json:"name"`
		Input struct {
			TenantID   string            `json:"tenantId"`
			TenantName string            `json:"tenantName"`
			TraceID    string            `json:"traceId"`
			Metadata   map[string]string `json:"metadata"`
		} `json:"input"`
		Expect struct {
			Error      string            `json:"error"`
			Valid      bool              `json:"valid"`
			TenantID   string            `json:"tenantId"`
			TenantName string            `json:"tenantName"`
			TraceID    string            `json:"traceId"`
			Metadata   map[string]string `json:"metadata"`
		} `json:"expect"`
	}
	loadFixture(t, "tenant-context.json", &cases)

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			tc, err := tenantcontext.New(c.Input.TenantID, c.Input.TenantName, c.Input.TraceID, c.Input.Metadata)
			if c.Expect.Error != "" {
				wantErr(t, c.Expect.Error, err)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.IsValid() != c.Expect.Valid {
				t.Errorf("IsValid = %v, want %v", tc.IsValid(), c.Expect.Valid)
			}
			if tc.TenantID() != c.Expect.TenantID || tc.TenantName() != c.Expect.TenantName || tc.TraceID() != c.Expect.TraceID {
				t.Errorf("scalars = (%q,%q,%q), want (%q,%q,%q)",
					tc.TenantID(), tc.TenantName(), tc.TraceID(),
					c.Expect.TenantID, c.Expect.TenantName, c.Expect.TraceID)
			}
			if !sameMap(tc.Metadata(), c.Expect.Metadata) {
				t.Errorf("metadata = %v, want %v", tc.Metadata(), c.Expect.Metadata)
			}
		})
	}
}

func TestHeaderResolverFixtures(t *testing.T) {
	var cases []struct {
		Name    string            `json:"name"`
		Header  string            `json:"header"`
		Headers map[string]string `json:"headers"`
		Expect  struct {
			Error    string `json:"error"`
			TenantID string `json:"tenantId"`
			TraceID  string `json:"traceId"`
		} `json:"expect"`
	}
	loadFixture(t, "resolver-header.json", &cases)

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			for k, v := range c.Headers {
				req.Header.Set(k, v)
			}
			tc, err := resolver.HeaderResolver{Header: c.Header}.Resolve(req)
			if c.Expect.Error != "" {
				wantErr(t, c.Expect.Error, err)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.TenantID() != c.Expect.TenantID || tc.TraceID() != c.Expect.TraceID {
				t.Errorf("got (%q,%q), want (%q,%q)", tc.TenantID(), tc.TraceID(), c.Expect.TenantID, c.Expect.TraceID)
			}
		})
	}
}

func TestSubdomainResolverFixtures(t *testing.T) {
	var cases []struct {
		Name       string   `json:"name"`
		BaseDomain string   `json:"baseDomain"`
		Host       string   `json:"host"`
		Ignore     []string `json:"ignore"`
		Expect     struct {
			Error    string `json:"error"`
			TenantID string `json:"tenantId"`
		} `json:"expect"`
	}
	loadFixture(t, "resolver-subdomain.json", &cases)

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Host = c.Host
			tc, err := resolver.SubdomainResolver{BaseDomain: c.BaseDomain, Ignore: c.Ignore}.Resolve(req)
			if c.Expect.Error != "" {
				wantErr(t, c.Expect.Error, err)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.TenantID() != c.Expect.TenantID {
				t.Errorf("tenantId = %q, want %q", tc.TenantID(), c.Expect.TenantID)
			}
		})
	}
}

func TestSchemaIsolationFixtures(t *testing.T) {
	var cases []struct {
		Name     string `json:"name"`
		Prefix   string `json:"prefix"`
		TenantID string `json:"tenantId"`
		Expect   struct {
			Error  string `json:"error"`
			Schema string `json:"schema"`
		} `json:"expect"`
	}
	loadFixture(t, "isolation-schema.json", &cases)

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			tc, err := tenantcontext.New(c.TenantID, "", "", nil)
			if err != nil {
				t.Fatalf("build context: %v", err)
			}
			schema, err := isolation.SchemaStrategy{Prefix: c.Prefix}.SchemaName(tc)
			if c.Expect.Error != "" {
				wantErr(t, c.Expect.Error, err)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if schema != c.Expect.Schema {
				t.Errorf("schema = %q, want %q", schema, c.Expect.Schema)
			}
		})
	}
}

func TestTenantEventFixtures(t *testing.T) {
	var cases []struct {
		Name   string          `json:"name"`
		Event  json.RawMessage `json:"event"`
		Expect struct {
			RequiredKeys []string          `json:"requiredKeys"`
			AbsentKeys   []string          `json:"absentKeys"`
			Values       map[string]string `json:"values"`
		} `json:"expect"`
	}
	loadFixture(t, "tenant-event.json", &cases)

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			var e audit.Event
			if err := json.Unmarshal(c.Event, &e); err != nil {
				t.Fatalf("decode event: %v", err)
			}
			out, err := json.Marshal(e)
			if err != nil {
				t.Fatalf("encode event: %v", err)
			}
			var m map[string]json.RawMessage
			if err := json.Unmarshal(out, &m); err != nil {
				t.Fatalf("re-decode event: %v", err)
			}
			for _, k := range c.Expect.RequiredKeys {
				if _, ok := m[k]; !ok {
					t.Errorf("required key %q missing from %s", k, out)
				}
			}
			for _, k := range c.Expect.AbsentKeys {
				if _, ok := m[k]; ok {
					t.Errorf("key %q should be omitted but is present in %s", k, out)
				}
			}
			for k, want := range c.Expect.Values {
				var got string
				if err := json.Unmarshal(m[k], &got); err != nil {
					t.Fatalf("value for %q not a string: %v", k, err)
				}
				if got != want {
					t.Errorf("%q = %q, want %q", k, got, want)
				}
			}
		})
	}
}

func TestAuditForgeSigningFixtures(t *testing.T) {
	var file struct {
		Secret string `json:"secret"`
		Cases  []struct {
			Name      string      `json:"name"`
			Event     audit.Event `json:"event"`
			Canonical string      `json:"canonical"`
			Signature string      `json:"signature"`
		} `json:"cases"`
	}
	b, err := os.ReadFile(filepath.Join(fixturesDir, "auditforge-signing.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := json.Unmarshal(b, &file); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	secret := []byte(file.Secret)
	for _, c := range file.Cases {
		t.Run(c.Name, func(t *testing.T) {
			// The canonical bytes are checked separately from the HMAC so a mismatch
			// localizes to serialization vs. the signing primitive.
			canonical, err := audit.CanonicalJSON(c.Event)
			if err != nil {
				t.Fatalf("CanonicalJSON: %v", err)
			}
			if string(canonical) != c.Canonical {
				t.Errorf("canonical form mismatch:\n got %s\nwant %s", canonical, c.Canonical)
			}

			sig, err := audit.Sign(c.Event, secret)
			if err != nil {
				t.Fatalf("Sign: %v", err)
			}
			if sig != c.Signature {
				t.Errorf("signature = %s, want %s", sig, c.Signature)
			}

			ok, err := audit.Verify(c.Event, secret, c.Signature)
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if !ok {
				t.Error("Verify rejected the golden signature")
			}

			// A tampered secret must not verify.
			if ok, _ := audit.Verify(c.Event, []byte("wrong-secret"), c.Signature); ok {
				t.Error("Verify accepted a signature under the wrong secret")
			}
		})
	}
}

func TestPropagationBaggageFixtures(t *testing.T) {
	var cases []struct {
		Name    string `json:"name"`
		Context struct {
			TenantID   string            `json:"tenantId"`
			TenantName string            `json:"tenantName"`
			TraceID    string            `json:"traceId"`
			Metadata   map[string]string `json:"metadata"`
		} `json:"context"`
		ExpectMembers map[string]string `json:"expectMembers"`
	}
	loadFixture(t, "propagation-baggage.json", &cases)

	p := propagation.New()
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			tc, err := tenantcontext.New(c.Context.TenantID, c.Context.TenantName, c.Context.TraceID, c.Context.Metadata)
			if err != nil {
				t.Fatalf("build context: %v", err)
			}

			carrier := otelprop.MapCarrier{}
			if err := p.Inject(context.Background(), tc, carrier); err != nil {
				t.Fatalf("Inject: %v", err)
			}

			bag, err := baggage.Parse(carrier.Get("baggage"))
			if err != nil {
				t.Fatalf("parse baggage %q: %v", carrier.Get("baggage"), err)
			}

			// Every expected member must be present with the exact value...
			for k, want := range c.ExpectMembers {
				if got := bag.Member(k).Value(); got != want {
					t.Errorf("member %q = %q, want %q", k, got, want)
				}
			}
			// ...and no unexpected tg.* member may leak.
			for _, m := range bag.Members() {
				if len(m.Key()) >= 3 && m.Key()[:3] == "tg." {
					if _, ok := c.ExpectMembers[m.Key()]; !ok {
						t.Errorf("unexpected tenant member %q=%q", m.Key(), m.Value())
					}
				}
			}

			// Round-trip: extracting the carrier reconstructs an equal context.
			got, ok, err := p.Extract(carrier)
			if err != nil || !ok {
				t.Fatalf("Extract: ok=%v err=%v", ok, err)
			}
			if got.TenantID() != tc.TenantID() || got.TenantName() != tc.TenantName() ||
				got.TraceID() != tc.TraceID() || !sameMap(got.Metadata(), tc.Metadata()) {
				t.Errorf("round-trip mismatch: got %+v", got)
			}
		})
	}
}
