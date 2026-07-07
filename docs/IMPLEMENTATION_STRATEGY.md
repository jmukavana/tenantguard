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

### Phase 2 — Propagation & completeness (Go) — DONE (partial)
- `propagation/`: `ContextPropagator` carrying tenant identity as W3C Baggage members
  (`tg.*`) via `go.opentelemetry.io/otel`; works over any `TextMapCarrier` (HTTP/MQ). Full
  trace-context (traceparent) integration lands with the OTel SDK in the telemetry package.
- `resolver/`: added `SubdomainResolver`. **Deferred:** JWT-claim resolver — extracting a
  claim safely requires signature verification / key management; shipping an unverified
  variant in a regulated framework is a foot-gun, so it waits for that infra.
- `isolation/`: added `RowLevelStrategy` (Postgres RLS via a `set_config` session
  parameter, proven with a real RLS integration test). **Deferred:** Database-per-Tenant —
  needs a connection-routing/pooling design, not just a query hook.
- `policy/`: minimal `PolicyHook` (allow/deny `Decision`), wired into the middleware as an
  optional 403 gate. Full engine is post-v1.0.
- **Gate:** `go test ./...` green; both isolation strategies verified against real Postgres.

### Phase 3 — Lock the contract — DONE
- Language-neutral fixtures live in `testkit/compliance-tests/fixtures/` (see the README
  there): tenant-context validation, header + subdomain resolution, schema-isolation naming
  and injection rejection, `TenantEvent` JSON shape, and W3C Baggage propagation encoding.
  Errors are symbolic codes mapped per-language.
- The Go reference harness at `go/tenantguard/conformance/` loads every fixture and asserts
  the real packages against it — the template each other language mirrors.
- **Gate:** `go test ./conformance/` green; these fixtures now *define* conformance.

### Phase 4 — Port Java, then Python, then TypeScript — IN PROGRESS (Java)
For each: populate the build file, implement Ring 0→4 against the **same fixtures**, then add
that ecosystem's idiomatic adapters. Wire the already-present per-language CI workflow.
- Java: **Rings 0–1 done** — `pom.xml` (single-module, JDK 17); immutable `TenantContext`
  record (Ring 0); `HeaderResolver` + `SubdomainResolver` over a dependency-free
  `HttpRequestView` port (Ring 1). A JUnit 5 conformance harness reads the shared fixtures
  (`tenant-context`, `resolver-header`, `resolver-subdomain`) and passes. CI wired
  (`ci-java.yml`). Next: Ring 2 (schema isolation vs `isolation-schema.json`, `TenantEvent`
  vs `tenant-event.json`), then propagation (`opentelemetry-java` vs `propagation-baggage.json`),
  then the servlet/Spring middleware + JPA/JDBC adapters. The multi-module split
  (`tenantguard-core`, `tenantguard-jpa`, ...) is a later refactor of the flat project.
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
