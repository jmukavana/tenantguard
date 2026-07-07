"""Tenant audit events and hooks (core/spec.md, Observability & Audit)."""

from __future__ import annotations

import hashlib
import hmac
import json
import logging
import secrets
from dataclasses import dataclass, field, replace
from datetime import datetime, timezone
from enum import Enum
from types import MappingProxyType
from typing import Mapping, Protocol, runtime_checkable

_LOG = logging.getLogger("tenantguard.audit")


class Outcome(str, Enum):
    """The result of an audited operation, as it appears on the wire."""

    ALLOWED = "allowed"
    DENIED = "denied"
    ERROR = "error"


@dataclass(frozen=True)
class TenantEvent:
    """Structured audit record. :meth:`to_json` is the cross-language wire contract and
    mirrors core/schemas/TenantEvent.schema.json: required keys are always present, empty
    optional keys are omitted, and metadata keys are sorted for determinism."""

    event_id: str
    event_type: str
    tenant_id: str
    timestamp: datetime
    outcome: Outcome
    trace_id: str = ""
    actor: str = ""
    resource: str = ""
    metadata: Mapping[str, str] = field(default_factory=dict)
    signature: str = ""

    def __post_init__(self) -> None:
        for name in ("trace_id", "actor", "resource", "signature"):
            if getattr(self, name) is None:
                object.__setattr__(self, name, "")
        object.__setattr__(self, "metadata", MappingProxyType(dict(self.metadata or {})))

    @staticmethod
    def create(event_type: str, tenant_id: str, trace_id: str, outcome: Outcome) -> "TenantEvent":
        """Build an event stamped with a fresh id and the current UTC time."""
        return TenantEvent(
            secrets.token_hex(16), event_type, tenant_id, datetime.now(timezone.utc),
            outcome, trace_id,
        )

    def to_json(self) -> str:
        """Serialize to canonical JSON (the AuditForge wire form)."""
        obj: dict = {
            "eventId": self.event_id,
            "eventType": self.event_type,
            "tenantId": self.tenant_id,
            "timestamp": _iso(self.timestamp),
            "outcome": self.outcome.value,
        }
        if self.trace_id:
            obj["traceId"] = self.trace_id
        if self.actor:
            obj["actor"] = self.actor
        if self.resource:
            obj["resource"] = self.resource
        if self.metadata:
            obj["metadata"] = dict(sorted(self.metadata.items()))
        if self.signature:
            obj["signature"] = self.signature
        return json.dumps(obj, separators=(",", ":"), ensure_ascii=False)


def canonical_json(event: TenantEvent) -> str:
    """Return the exact bytes that are signed for ``event``: its canonical JSON with any
    signature cleared, so a stale signature never feeds back into the signed form
    (ecosystem/auditforge/spec.md, "The signing contract")."""
    if event.signature:
        event = replace(event, signature="")
    return event.to_json()


def sign(event: TenantEvent, secret: bytes) -> str:
    """Return the lowercase-hex HMAC-SHA256 of ``event``'s canonical form under ``secret``."""
    return hmac.new(secret, canonical_json(event).encode("utf-8"), hashlib.sha256).hexdigest()


def verify(event: TenantEvent, secret: bytes, signature: str) -> bool:
    """Report whether ``signature`` is a valid HMAC-SHA256 over ``event``'s canonical form.

    The comparison is constant-time."""
    return hmac.compare_digest(sign(event, secret), signature)


def _iso(ts: datetime) -> str:
    """RFC 3339 / ISO-8601, using 'Z' for a UTC offset (matches Go/Java output)."""
    s = ts.isoformat()
    return s[:-6] + "Z" if s.endswith("+00:00") else s


@runtime_checkable
class AuditHook(Protocol):
    """Publishes tenant audit events to a sink (AuditForge in production)."""

    def publish(self, event: TenantEvent) -> None:
        ...


class LoggingAuditHook:
    """Development :class:`AuditHook` that logs events. Replace with the AuditForge client."""

    def publish(self, event: TenantEvent) -> None:
        _LOG.info("tenant.audit %s", event.to_json())
