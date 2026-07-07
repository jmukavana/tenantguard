"""Loads the shared, language-neutral compliance fixtures.

The fixtures (testkit/compliance-tests/fixtures) define TenantGuard conformance across all
language bindings; this is the Python mirror of the Go and Java loaders.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any


def fixtures_dir() -> Path:
    """Locate the fixtures directory by walking up from this file."""
    for parent in Path(__file__).resolve().parents:
        candidate = parent / "testkit" / "compliance-tests" / "fixtures"
        if candidate.is_dir():
            return candidate
    raise RuntimeError("could not locate testkit/compliance-tests/fixtures")


def load(name: str) -> Any:
    """Read and parse the named fixture file (e.g. 'tenant-context.json')."""
    return json.loads((fixtures_dir() / name).read_text(encoding="utf-8"))


def cases(name: str) -> list[dict]:
    """Return the ``cases`` list from a fixture file."""
    return load(name)["cases"]
