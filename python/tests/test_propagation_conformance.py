"""Runs the shared propagation-baggage.json fixture against the Python Propagator."""

from __future__ import annotations

import pytest
from opentelemetry import baggage
from opentelemetry.baggage.propagation import W3CBaggagePropagator
from opentelemetry.context import Context

from _fixtures import cases
from tenantguard.context import TenantContext
from tenantguard.propagation import Propagator

_CASES = cases("propagation-baggage.json")
_PROPAGATOR = Propagator()
_W3C = W3CBaggagePropagator()


@pytest.mark.parametrize("case", _CASES, ids=[c["name"] for c in _CASES])
def test_propagation(case: dict) -> None:
    ctx_node = case["context"]
    tc = TenantContext(
        ctx_node["tenantId"],
        ctx_node.get("tenantName", ""),
        ctx_node.get("traceId", ""),
        ctx_node.get("metadata"),
    )

    carrier: dict = {}
    _PROPAGATOR.inject(tc, carrier)

    members = baggage.get_all(_W3C.extract(carrier, context=Context()))
    expect = case["expectMembers"]

    # Every expected member present with the exact value...
    for key, value in expect.items():
        assert members.get(key) == value, f"member {key}"
    # ...and no unexpected tg.* member leaks.
    for key in members:
        if key.startswith("tg."):
            assert key in expect, f"unexpected tenant member {key}"

    # Round-trip reconstructs an equal context.
    got = _PROPAGATOR.extract(carrier)
    assert got is not None
    assert got.tenant_id == tc.tenant_id
    assert got.tenant_name == tc.tenant_name
    assert got.trace_id == tc.trace_id
    assert dict(got.metadata) == dict(tc.metadata)
