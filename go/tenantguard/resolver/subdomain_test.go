package resolver

import (
	"errors"
	"net/http/httptest"
	"testing"
)

func TestSubdomainResolver(t *testing.T) {
	sr := SubdomainResolver{BaseDomain: "example.com"}

	cases := []struct {
		host   string
		want   string // "" means expect ErrNoTenant
	}{
		{"acme.example.com", "acme"},
		{"acme.example.com:8443", "acme"},
		{"ACME.Example.Com", "acme"},
		{"acme.eu.example.com", "acme"}, // left-most label wins
		{"example.com", ""},             // apex, no tenant
		{"www.example.com", ""},         // ignored label
		{"acme.other.com", ""},          // wrong base domain
	}

	for _, c := range cases {
		r := httptest.NewRequest("GET", "/", nil)
		r.Host = c.host
		tc, err := sr.Resolve(r)
		if c.want == "" {
			if !errors.Is(err, ErrNoTenant) {
				t.Errorf("host %q: got tenant %q err %v, want ErrNoTenant", c.host, tc.TenantID(), err)
			}
			continue
		}
		if err != nil || tc.TenantID() != c.want {
			t.Errorf("host %q: got %q err %v, want %q", c.host, tc.TenantID(), err, c.want)
		}
	}
}

func TestSubdomainResolver_CustomIgnore(t *testing.T) {
	sr := SubdomainResolver{BaseDomain: "example.com", Ignore: []string{"admin", "www"}}
	r := httptest.NewRequest("GET", "/", nil)
	r.Host = "admin.example.com"
	if _, err := sr.Resolve(r); !errors.Is(err, ErrNoTenant) {
		t.Fatalf("admin should be ignored, got %v", err)
	}
}
