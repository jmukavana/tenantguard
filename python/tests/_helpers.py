"""Shared test doubles."""

from __future__ import annotations

from typing import Optional


class FakeRequest:
    """Test double for the resolver HttpRequestView with case-insensitive headers."""

    def __init__(self, headers: Optional[dict] = None, host: Optional[str] = None) -> None:
        self._headers = {k.lower(): v for k, v in (headers or {}).items()}
        self._host = host

    def header(self, name: str) -> Optional[str]:
        return self._headers.get(name.lower())

    def host(self) -> Optional[str]:
        return self._host
