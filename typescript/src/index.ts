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
