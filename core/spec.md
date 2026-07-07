# TenantGuard Core Specification (v1.0)

Language-neutral specification for the multitenancy framework.

## Core Principles
- **Immutable Tenant Context**: Once created, the context cannot be changed (prevents tenant hopping).
- **Secure by Default**: All resolvers and isolation strategies must enforce tenant boundaries.
- **Regulatory First**: Designed for CBK, IRA, Data Protection Act compliance.
- **Observability & Audit**: Every tenant-aware operation must publish events to AuditForge.
- **Platform Agnostic**: Same concepts apply across Java, Go, Python, TypeScript.

## Core Interfaces

### TenantContext
- `TenantID()` string
- `TenantName()` string
- `Metadata()` map[string]string
- `TraceID()` string (W3C Trace Context)
- `IsValid()` bool
- Must be immutable after creation.

### TenantResolver
- `Resolve(req Request) (TenantContext, error)`
- Supported strategies: HTTP Header (`X-Tenant-ID`), JWT claim, Subdomain, Client Certificate, Message Queue Header, Custom.

### IsolationStrategy
- `Apply(ctx TenantContext, query Query) (Query, error)`
- Implementations: Schema-based (recommended for regulated systems), Row-Level Security, Database-per-Tenant.
- Must enforce strict data isolation.

### ContextPropagator
- Propagates tenant context across HTTP calls, async jobs, message queues, and threads.
- Uses W3C Baggage for tenant data + OpenTelemetry Trace Context.

### AuditHook
- `Publish(event TenantEvent)`
- Automatically sends structured events to AuditForge.

## TenantEvent Schema
See `TenantEvent.schema.json`

## Security Requirements
- Tenant context must be validated on every request.
- Reject any attempt to switch tenants mid-request.
- All events must be signed or sent over authenticated channel to AuditForge.

## ADR References
- ADR-001: Why Schema-Based Isolation is Default for Regulated Systems
- ADR-002: Choice of W3C Trace Context + Baggage for Propagation
- ADR-004: Enforcing Immutable Tenant Context

This specification is the single source of truth. All language implementations must follow it.