"""Exercises AuditForgeHook against an in-process fake AuditForge (stdlib http.server)."""

from __future__ import annotations

import json
import threading
from contextlib import contextmanager
from datetime import datetime
from http.server import BaseHTTPRequestHandler, HTTPServer
from typing import Callable, Iterator

import pytest

from tenantguard.audit import Outcome, TenantEvent, verify
from tenantguard.audit.auditforge import AuditForgeError, AuditForgeHook

SECRET = b"tenantguard-shared-secret"

# A responder maps (attempt, request) to an (http_status, response_body) pair.
Responder = Callable[[int, dict], "tuple[int, str]"]


class _Handler(BaseHTTPRequestHandler):
    def do_POST(self) -> None:  # noqa: N802 (stdlib naming)
        server = self.server
        server.calls += 1
        length = int(self.headers.get("Content-Length", 0))
        request = json.loads(self.rfile.read(length))
        server.last_path = self.path
        server.last_request = request

        status, body = server.responder(server.calls, request)
        data = body.encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def log_message(self, *args: object) -> None:  # silence the test server
        pass


@contextmanager
def fake_auditforge(responder: Responder) -> Iterator[tuple[HTTPServer, str]]:
    server = HTTPServer(("localhost", 0), _Handler)
    server.responder = responder
    server.calls = 0
    server.last_path = None
    server.last_request = None
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield server, f"http://localhost:{server.server_address[1]}"
    finally:
        server.shutdown()
        server.server_close()


def _hook(endpoint: str, max_retries: int) -> AuditForgeHook:
    return AuditForgeHook(endpoint, "k1", SECRET, max_retries=max_retries, backoff=lambda a: 0.0)


def _event_from(wire: dict) -> TenantEvent:
    return TenantEvent(
        event_id=wire["eventId"],
        event_type=wire["eventType"],
        tenant_id=wire["tenantId"],
        timestamp=datetime.fromisoformat(wire["timestamp"]),
        outcome=Outcome(wire["outcome"]),
        trace_id=wire.get("traceId", ""),
        actor=wire.get("actor", ""),
        resource=wire.get("resource", ""),
        metadata=wire.get("metadata") or {},
        signature=wire.get("signature", ""),
    )


def _ack(**partitions: object) -> str:
    base = {"received": 1, "accepted": [], "duplicate": [], "rejected": []}
    base.update(partitions)
    return json.dumps(base)


def test_signs_and_accepts() -> None:
    def responder(attempt: int, request: dict) -> tuple[int, str]:
        return 200, _ack(accepted=[request["events"][0]["eventId"]])

    with fake_auditforge(responder) as (server, url):
        event = TenantEvent.create("tenant.resolved", "acme", "", Outcome.ALLOWED)
        _hook(url, 2).publish(event)

        assert server.calls == 1
        assert server.last_path == "/v1/events"
        assert server.last_request["keyId"] == "k1"
        sent = server.last_request["events"][0]
        assert sent.get("signature")
        # The server verifies exactly as AuditForge would.
        assert verify(_event_from(sent), SECRET, sent["signature"])


def test_duplicate_is_idempotent_success() -> None:
    def responder(attempt: int, request: dict) -> tuple[int, str]:
        return 200, _ack(duplicate=[request["events"][0]["eventId"]])

    with fake_auditforge(responder) as (_server, url):
        _hook(url, 0).publish(TenantEvent.create("x", "acme", "", Outcome.ALLOWED))


def test_rejected_is_error() -> None:
    def responder(attempt: int, request: dict) -> tuple[int, str]:
        rejection = {"eventId": request["events"][0]["eventId"], "reason": "invalid_signature"}
        return 200, _ack(rejected=[rejection])

    with fake_auditforge(responder) as (_server, url):
        with pytest.raises(AuditForgeError, match="invalid_signature"):
            _hook(url, 0).publish(TenantEvent.create("x", "acme", "", Outcome.ALLOWED))


def test_unacknowledged_is_error() -> None:
    def responder(attempt: int, request: dict) -> tuple[int, str]:
        return 200, _ack()

    with fake_auditforge(responder) as (_server, url):
        with pytest.raises(AuditForgeError, match="missing from ack"):
            _hook(url, 0).publish(TenantEvent.create("x", "acme", "", Outcome.ALLOWED))


def test_retries_on_5xx() -> None:
    def responder(attempt: int, request: dict) -> tuple[int, str]:
        if attempt == 1:
            return 503, "unavailable"
        return 200, _ack(accepted=[request["events"][0]["eventId"]])

    with fake_auditforge(responder) as (server, url):
        _hook(url, 2).publish(TenantEvent.create("x", "acme", "", Outcome.ALLOWED))
        assert server.calls == 2


def test_does_not_retry_4xx() -> None:
    def responder(attempt: int, request: dict) -> tuple[int, str]:
        return 401, "unknown key"

    with fake_auditforge(responder) as (server, url):
        with pytest.raises(AuditForgeError, match="401"):
            _hook(url, 3).publish(TenantEvent.create("x", "acme", "", Outcome.ALLOWED))
        assert server.calls == 1


def test_exhausts_retries() -> None:
    def responder(attempt: int, request: dict) -> tuple[int, str]:
        return 502, "down"

    with fake_auditforge(responder) as (server, url):
        with pytest.raises(AuditForgeError, match="giving up after 3 attempts"):
            _hook(url, 2).publish(TenantEvent.create("x", "acme", "", Outcome.ALLOWED))
        assert server.calls == 3
