"""Tenant resolvers (core/spec.md, TenantResolver).

A resolver turns an inbound request into a :class:`~tenantguard.context.TenantContext`,
returning ``None`` when the request carries no tenant identifier (the Python form of the
fixtures' ``no_tenant`` outcome). Resolvers work against the small :class:`HttpRequestView`
protocol so the core needn't depend on any web framework; adapters wrap their framework's
request into it.
"""

from __future__ import annotations

import re
from typing import Iterable, Optional, Protocol, runtime_checkable

from tenantguard.context import TenantContext

#: The conventional tenant header (core/spec.md, TenantResolver).
DEFAULT_HEADER = "X-Tenant-ID"

_TRACE_ID = re.compile(r"^[0-9a-f]{32}$")


@runtime_checkable
class HttpRequestView(Protocol):
    """Minimal read-only view of an inbound request that resolvers need."""

    def header(self, name: str) -> Optional[str]:
        """Return the named header's value, or ``None`` if absent."""
        ...

    def host(self) -> Optional[str]:
        """Return the request host, possibly including a port."""
        ...


class HeaderResolver:
    """Resolves the tenant from an HTTP header (default ``X-Tenant-ID``).

    When present, the W3C ``traceparent`` header seeds the context's trace id.
    """

    def __init__(self, header: str = DEFAULT_HEADER) -> None:
        self.header = header or DEFAULT_HEADER

    def resolve(self, request: HttpRequestView) -> Optional[TenantContext]:
        raw = request.header(self.header)
        tenant_id = (raw or "").strip()
        if not tenant_id:
            return None
        return TenantContext(tenant_id, "", _trace_id(request.header("traceparent")), None)


class SubdomainResolver:
    """Resolves the tenant from the left-most label of the request host.

    ``acme.example.com`` -> ``acme`` when ``base_domain`` is ``example.com``.
    """

    def __init__(self, base_domain: str, ignore: Optional[Iterable[str]] = None) -> None:
        self.base_domain = base_domain or ""
        self.ignore = [label.lower() for label in ignore] if ignore else ["www"]

    def resolve(self, request: HttpRequestView) -> Optional[TenantContext]:
        host = request.host()
        if not host or not self.base_domain:
            return None
        host = _strip_port(host).lower().rstrip(".")

        suffix = "." + self.base_domain.lower()
        if not host.endswith(suffix):
            return None

        label = host[: -len(suffix)].split(".", 1)[0]
        if not label or label in self.ignore:
            return None
        return TenantContext(label, "", _trace_id(request.header("traceparent")), None)


def _trace_id(traceparent: Optional[str]) -> str:
    """Extract the W3C trace-id from a traceparent, or "" when missing/malformed."""
    if not traceparent:
        return ""
    parts = traceparent.split("-")
    if len(parts) < 4:
        return ""
    trace_id = parts[1].lower()
    return trace_id if _TRACE_ID.match(trace_id) else ""


def _strip_port(host: str) -> str:
    """Remove a trailing ``:port`` for a plain host:port (leaves bare hosts untouched)."""
    first = host.find(":")
    if first >= 0 and first == host.rfind(":"):
        return host[:first]
    return host
