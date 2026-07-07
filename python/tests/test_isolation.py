"""Unit tests for schema isolation."""

from __future__ import annotations

import pytest

from tenantguard.context import TenantContext
from tenantguard.isolation import UNSAFE_TENANT_ID, IsolationError, SchemaStrategy


class _FakeCursor:
    def __init__(self) -> None:
        self.executed: list[str] = []

    def execute(self, sql: str, *args) -> None:
        self.executed.append(sql)


def test_search_path_sql_quotes_schema() -> None:
    sql = SchemaStrategy("tenant_").search_path_sql(TenantContext("acme"))
    assert sql == 'SET search_path TO "tenant_acme"'


def test_injection_in_tenant_id_is_rejected() -> None:
    with pytest.raises(IsolationError) as excinfo:
        SchemaStrategy("tenant_").schema_name(
            TenantContext('acme"; DROP SCHEMA public CASCADE; --')
        )
    assert excinfo.value.code == UNSAFE_TENANT_ID


def test_bind_executes_set_statement() -> None:
    cursor = _FakeCursor()
    SchemaStrategy("tenant_").bind(cursor, TenantContext("acme"))
    assert cursor.executed == ['SET search_path TO "tenant_acme"']
