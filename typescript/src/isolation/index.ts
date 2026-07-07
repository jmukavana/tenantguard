/**
 * Tenant data-isolation strategies (core/spec.md, IsolationStrategy).
 */
import { TenantContext } from "../context/index.js";

/** Tenant id cannot be used as a safe schema identifier. */
export const UNSAFE_TENANT_ID = "unsafe_tenant_id";

const SAFE_IDENT = /^[A-Za-z0-9_]+$/;

/** Raised when an isolation strategy refuses to act. `code` matches the compliance fixtures. */
export class IsolationError extends Error {
  readonly code: string;

  constructor(code: string, message: string) {
    super(message);
    this.name = "IsolationError";
    this.code = code;
  }
}

/** Minimal SQL executor (a `pg`/`mysql2`-style client) that {@link SchemaStrategy.bind} uses. */
export interface QueryRunner {
  query(sql: string): Promise<unknown> | unknown;
}

/**
 * Schema-based isolation — the recommended default for regulated systems. Each tenant's
 * data lives in its own Postgres schema; queries run with `search_path` scoped to it.
 *
 * Because a schema name is interpolated into a `SET` statement (which takes no bind
 * parameters), tenant ids are restricted to a strict allowlist so SQL injection is
 * impossible.
 */
export class SchemaStrategy {
  constructor(private readonly prefix: string = "") {}

  /** Return the validated, injection-safe schema name for `tc`. */
  schemaName(tc: TenantContext): string {
    const id = tc.tenantId;
    if (!id || !SAFE_IDENT.test(id)) {
      throw new IsolationError(
        UNSAFE_TENANT_ID,
        `tenant id is not a safe schema identifier: ${id}`,
      );
    }
    return this.prefix + id;
  }

  /** The SET statement that scopes `search_path` to `tc`'s schema. */
  searchPathSql(tc: TenantContext): string {
    // schemaName matched the allowlist, so double-quoting it can carry no injection.
    return `SET search_path TO "${this.schemaName(tc)}"`;
  }

  /** Scope subsequent statements on `runner` to the tenant's schema. */
  async bind(runner: QueryRunner, tc: TenantContext): Promise<void> {
    await runner.query(this.searchPathSql(tc));
  }
}
