/**
 * Immutable tenant identity (core/spec.md, ADR-004).
 */

/** Tenant id was empty. */
export const EMPTY_TENANT_ID = "empty_tenant_id";
/** Trace id was present but not a valid W3C trace-id. */
export const INVALID_TRACE_ID = "invalid_trace_id";

const TRACE_ID = /^[0-9a-f]{32}$/;

/**
 * Raised when a {@link TenantContext} cannot be constructed. `code` is a stable,
 * language-neutral symbol matching the compliance fixtures.
 */
export class TenantContextError extends Error {
  readonly code: string;

  constructor(code: string, message: string) {
    super(message);
    this.name = "TenantContextError";
    this.code = code;
  }
}

/**
 * The tenant an operation runs for. Instances are frozen and their metadata is a frozen
 * copy, so neither reassigning a field nor mutating the source/returned metadata can alter
 * the context — the property that prevents tenant hopping. Build one with {@link create}.
 */
export class TenantContext {
  readonly tenantId: string;
  readonly tenantName: string;
  readonly traceId: string;
  readonly metadata: Readonly<Record<string, string>>;

  private constructor(
    tenantId: string,
    tenantName: string,
    traceId: string,
    metadata: Record<string, string>,
  ) {
    this.tenantId = tenantId;
    this.tenantName = tenantName;
    this.traceId = traceId;
    this.metadata = Object.freeze({ ...metadata });
    Object.freeze(this);
  }

  /**
   * Constructs an immutable TenantContext.
   *
   * @throws {TenantContextError} if `tenantId` is empty or `traceId` is malformed
   */
  static create(
    tenantId: string,
    tenantName = "",
    traceId = "",
    metadata: Record<string, string> = {},
  ): TenantContext {
    if (!tenantId) {
      throw new TenantContextError(EMPTY_TENANT_ID, "tenant id must not be empty");
    }
    if (traceId && !TRACE_ID.test(traceId)) {
      throw new TenantContextError(
        INVALID_TRACE_ID,
        "trace id must be 32 lowercase hex characters",
      );
    }
    return new TenantContext(tenantId, tenantName, traceId, metadata);
  }

  /** Whether this context carries a tenant id (always true once constructed). */
  isValid(): boolean {
    return this.tenantId.length > 0;
  }
}
