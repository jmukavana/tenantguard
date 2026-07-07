// Command auditforged runs the AuditForge ingestion service (ecosystem/auditforge/spec.md).
// This skeleton uses the in-memory store; the Postgres, schema-isolated backend is the next
// step. Configuration comes from the environment:
//
//	AUDITFORGE_ADDR                listen address (default ":8080")
//	AUDITFORGE_KEY_ID              the single signing key's id (default "default")
//	AUDITFORGE_SECRET              that key's shared secret (required)
//	AUDITFORGE_CHECKPOINT_SECRET   key for signing retention checkpoints (default: the secret)
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/tenantguard/auditforge/ingest"
	"github.com/tenantguard/auditforge/keyring"
	"github.com/tenantguard/auditforge/store"
)

func main() {
	secret := os.Getenv("AUDITFORGE_SECRET")
	if secret == "" {
		log.Fatal("AUDITFORGE_SECRET is required")
	}
	keyID := envOr("AUDITFORGE_KEY_ID", "default")
	addr := envOr("AUDITFORGE_ADDR", ":8080")
	checkpointSecret := []byte(envOr("AUDITFORGE_CHECKPOINT_SECRET", secret))

	service := ingest.Service{
		Keys:  keyring.Map{keyID: []byte(secret)},
		Store: store.NewMemory(checkpointSecret),
	}

	log.Printf("auditforge: listening on %s (in-memory store, key %q)", addr, keyID)
	if err := http.ListenAndServe(addr, service.Handler()); err != nil {
		log.Fatalf("auditforge: %v", err)
	}
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
