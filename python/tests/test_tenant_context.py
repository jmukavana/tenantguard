"""Security-critical guard: a TenantContext must not change after construction.

Mirrors the Go TestImmutable and the Java TenantContextTest.
"""

from __future__ import annotations

import dataclasses

import pytest

from tenantguard.context import (
    EMPTY_TENANT_ID,
    INVALID_TRACE_ID,
    TenantContext,
    TenantContextError,
)


def test_mutating_source_map_does_not_leak() -> None:
    src = {"region": "ke", "tier": "gold"}
    tc = TenantContext("acme", metadata=src)

    src["region"] = "tz"
    src["injected"] = "evil"

    assert tc.metadata["region"] == "ke"
    assert "injected" not in tc.metadata


def test_metadata_is_read_only() -> None:
    tc = TenantContext("acme", metadata={"a": "b"})
    with pytest.raises(TypeError):
        tc.metadata["x"] = "y"  # type: ignore[index]


def test_attributes_are_frozen() -> None:
    tc = TenantContext("acme")
    with pytest.raises(dataclasses.FrozenInstanceError):
        tc.tenant_id = "beta"  # type: ignore[misc]


def test_empty_tenant_id_raises() -> None:
    with pytest.raises(TenantContextError) as excinfo:
        TenantContext("")
    assert excinfo.value.code == EMPTY_TENANT_ID


def test_malformed_trace_id_raises() -> None:
    with pytest.raises(TenantContextError) as excinfo:
        TenantContext("acme", trace_id="not-a-trace")
    assert excinfo.value.code == INVALID_TRACE_ID


def test_valid_context_exposes_fields() -> None:
    tc = TenantContext("acme", "Acme", "4bf92f3577b34da6a3ce929d0e0e4736")
    assert tc.is_valid()
    assert tc.tenant_id == "acme"
    assert tc.trace_id == "4bf92f3577b34da6a3ce929d0e0e4736"
    assert dict(tc.metadata) == {}
