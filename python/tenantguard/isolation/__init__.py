"""Tenant data-isolation strategies (core/spec.md, IsolationStrategy)."""

from __future__ import annotations

import re

from tenantguard.context import TenantContext

#: Tenant id cannot be used as a safe schema identifier.
UNSAFE_TENANT_ID = "unsafe_tenant_id"

_SAFE_IDENT = re.compile(r"^[A-Za-z0-9_]+$")


class IsolationError(ValueError):
    """Raised when an isolation strategy refuses to act.

    ``code`` is a stable, language-neutral symbol matching the compliance fixtures.
    """

    def __init__(self, code: str, message: str) -> None:
        super().__init__(message)
        self.code = code


class SchemaStrategy:
    """Schema-based isolation — the recommended default for regulated systems.

    Each tenant's data lives in its own Postgres schema; queries run with ``search_path``
    scoped to that schema. Because a schema name is interpolated into a ``SET`` statement
    (which takes no bind parameters), tenant ids are restricted to a strict allowlist so
    SQL injection is impossible.
    """

    def __init__(self, prefix: str = "") -> None:
        self.prefix = prefix or ""

    def schema_name(self, tc: TenantContext) -> str:
        """Return the validated, injection-safe schema name for ``tc``."""
        tenant_id = tc.tenant_id
        if not tenant_id or not _SAFE_IDENT.match(tenant_id):
            raise IsolationError(
                UNSAFE_TENANT_ID, f"tenant id is not a safe schema identifier: {tenant_id!r}"
            )
        return self.prefix + tenant_id

    def search_path_sql(self, tc: TenantContext) -> str:
        """The SET statement that scopes ``search_path`` to ``tc``'s schema."""
        # schema_name matched the allowlist, so double-quoting it can carry no injection.
        return f'SET search_path TO "{self.schema_name(tc)}"'

    def bind(self, cursor, tc: TenantContext) -> None:
        """Scope subsequent statements on a DB-API ``cursor`` to the tenant's schema.

        Run it at the start of the tenant-scoped transaction; afterwards an unqualified
        ``SELECT ... FROM widgets`` resolves only within the tenant's schema.
        """
        cursor.execute(self.search_path_sql(tc))
