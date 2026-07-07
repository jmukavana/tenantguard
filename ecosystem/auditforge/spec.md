# AuditForge Core Specification (v0.1, draft)

Language-neutral specification for **AuditForge** — the regulatory-grade, tamper-evident
audit event store that TenantGuard publishes to. This is the sibling system referenced
throughout `core/spec.md`; TenantGuard emits `TenantEvent`s, AuditForge ingests, verifies,
and retains them for compliance (CBK, IRA, Data Protection Act).

This spec is the single source of truth for AuditForge. Like TenantGuard, cross-language
behavior is locked by shared fixtures (`testkit/compliance-tests/fixtures/auditforge-*.json`).

## Core principles

- **Tamper-evident**: every event is signed by the emitter; AuditForge verifies the
  signature on ingest and rejects anything that does not verify.
- **Append-only**: events are never updated or deleted within their retention window.
- **Idempotent**: re-delivering an event (same `eventId`) is safe and acknowledged as a
  duplicate, never double-stored.
- **Tenant-isolated**: AuditForge is itself multi-tenant and **dogfoods TenantGuard** — an
  event for tenant A is never returned to a query scoped to tenant B.
- **Regulatory-first**: retention and immutability windows are configurable per regulator.

## The signing contract (interop-critical)

An emitter signs each event so AuditForge (and auditors) can prove it was not altered.
Every TenantGuard language binding MUST produce byte-identical signatures.

1. **Canonical form** — the event serialized by the shared `TenantEvent.toJson()` rules
   (declaration-order keys, empty optionals omitted, metadata keys sorted, compact
   separators, UTF-8, no ASCII escaping) **with the `signature` field absent**. Because
   `toJson()` already omits an empty `signature`, the canonical form is exactly
   `toJson()` of the otherwise-complete but unsigned event.
2. **Signature** — `lowercasehex( HMAC-SHA256( secret, utf8(canonicalForm) ) )`, where
   `secret` is a shared symmetric key identified by a `keyId`. (Asymmetric signing is a
   post-v1 option; the wire field is algorithm-agnostic.)
3. The signature is placed in the event's `signature` field for transport.

Golden vectors: `testkit/compliance-tests/fixtures/auditforge-signing.json`.

> HMAC (symmetric) is the v0.1 baseline because it is trivially reproducible across all
> four languages and needs no key distribution beyond the shared secret. Ed25519 detached
> signatures are the intended v1 upgrade for non-repudiation; the canonical form does not
> change, only the algorithm over it.

## Ingestion API

### `POST /v1/events`

Body: an **IngestRequest** (`schemas/IngestRequest.schema.json`) — a `keyId` plus a batch
of signed `TenantEvent`s. AuditForge, for each event:

1. recomputes the canonical form and verifies `signature` against the key for `keyId`;
2. checks `eventId` for idempotency;
3. appends it to the tenant's immutable log.

Returns an **IngestAck** (`schemas/IngestAck.schema.json`) partitioning the batch into
`accepted`, `duplicate`, and `rejected` (with a reason per rejection). A batch with some
rejections still returns `200`; transport/auth failure returns `4xx`. An event that fails
signature verification is `rejected`, never stored.

### `GET /v1/events?tenantId=&from=&to=`

Tenant-scoped compliance query. The caller's tenant is resolved by TenantGuard; results are
isolated to that tenant. `from`/`to` are RFC 3339 bounds on `timestamp`.

## Storage model

- **Append-only log per tenant.** Schema-based isolation (TenantGuard's default) maps each
  tenant to its own AuditForge schema.
- **Hash chaining (tamper-evidence).** Each stored record carries
  `prevHash` and `hash = SHA-256(prevHash || canonicalForm)`, forming a per-tenant chain an
  auditor can replay to detect any insertion, deletion, or edit. (v1 detail; the ingest
  contract above does not depend on it.)
- **Retention.** Configurable minimum retention per regulator; deletion before expiry is
  prohibited.

## Security requirements

- Reject any event whose signature does not verify.
- Never return one tenant's events to another (enforced via TenantGuard).
- Treat `keyId` as untrusted input: an unknown or revoked key rejects the batch.
- Transport is TLS; the signature protects integrity end-to-end regardless.

## Roadmap pointer

See `docs/auditforge-strategy.md` for the build-out plan (Go ingestion service first, then
the signing clients in each TenantGuard language, all against the shared fixtures).
