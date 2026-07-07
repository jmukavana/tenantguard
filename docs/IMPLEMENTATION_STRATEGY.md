# TenantGuard — Implementation Strategy (fresh start)

Status: proposal. This describes *how* to build TenantGuard out of the current empty
scaffold, in what order, and with what dependencies. The behavioral source of truth remains
`core/spec.md`.

## 1. Guiding principles

1. **One language first, all the way through.** Do **not** build four half-implementations
   in parallel. Go is the reference (it's the only wired CI, and `ROADMAP.MD` names it for
   v1.0). Prove the entire spec in Go, then port Java → Python → TypeScript against a shared
   conformance suite.
2. **Vertical slice over horizontal layers.** The first milestone is one *working end-to-end
   request path*, not fully-built modules in isolation. A thin slice that touches every
   interface flushes out spec ambiguities early.
3. **Spec conformance is a test artifact, not a promise.** Behavior that must be identical
   across languages gets encoded as language-neutral JSON fixtures (`core/schemas/`,
   `testkit/compliance-tests/`). Every implementation runs the same fixtures.
4. **Security invariants are tests, not comments.** Immutability, per-request validation, and
   "reject tenant switch mid-request" each get an explicit adversarial test. These are the
   product; treat a gap here as a P0 bug.

## 2. Module dependency graph (build order)

Implement inner rings before outer rings — an outer ring may only depend on inner rings.

```
Ring 0  errors, TenantEvent schema, TenantContext (immutable value type)
           │
Ring 1  TenantResolver ──┐        ContextPropagator
           │             │            │
Ring 2  IsolationStrategy │        AuditHook
           │             │            │
Ring 3  Middleware  (resolver → context → propagator → isolation → audit pipeline)
           │
Ring 4  PolicyHook / compliance
           │
Ring 5  Integration adapters (HTTP frameworks, Postgres, Kafka, Redis, OTel, AuditForge)
```

`TenantContext` is the keystone — everything imports it, nothing it imports is internal.
Build and freeze it first.

## 3. Phased plan

### Phase 0 — Foundations (Go)
- Populate `go/go.mod` (module `github.com/tenantguard/tenantguard`, Go 1.23).
- `context/`: immutable `TenantContext` — unexported fields, constructor validation, no
  setters, defensive copies of the metadata map on the way in and out. `IsValid()`.
- Error types and the `TenantEvent` struct mirroring `core/schemas/TenantEvent.schema.json`
  (author that schema now if still empty).
- **Gate:** a test proving a `TenantContext` cannot be mutated after construction.

### Phase 1 — First vertical slice (Go)
Wire the smallest real path: **`X-Tenant-ID` header resolver → schema-based isolation for
Postgres → audit event to a logging AuditHook**, joined by one `net/http` middleware.
- `resolver/`: header resolver implementing `Resolve(req) (TenantContext, error)`.
- `isolation/`: schema strategy that rewrites/prefixes queries to the tenant schema (spec's
  recommended default). Start with `database/sql` + `pgx`.
- `audit/`: `AuditHook` interface + a stdout/logging implementation (real AuditForge client
  is Ring 5, deferred).
- `middleware/`: the pipeline; rejects requests with no/invalid tenant and any mid-request
  tenant switch.
- `examples/gin-example` or a `net/http` demo exercising the slice.
- **Gate:** an integration test (testcontainers-go + Postgres) showing tenant A cannot read
  tenant B's rows, and an audit event is emitted per request.

### Phase 2 — Propagation & completeness (Go)
- `propagation/`: `ContextPropagator` over W3C Baggage + OpenTelemetry Trace Context
  (`go.opentelemetry.io/otel`), covering HTTP out-calls, goroutines, and MQ headers.
- Additional resolvers (JWT claim, subdomain) and isolation strategies (Row-Level Security,
  Database-per-Tenant) behind the same interfaces.
- `policy/`: minimal `PolicyHook` (allow/deny decision object) — full engine is post-v1.0.
- **Gate:** Go reference is spec-complete; `go test ./... -v` green (matches CI).

### Phase 3 — Lock the contract
- Extract behavior into language-neutral fixtures under `testkit/compliance-tests/`:
  resolver inputs→expected context, isolation query in→query out, event in→serialized JSON.
- Build a Go harness that loads and asserts them. These fixtures become the definition of
  "conformant" for every other language.

### Phase 4 — Port Java, then Python, then TypeScript
For each: populate the build file, implement Ring 0→4 against the **same fixtures**, then add
that ecosystem's idiomatic adapters. Wire the already-present per-language CI workflow.
- Java: `pom.xml` (JDK 17+), Spring Boot starter, JPA + JDBC, `opentelemetry-java`, Jackson,
  JUnit 5, Testcontainers.
- Python: `pyproject.toml` (3.10+), FastAPI/Starlette, SQLAlchemy, `opentelemetry-python`,
  Pydantic, pytest, testcontainers-python.
- TypeScript: `package.json`, Express, `@opentelemetry/*`, Zod, Vitest.

### Phase 5 — Ecosystem edges
Real AuditForge client (signed/authenticated channel per spec §Security), Kafka/Redis
adapters, `platform/` Docker/K8s, `benchmarks/`. Driven by demand, not upfront.

## 4. Third-party dependency policy

- **Keep the core rings dependency-light.** Ring 0–2 should lean on the standard library +
  the DB driver only. Heavy deps (web frameworks, Kafka, OTel exporters) live in Ring 5
  adapter packages so a consumer pulls in only what they use.
- **One OpenTelemetry stack per language** — it is the propagation substrate, not optional.
- **Pin and converge versions** across the four `ci-*.yml` workflows so CI matches local.

## 5. Definition of done (every ring, every language)

- Conforms to `core/spec.md` and passes the shared `testkit/compliance-tests/` fixtures.
- Adversarial tests exist for: post-construction mutation, missing/invalid tenant, and
  mid-request tenant switch — all must fail closed.
- An audit event is emitted for every tenant-aware operation.
- Green in that language's `.github/workflows/ci-*.yml`.

## 6. First three concrete steps

1. `git init` (repo is not yet under version control) and commit the current scaffold.
2. Author `core/schemas/TenantEvent.schema.json` + `TenantContext.schema.json` — Phase 0
   depends on them.
3. Fill `go/go.mod` and implement the immutable `TenantContext` with its mutation-guard test.
