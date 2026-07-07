"""TenantGuard: platform-agnostic multitenancy for regulated environments.

Python reference implementation. See core/spec.md for the language-neutral contract.
"""

from tenantguard.context import (
    EMPTY_TENANT_ID,
    INVALID_TRACE_ID,
    TenantContext,
    TenantContextError,
)

__all__ = [
    "TenantContext",
    "TenantContextError",
    "EMPTY_TENANT_ID",
    "INVALID_TRACE_ID",
]
