package resolver

import (
	"net"
	"net/http"
	"strings"

	tenantcontext "github.com/tenantguard/tenantguard/context"
)

// SubdomainResolver derives the tenant from the left-most label of the request host,
// e.g. "acme.example.com" -> "acme" when BaseDomain is "example.com".
type SubdomainResolver struct {
	// BaseDomain is the shared apex, e.g. "example.com". Required.
	BaseDomain string
	// Ignore lists subdomain labels that are not tenants (defaults to {"www"} when nil).
	Ignore []string
}

// Resolve returns the tenant subdomain, or ErrNoTenant when the host is the apex, an
// ignored label, or does not sit under BaseDomain.
func (sr SubdomainResolver) Resolve(r *http.Request) (tenantcontext.TenantContext, error) {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))

	suffix := "." + strings.ToLower(sr.BaseDomain)
	if sr.BaseDomain == "" || !strings.HasSuffix(host, suffix) {
		return tenantcontext.TenantContext{}, ErrNoTenant
	}

	label, _, _ := strings.Cut(strings.TrimSuffix(host, suffix), ".")
	if label == "" || sr.ignored(label) {
		return tenantcontext.TenantContext{}, ErrNoTenant
	}
	return tenantcontext.New(label, "", traceIDFromRequest(r), nil)
}

func (sr SubdomainResolver) ignored(label string) bool {
	ignore := sr.Ignore
	if ignore == nil {
		ignore = []string{"www"}
	}
	for _, i := range ignore {
		if strings.EqualFold(i, label) {
			return true
		}
	}
	return false
}
