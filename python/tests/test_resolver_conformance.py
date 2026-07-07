"""Runs the shared resolver fixtures against the Python resolvers."""

from __future__ import annotations

import pytest

from _fixtures import cases
from _helpers import FakeRequest
from tenantguard.resolver import HeaderResolver, SubdomainResolver

_HEADER = cases("resolver-header.json")
_SUBDOMAIN = cases("resolver-subdomain.json")


@pytest.mark.parametrize("case", _HEADER, ids=[c["name"] for c in _HEADER])
def test_header_resolver(case: dict) -> None:
    request = FakeRequest(headers=case.get("headers", {}))
    result = HeaderResolver(case.get("header", "")).resolve(request)
    expect = case["expect"]

    if "error" in expect:
        assert expect["error"] == "no_tenant"
        assert result is None
        return
    assert result is not None
    assert result.tenant_id == expect.get("tenantId", "")
    assert result.trace_id == expect.get("traceId", "")


@pytest.mark.parametrize("case", _SUBDOMAIN, ids=[c["name"] for c in _SUBDOMAIN])
def test_subdomain_resolver(case: dict) -> None:
    request = FakeRequest(host=case["host"])
    result = SubdomainResolver(case["baseDomain"], case.get("ignore")).resolve(request)
    expect = case["expect"]

    if "error" in expect:
        assert expect["error"] == "no_tenant"
        assert result is None
        return
    assert result is not None
    assert result.tenant_id == expect.get("tenantId", "")
