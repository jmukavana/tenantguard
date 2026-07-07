"""Runs the shared isolation-schema.json fixture against the Python SchemaStrategy."""

from __future__ import annotations

import pytest

from _fixtures import cases
from tenantguard.context import TenantContext
from tenantguard.isolation import IsolationError, SchemaStrategy

_CASES = cases("isolation-schema.json")


@pytest.mark.parametrize("case", _CASES, ids=[c["name"] for c in _CASES])
def test_schema_isolation(case: dict) -> None:
    tc = TenantContext(case["tenantId"])
    strategy = SchemaStrategy(case.get("prefix", ""))
    expect = case["expect"]

    if "error" in expect:
        with pytest.raises(IsolationError) as excinfo:
            strategy.schema_name(tc)
        assert excinfo.value.code == expect["error"]
        return
    assert strategy.schema_name(tc) == expect["schema"]
