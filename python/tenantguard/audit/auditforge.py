"""AuditForge delivery client (ecosystem/auditforge/spec.md).

:class:`AuditForgeHook` signs each event and POSTs it to AuditForge's ``POST /v1/events``.
It is the production replacement for
:class:`~tenantguard.audit.LoggingAuditHook`: every event is HMAC-signed with a shared key
and appended to the tamper-evident audit log. Uses only the standard library (``urllib``).
"""

from __future__ import annotations

import json
import time
import urllib.error
import urllib.request
from dataclasses import dataclass, replace
from typing import Callable

from tenantguard.audit import TenantEvent, sign

_EVENTS_PATH = "/v1/events"


class AuditForgeError(Exception):
    """Raised when an event cannot be delivered: rejected, unacknowledged, or undeliverable."""


class _Retry(Exception):
    """Internal signal that an attempt failed but is worth retrying."""


def _default_backoff(attempt: int) -> float:
    """Exponential backoff in seconds for a 1-based attempt number (0.1s, 0.2s, ...)."""
    return 0.1 * (2 ** (attempt - 1))


@dataclass
class AuditForgeHook:
    """An :class:`~tenantguard.audit.AuditHook` that signs and delivers events to AuditForge.

    A ``duplicate`` acknowledgement is an idempotent success; a ``rejected`` event, or one
    absent from every ack partition, raises :class:`AuditForgeError` so a silent drop can
    never masquerade as success in a compliance log. Network errors and 5xx responses are
    retried (4xx are not) up to ``max_retries`` with backoff.
    """

    endpoint: str
    key_id: str
    secret: bytes
    max_retries: int = 2
    timeout: float = 10.0
    backoff: Callable[[int], float] = _default_backoff

    def publish(self, event: TenantEvent) -> None:
        signed = replace(event, signature=sign(event, self.secret))
        body = (
            '{"keyId":' + json.dumps(self.key_id, ensure_ascii=False)
            + ',"events":[' + signed.to_json() + "]}"
        ).encode("utf-8")
        ack = self._post(body)
        _check_ack(event.event_id, ack)

    def _post(self, body: bytes) -> dict:
        """Deliver ``body``, retrying retryable failures up to ``max_retries``."""
        url = self.endpoint.rstrip("/") + _EVENTS_PATH
        last_error = None
        for attempt in range(self.max_retries + 1):
            if attempt > 0:
                time.sleep(self.backoff(attempt))
            try:
                return self._attempt(url, body)
            except _Retry as exc:
                last_error = exc
        raise AuditForgeError(
            f"giving up after {self.max_retries + 1} attempts: {last_error}")

    def _attempt(self, url: str, body: bytes) -> dict:
        """Make one POST, raising :class:`_Retry` for failures worth retrying."""
        request = urllib.request.Request(
            url, data=body, method="POST", headers={"Content-Type": "application/json"})
        try:
            with urllib.request.urlopen(request, timeout=self.timeout) as response:
                return _parse_ack(response.read().decode("utf-8"))
        except urllib.error.HTTPError as exc:
            detail = exc.read().decode("utf-8", "replace")
            if exc.code >= 500:
                raise _Retry(f"server error {exc.code}: {detail}") from exc
            # 4xx is a transport/auth failure (bad key, malformed batch); retrying won't help.
            raise AuditForgeError(f"request rejected {exc.code}: {detail}") from exc
        except urllib.error.URLError as exc:
            raise _Retry(f"POST failed: {exc.reason}") from exc


def _parse_ack(payload: str) -> dict:
    ack = json.loads(payload)
    if not isinstance(ack, dict):
        raise AuditForgeError("malformed ack: not a JSON object")
    return ack


def _check_ack(event_id: str, ack: dict) -> None:
    """Raise if ``event_id`` was rejected or is absent from every ack partition."""
    for rejection in ack.get("rejected", []):
        if rejection.get("eventId") == event_id:
            raise AuditForgeError(f"event {event_id} rejected: {rejection.get('reason')}")
    if event_id in ack.get("accepted", []) or event_id in ack.get("duplicate", []):
        return
    raise AuditForgeError(
        f"event {event_id} missing from ack (received={ack.get('received')})")
