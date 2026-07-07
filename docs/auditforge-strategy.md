# AuditForge — Implementation Strategy (fresh start)

Status: proposal. AuditForge is the audit event store TenantGuard publishes to
(`ecosystem/auditforge/spec.md`). This describes how to build it, in what order, reusing
the same fixture-driven, spec-first method that produced the four TenantGuard cores.

## Guiding principles

1. **Same contract discipline.** The signing format and the ingest/ack shapes are locked by
   shared fixtures (`testkit/compliance-tests/fixtures/auditforge-signing.json`, and the
   `ecosystem/auditforge/schemas/*.json`) before implementations chase each other.
2. **Signing is the keystone.** The one thing that MUST be identical everywhere is the bytes
   that get signed. That is already frozen: canonical form = `TenantEvent.toJson()` of the
   unsigned event; signature = hex HMAC-SHA256. Golden vectors exist and are verified
   (cross-checked with `openssl`). Build outward from there.
3. **AuditForge dogfoods TenantGuard.** Its own multitenancy (schema isolation, tenant
   resolution, and — recursively — its own audit trail) uses the TenantGuard libraries. This
   is both a design win and the best possible integration test of TenantGuard.
4. **Go service first**, then the per-language signing clients, mirroring the TenantGuard
   port order.

## Phased plan

### Phase A — Contract (DONE)
- `ecosystem/auditforge/spec.md`: principles, signing contract, ingest/query API, storage.
- `ecosystem/auditforge/schemas/`: `IngestRequest`, `IngestAck` (IngestRequest `$ref`s the
  existing `TenantEvent` schema).
- `testkit/compliance-tests/fixtures/auditforge-signing.json`: HMAC-SHA256 golden vectors
  over the canonical event form, independently verified.

### Phase B — Signing clients (next)
Turn the deferred Ring-5 `AuditHook` stub into a real signer in each TenantGuard language:
- a `sign(event, secret) -> hex` function proven against `auditforge-signing.json` (add a
  case to each language's conformance harness — a pure, DB-free, network-free vector test);
- an `AuditForgeHook implements AuditHook` that signs each event and POSTs an `IngestRequest`
  to `/v1/events`, with retry/backoff and the `keyId` header.
- Start in Go (reference), then Java/Python/TS. Each only needs an HMAC primitive + an HTTP
  client — both in every standard library.

### Phase C — Ingestion service (Go)
- `POST /v1/events`: verify each event's signature (recompute canonical form, HMAC, constant
  -time compare), apply idempotency by `eventId`, append to the tenant's schema-isolated log.
  Return an `IngestAck`. Uses TenantGuard's `SchemaStrategy` for per-tenant storage.
- `GET /v1/events`: tenant-scoped query via TenantGuard resolution + isolation.
- **Gate:** a real-Postgres integration test (the testcontainers pattern already used) showing
  a valid batch is stored once (idempotent on redelivery), a tampered event is `rejected`,
  and tenant A cannot read tenant B's events.

### Phase D — Tamper-evidence & retention
- Per-tenant hash chaining (`hash = SHA-256(prevHash || canonical)`), a chain-verify endpoint,
  and configurable retention windows. Post-v1: Ed25519 detached signatures (same canonical
  form, asymmetric algorithm) for non-repudiation.

## First three concrete steps
1. Add a `sign()` + `auditforge-signing.json` conformance test to the Go audit package
   (proves the golden vectors against real code).
2. Implement `AuditForgeHook` (Go) that signs + POSTs an `IngestRequest`.
3. Stand up the Go ingestion service skeleton with signature verification, tested against the
   same golden vectors from the verifier's side.
