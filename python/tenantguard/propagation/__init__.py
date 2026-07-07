"""Cross-boundary tenant context propagation (core/spec.md, ContextPropagator).

Tenant identity travels as W3C Baggage members under the ``tg.*`` namespace via
OpenTelemetry, so any instrumented hop preserves it. This is the Python counterpart of the
Go and Java propagation modules and shares their wire encoding. Within a process, tenant
context needs no serialization; ``inject``/``extract`` are for crossing a wire.
"""

from __future__ import annotations

import json
from typing import Mapping, Optional

from opentelemetry import baggage
from opentelemetry.baggage.propagation import W3CBaggagePropagator
from opentelemetry.context import Context

from tenantguard.context import TenantContext

MEMBER_TENANT_ID = "tg.tenant.id"
MEMBER_TENANT_NAME = "tg.tenant.name"
MEMBER_TRACE_ID = "tg.trace.id"
MEMBER_METADATA = "tg.tenant.meta"

_W3C = W3CBaggagePropagator()


class Propagator:
    """Serializes a :class:`TenantContext` into and out of a carrier as W3C Baggage."""

    def inject(self, tc: TenantContext, carrier: dict, context: Optional[Context] = None) -> None:
        """Write ``tc`` into ``carrier``, preserving any baggage already in ``context``."""
        if tc is None or not tc.is_valid():
            raise ValueError("cannot propagate an invalid tenant context")
        ctx = context if context is not None else Context()
        ctx = baggage.set_baggage(MEMBER_TENANT_ID, tc.tenant_id, ctx)
        if tc.tenant_name:
            ctx = baggage.set_baggage(MEMBER_TENANT_NAME, tc.tenant_name, ctx)
        if tc.trace_id:
            ctx = baggage.set_baggage(MEMBER_TRACE_ID, tc.trace_id, ctx)
        if tc.metadata:
            ctx = baggage.set_baggage(MEMBER_METADATA, _encode(tc.metadata), ctx)
        _W3C.inject(carrier, context=ctx)

    def extract(self, carrier: Mapping[str, str], context: Optional[Context] = None) -> Optional[TenantContext]:
        """Reconstruct a TenantContext from ``carrier``, or ``None`` when it carries no tenant."""
        ctx = _W3C.extract(carrier, context=context if context is not None else Context())
        tenant_id = baggage.get_baggage(MEMBER_TENANT_ID, ctx)
        if not tenant_id:
            return None
        raw_meta = baggage.get_baggage(MEMBER_METADATA, ctx)
        metadata = json.loads(raw_meta) if raw_meta else None
        return TenantContext(
            str(tenant_id),
            _text(baggage.get_baggage(MEMBER_TENANT_NAME, ctx)),
            _text(baggage.get_baggage(MEMBER_TRACE_ID, ctx)),
            metadata,
        )


def _encode(metadata: Mapping[str, str]) -> str:
    """Canonical, sorted, compact JSON — byte-compatible with the Go/Java encoders."""
    return json.dumps(dict(sorted(metadata.items())), separators=(",", ":"), ensure_ascii=False)


def _text(value: object) -> str:
    return "" if value is None else str(value)
