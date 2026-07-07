"""Idiomatic-usage tests for the resolvers. Behavioral coverage lives in the fixture
conformance tests; these document the API."""

from __future__ import annotations

from _helpers import FakeRequest
from tenantguard.resolver import HeaderResolver, SubdomainResolver


def test_header_resolver_custom_header_and_traceparent() -> None:
    request = FakeRequest(headers={
        "X-Org": "beta",
        "traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
    })
    tc = HeaderResolver("X-Org").resolve(request)
    assert tc is not None
    assert tc.tenant_id == "beta"
    assert tc.trace_id == "4bf92f3577b34da6a3ce929d0e0e4736"


def test_header_resolver_missing_returns_none() -> None:
    assert HeaderResolver().resolve(FakeRequest()) is None


def test_subdomain_resolver_custom_ignore() -> None:
    resolver = SubdomainResolver("example.com", ["admin", "www"])
    assert resolver.resolve(FakeRequest(host="admin.example.com")) is None
    resolved = resolver.resolve(FakeRequest(host="acme.example.com"))
    assert resolved is not None and resolved.tenant_id == "acme"
