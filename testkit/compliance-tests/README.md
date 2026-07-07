# TenantGuard Compliance Fixtures

Language-neutral, data-only test vectors that define what "conformant" means for **every**
TenantGuard implementation. They are the executable form of `core/spec.md`: if two
languages both pass these fixtures, they agree on tenant behavior.

## How it works

Each file in `fixtures/` is a JSON document:

```json
{ "description": "...", "cases": [ { "name": "...", <inputs>, "expect": { ... } } ] }
```

Every implementation ships a thin *harness* that, for each fixture, feeds `<inputs>` to
its real code and asserts the result matches `expect`. The Go reference harness lives at
`go/tenantguard/conformance/` and doubles as the template for other languages.

`expect.error` is a **symbolic error code** (e.g. `empty_tenant_id`), not a message — each
language maps it to its own error type. See the mapping table below.

| code                | raised by                              |
|---------------------|----------------------------------------|
| `empty_tenant_id`   | TenantContext construction with no id  |
| `invalid_trace_id`  | TenantContext with a non-W3C trace id  |
| `no_tenant`         | a resolver finding no tenant           |
| `unsafe_tenant_id`  | schema isolation rejecting an id       |

## Fixtures

| file                       | contract                                             |
|----------------------------|------------------------------------------------------|
| `tenant-context.json`      | construction, validation, immutable accessors        |
| `resolver-header.json`     | `X-Tenant-ID` header + traceparent extraction        |
| `resolver-subdomain.json`  | subdomain-to-tenant mapping                          |
| `isolation-schema.json`    | tenant-id → schema name, injection rejection         |
| `tenant-event.json`        | `TenantEvent` JSON shape (required/omitted keys)     |
| `propagation-baggage.json` | W3C Baggage encoding of a tenant context (interop)   |

Changing a fixture changes the contract for all languages; do it deliberately and in step
with `core/spec.md`.
