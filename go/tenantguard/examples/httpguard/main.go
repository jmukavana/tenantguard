// Command httpguard is a minimal end-to-end demo of the Phase 1 vertical slice:
// a header-resolved tenant, an audit event per request, and a handler that reads the
// immutable TenantContext the middleware bound.
//
//	go run ./examples/httpguard
//	curl -H 'X-Tenant-ID: acme' localhost:8080/whoami   # 200, "tenant: acme"
//	curl localhost:8080/whoami                           # 401, tenant not identified
package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"

	"github.com/tenantguard/tenantguard/audit"
	"github.com/tenantguard/tenantguard/middleware"
	"github.com/tenantguard/tenantguard/resolver"
)

func main() {
	guard := middleware.Guard{
		Resolver: resolver.HeaderResolver{},
		Audit:    audit.LoggingHook{Logger: slog.Default()},
	}

	whoami := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc, _ := middleware.FromContext(r.Context())
		fmt.Fprintf(w, "tenant: %s\n", tc.TenantID())
	})

	http.Handle("/whoami", guard.Handler(whoami))
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
