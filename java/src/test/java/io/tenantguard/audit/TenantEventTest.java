package io.tenantguard.audit;

import org.junit.jupiter.api.Test;

import java.time.Instant;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

class TenantEventTest {

    @Test
    void minimalEventOmitsEmptyOptionalsAndEscapes() {
        TenantEvent e = new TenantEvent("e1", "tenant.resolved", "ac\"me",
                Instant.parse("2026-01-01T00:00:00Z"), Outcome.ALLOWED, "", "", "", Map.of(), "");
        String json = e.toJson();

        assertEquals(
                "{\"eventId\":\"e1\",\"eventType\":\"tenant.resolved\",\"tenantId\":\"ac\\\"me\","
                        + "\"timestamp\":\"2026-01-01T00:00:00Z\",\"outcome\":\"allowed\"}",
                json);
    }

    @Test
    void metadataKeysAreSortedForDeterminism() {
        TenantEvent e = new TenantEvent("e2", "x", "acme",
                Instant.parse("2026-01-01T00:00:00Z"), Outcome.DENIED, "", "", "",
                Map.of("z", "1", "a", "2"), "");
        assertTrue(e.toJson().contains("\"metadata\":{\"a\":\"2\",\"z\":\"1\"}"), e.toJson());
    }

    @Test
    void createGeneratesFreshIdAndTime() {
        TenantEvent a = TenantEvent.create("x", "acme", "", Outcome.ALLOWED);
        TenantEvent b = TenantEvent.create("x", "acme", "", Outcome.ALLOWED);
        assertEquals(32, a.eventId().length());
        assertNotEquals(a.eventId(), b.eventId());
    }
}
