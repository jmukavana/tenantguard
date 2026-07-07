"""Unit tests for the TenantEvent audit record."""

from __future__ import annotations

import json
from datetime import datetime, timezone

from tenantguard.audit import Outcome, TenantEvent


def test_minimal_event_omits_empty_optionals() -> None:
    event = TenantEvent("e1", "tenant.resolved", "acme",
                        datetime(2026, 1, 1, tzinfo=timezone.utc), Outcome.ALLOWED)
    out = json.loads(event.to_json())
    assert set(out) == {"eventId", "eventType", "tenantId", "timestamp", "outcome"}
    assert out["outcome"] == "allowed"
    assert out["timestamp"] == "2026-01-01T00:00:00Z"


def test_metadata_keys_are_sorted() -> None:
    event = TenantEvent("e2", "x", "acme", datetime(2026, 1, 1, tzinfo=timezone.utc),
                        Outcome.DENIED, metadata={"z": "1", "a": "2"})
    assert '"metadata":{"a":"2","z":"1"}' in event.to_json()


def test_create_generates_fresh_id() -> None:
    a = TenantEvent.create("x", "acme", "", Outcome.ALLOWED)
    b = TenantEvent.create("x", "acme", "", Outcome.ALLOWED)
    assert len(a.event_id) == 32
    assert a.event_id != b.event_id
