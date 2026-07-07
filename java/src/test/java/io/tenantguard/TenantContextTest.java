package io.tenantguard;

import org.junit.jupiter.api.Test;

import java.util.HashMap;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * Security-critical guard: a TenantContext must not change after construction. Mirrors the
 * Go package's TestImmutable.
 */
class TenantContextTest {

    @Test
    void mutatingSourceMapDoesNotLeak() {
        Map<String, String> src = new HashMap<>();
        src.put("region", "ke");
        src.put("tier", "gold");

        TenantContext tc = TenantContext.of("acme", "Acme", "", src);

        src.put("region", "tz");
        src.put("injected", "evil");

        assertEquals("ke", tc.metadata().get("region"), "context leaked a source-map mutation");
        assertFalse(tc.metadata().containsKey("injected"), "context absorbed a later-added key");
    }

    @Test
    void returnedMetadataIsUnmodifiable() {
        TenantContext tc = TenantContext.of("acme", "", "", Map.of("a", "b"));
        assertThrows(UnsupportedOperationException.class, () -> tc.metadata().put("x", "y"));
    }

    @Test
    void emptyTenantIdThrows() {
        TenantContextException e = assertThrows(TenantContextException.class,
                () -> TenantContext.of("", "", "", null));
        assertEquals(TenantContextException.EMPTY_TENANT_ID, e.code());
    }

    @Test
    void malformedTraceIdThrows() {
        TenantContextException e = assertThrows(TenantContextException.class,
                () -> TenantContext.of("acme", "", "not-a-trace", null));
        assertEquals(TenantContextException.INVALID_TRACE_ID, e.code());
    }

    @Test
    void validContextExposesFields() {
        TenantContext tc = TenantContext.of("acme", "Acme", "4bf92f3577b34da6a3ce929d0e0e4736", null);
        assertTrue(tc.isValid());
        assertEquals("acme", tc.tenantId());
        assertEquals("4bf92f3577b34da6a3ce929d0e0e4736", tc.traceId());
        assertEquals(Map.of(), tc.metadata());
    }
}
