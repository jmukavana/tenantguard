/**
 * TenantGuard: platform-agnostic multitenancy for regulated environments.
 *
 * TypeScript reference implementation. See core/spec.md for the language-neutral contract.
 */
export {
  TenantContext,
  TenantContextError,
  EMPTY_TENANT_ID,
  INVALID_TRACE_ID,
} from "./context/index.js";

export {
  DEFAULT_HEADER,
  HeaderResolver,
  SubdomainResolver,
} from "./resolver/index.js";
export type { HttpRequestView, TenantResolver } from "./resolver/index.js";

export { IsolationError, SchemaStrategy, UNSAFE_TENANT_ID } from "./isolation/index.js";
export type { QueryRunner } from "./isolation/index.js";

export { LoggingAuditHook, TenantEvent } from "./audit/index.js";
export type { AuditHook, Outcome, TenantEventInit } from "./audit/index.js";

export { Propagator } from "./propagation/index.js";
export type { Carrier } from "./propagation/index.js";
