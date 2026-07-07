"""Runs the shared tenant-event.json fixture against TenantEvent.to_json()."""

from __future__ import annotations

import json
from datetime import datetime

import pytest

from _fixtures import cases
from tenantguard.audit import Outcome, TenantEvent

_CASES = cases("tenant-event.json")


@pytest.mark.parametrize("case", _CASES, ids=[c["name"] for c in _CASES])
def test_tenant_event(case: dict) -> None:
    ev = case["event"]
    event = TenantEvent(
        event_id=ev["eventId"],
        event_type=ev["eventType"],
        tenant_id=ev["tenantId"],
        timestamp=datetime.fromisoformat(ev["timestamp"]),
        outcome=Outcome(ev["outcome"]),
        trace_id=ev.get("traceId", ""),
        actor=ev.get("actor", ""),
        resource=ev.get("resource", ""),
        metadata=ev.get("metadata") or {},
        signature=ev.get("signature", ""),
    )

    out = json.loads(event.to_json())
    expect = case["expect"]

    for key in expect["requiredKeys"]:
        assert key in out, f"required key missing: {key}"
    for key in expect["absentKeys"]:
        assert key not in out, f"key should be omitted: {key}"
    for key, value in expect["values"].items():
        assert out[key] == value
