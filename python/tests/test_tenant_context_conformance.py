"""Runs the shared tenant-context.json fixture against the Python implementation.

Python mirror of go/tenantguard/conformance and the Java conformance harness; all three
must pass the same vectors.
"""

from __future__ import annotations

import pytest

from _fixtures import cases
from tenantguard.context import TenantContext, TenantContextError

_CASES = cases("tenant-context.json")


@pytest.mark.parametrize("case", _CASES, ids=[c["name"] for c in _CASES])
def test_tenant_context(case: dict) -> None:
    inp = case["input"]
    expect = case["expect"]

    def build() -> TenantContext:
        return TenantContext(
            tenant_id=inp.get("tenantId", ""),
            tenant_name=inp.get("tenantName", ""),
            trace_id=inp.get("traceId", ""),
            metadata=inp.get("metadata"),
        )

    if "error" in expect:
        with pytest.raises(TenantContextError) as excinfo:
            build()
        assert excinfo.value.code == expect["error"]
        return

    tc = build()
    assert tc.is_valid() is expect["valid"]
    assert tc.tenant_id == expect.get("tenantId", "")
    assert tc.tenant_name == expect.get("tenantName", "")
    assert tc.trace_id == expect.get("traceId", "")
    assert dict(tc.metadata) == expect.get("metadata", {})
