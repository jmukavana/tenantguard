package io.tenantguard.isolation;

import io.tenantguard.TenantContext;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

class SchemaStrategyTest {

    private static TenantContext tenant(String id) {
        return TenantContext.of(id, "", "", null);
    }

    @Test
    void bindStatementQuotesTheSchema() {
        // Same package, so the package-private SQL builder is visible — asserts the exact
        // statement without needing a live database.
        String sql = new SchemaStrategy("tenant_").searchPathSql(tenant("acme"));
        assertEquals("SET search_path TO \"tenant_acme\"", sql);
    }

    @Test
    void injectionInTenantIdIsRejected() {
        SchemaStrategy strategy = new SchemaStrategy("tenant_");
        IsolationException e = assertThrows(IsolationException.class,
                () -> strategy.schemaName(tenant("acme\"; DROP SCHEMA public CASCADE; --")));
        assertEquals(IsolationException.UNSAFE_TENANT_ID, e.code());
    }
}
