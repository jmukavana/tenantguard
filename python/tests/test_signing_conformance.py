"""Runs the shared auditforge-signing.json golden vectors against the signing helpers."""

from __future__ import annotations

from datetime import datetime

import pytest

from _fixtures import load
from tenantguard.audit import Outcome, TenantEvent, canonical_json, sign, verify

_FIXTURE = load("auditforge-signing.json")
_SECRET = _FIXTURE["secret"].encode("utf-8")
_CASES = _FIXTURE["cases"]


def _event(ev: dict) -> TenantEvent:
    return TenantEvent(
        event_id=ev["eventId"],
        event_type=ev["eventType"],
        tenant_id=ev["tenantId"],
        timestamp=datetime.fromisoformat(ev["timestamp"]),
        outcome=Outcome(ev["outcome"]),
        trace_id=ev.get("traceId", ""),
        actor=ev.get("actor", ""),
        resource=ev.get("resource", ""),
        metadata=ev.get("metadata") or {},
    )


@pytest.mark.parametrize("case", _CASES, ids=[c["name"] for c in _CASES])
def test_signing(case: dict) -> None:
    event = _event(case["event"])

    # Canonical bytes and the HMAC are checked separately so a mismatch localizes to
    # serialization vs. the signing primitive.
    assert canonical_json(event) == case["canonical"]
    assert sign(event, _SECRET) == case["signature"]

    assert verify(event, _SECRET, case["signature"])
    assert not verify(event, b"wrong-secret", case["signature"])
