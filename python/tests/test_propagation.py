"""Unit tests for the W3C Baggage propagator."""

from __future__ import annotations

import pytest
from opentelemetry import baggage
from opentelemetry.baggage.propagation import W3CBaggagePropagator
from opentelemetry.context import Context

from tenantguard.context import TenantContext
from tenantguard.propagation import Propagator


def test_round_trip_with_metadata() -> None:
    tc = TenantContext("acme", "Acme Ltd", "4bf92f3577b34da6a3ce929d0e0e4736",
                       {"region": "ke", "tier": "gold"})
    carrier: dict = {}
    Propagator().inject(tc, carrier)
    assert "baggage" in carrier

    got = Propagator().extract(carrier)
    assert got is not None
    assert got.tenant_id == "acme"
    assert got.tenant_name == "Acme Ltd"
    assert got.trace_id == "4bf92f3577b34da6a3ce929d0e0e4736"
    assert dict(got.metadata) == {"region": "ke", "tier": "gold"}


def test_extract_empty_carrier_returns_none() -> None:
    assert Propagator().extract({}) is None


def test_inject_invalid_context_raises() -> None:
    with pytest.raises(ValueError):
        Propagator().inject(None, {})  # type: ignore[arg-type]


def test_inject_preserves_existing_baggage() -> None:
    context = baggage.set_baggage("app.feature", "on", Context())
    carrier: dict = {}
    Propagator().inject(TenantContext("acme"), carrier, context=context)

    members = baggage.get_all(W3CBaggagePropagator().extract(carrier, context=Context()))
    assert members.get("app.feature") == "on"
    assert members.get("tg.tenant.id") == "acme"
