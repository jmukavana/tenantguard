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

### Phase B — Signing clients (in progress)
Turn the deferred Ring-5 `AuditHook` stub into a real signer in each TenantGuard language:
- a `sign(event, secret) -> hex` function proven against `auditforge-signing.json` (add a
  case to each language's conformance harness — a pure, DB-free, network-free vector test).
  **Done in all four languages** — each exposes `sign`/`verify`/`canonicalJson` next to its
  `TenantEvent` (Go `audit.Sign`; Java `io.tenantguard.audit.Signing`; Python
  `tenantguard.audit.sign`; TS `sign` in `@tenantguard/core/audit`), and each conformance
  harness proves both golden vectors (canonical bytes and HMAC checked separately). Every
  signer is stdlib-only (HMAC-SHA256 + hex).
- an `AuditForgeHook implements AuditHook` that signs each event and POSTs an `IngestRequest`
  to `/v1/events`, with retry/backoff and the `keyId`. **Done in all four languages** — Go
  `auditforge.Hook`, Java `auditforge.AuditForgeHook` (`java.net.http`), Python
  `tenantguard.audit.auditforge.AuditForgeHook` (`urllib`), TS `AuditForgeHook` (`fetch`).
  Each signs, POSTs a one-event batch, treats a `duplicate` ack as idempotent success and a
  `rejected`/unacknowledged event as an error, and retries 5xx + network errors (not 4xx) with
  exponential backoff. Every client is stdlib-only (HMAC + an HTTP client) and is tested
  against an in-process fake AuditForge that verifies the signature it receives.

### Phase C — Ingestion service (Go) — in progress
The service lives in its own module, `github.com/tenantguard/auditforge` under
`ecosystem/auditforge/`, importing TenantGuard (local `replace`). Structure:
`keyring` (keyId→secret), `store` (the `Store` seam + a `Memory` implementation), `ingest`
(the `Service` + `net/http` handlers), and `cmd/auditforged`.
- `POST /v1/events`: **done** — recompute the canonical form and verify each signature with
  TenantGuard's `audit.Verify` (byte-identical to every emitter), reject `unknown_key` /
  `malformed_event` / `invalid_signature` per event (batch still 200), apply idempotency by
  `eventId`, and append via the `Store`. Proven against the `auditforge-signing.json` golden
  vectors from the verifier's side, plus tamper/unknown-key/malformed/redelivery cases.
- `GET /v1/events`: **done** — tenant resolved from the request by TenantGuard's
  `HeaderResolver` (dogfooding), results isolated to that tenant; `from`/`to` RFC 3339 bounds.
  End-to-end `httptest` proves tenant A cannot read tenant B's events.
- **Remaining gate:** swap the `Memory` store for a Postgres, schema-isolated backend using
  TenantGuard's `SchemaStrategy`, proven by a real-Postgres integration test (the
  testcontainers pattern already used) — one event stored once, idempotent on redelivery,
  isolation enforced at the database.

### Phase D — Tamper-evidence & retention
- Per-tenant hash chaining (`hash = SHA-256(prevHash || canonical)`), a chain-verify endpoint,
  and configurable retention windows. Post-v1: Ed25519 detached signatures (same canonical
  form, asymmetric algorithm) for non-repudiation.

## First three concrete steps
1. ~~Add a `sign()` + `auditforge-signing.json` conformance test to the Go audit package
   (proves the golden vectors against real code).~~ **Done** — `audit.Sign`/`Verify`/
   `CanonicalJSON` + `TestAuditForgeSigningFixtures`.
2. ~~Implement `AuditForgeHook` (Go) that signs + POSTs an `IngestRequest`.~~ **Done** —
   `auditforge.Hook`, tested against a fake AuditForge that verifies each signature.
3. ~~Stand up the Go ingestion service skeleton with signature verification, tested against
   the same golden vectors from the verifier's side.~~ **Done** — `ecosystem/auditforge/`
   (`ingest.Service` + `Memory` store); next is the Postgres, schema-isolated backend.
