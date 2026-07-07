"""Immutable tenant identity (core/spec.md, ADR-004)."""

from __future__ import annotations

import re
from dataclasses import dataclass, field
from types import MappingProxyType
from typing import Mapping, Optional

#: Tenant id was empty.
EMPTY_TENANT_ID = "empty_tenant_id"
#: Trace id was present but not a valid W3C trace-id.
INVALID_TRACE_ID = "invalid_trace_id"

_TRACE_ID = re.compile(r"^[0-9a-f]{32}$")


class TenantContextError(ValueError):
    """Raised when a :class:`TenantContext` cannot be constructed.

    ``code`` is a stable, language-neutral symbol matching the compliance fixtures.
    """

    def __init__(self, code: str, message: str) -> None:
        super().__init__(message)
        self.code = code


@dataclass(frozen=True)
class TenantContext:
    """The tenant an operation runs for.

    Frozen (so attributes cannot be reassigned) with metadata exposed as a read-only
    mapping over a defensive copy — neither the caller's later mutation of the source dict
    nor an attempt to mutate ``metadata`` can alter the context. This immutability is what
    prevents tenant hopping.
    """

    tenant_id: str
    tenant_name: str = ""
    trace_id: str = ""
    metadata: Mapping[str, str] = field(default_factory=dict)

    def __post_init__(self) -> None:
        if not self.tenant_id:
            raise TenantContextError(EMPTY_TENANT_ID, "tenant id must not be empty")

        object.__setattr__(self, "tenant_name", self.tenant_name or "")
        object.__setattr__(self, "trace_id", self.trace_id or "")

        if self.trace_id and not _TRACE_ID.match(self.trace_id):
            raise TenantContextError(
                INVALID_TRACE_ID, "trace id must be 32 lowercase hex characters"
            )

        source: Optional[Mapping[str, str]] = self.metadata
        object.__setattr__(self, "metadata", MappingProxyType(dict(source or {})))

    def is_valid(self) -> bool:
        """Whether this context carries a tenant id (always true once constructed)."""
        return bool(self.tenant_id)
