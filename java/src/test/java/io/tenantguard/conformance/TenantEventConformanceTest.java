package io.tenantguard.conformance;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.tenantguard.audit.Outcome;
import io.tenantguard.audit.TenantEvent;
import org.junit.jupiter.api.DynamicTest;
import org.junit.jupiter.api.TestFactory;

import java.io.IOException;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Runs the shared tenant-event.json fixture against TenantEvent.toJson(). */
class TenantEventConformanceTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    @TestFactory
    Stream<DynamicTest> tenantEvent() throws IOException {
        JsonNode root = Fixtures.load("tenant-event.json");
        List<DynamicTest> tests = new ArrayList<>();
        for (JsonNode c : root.get("cases")) {
            tests.add(DynamicTest.dynamicTest(c.get("name").asText(), () -> runCase(c)));
        }
        return tests.stream();
    }

    private void runCase(JsonNode c) throws IOException {
        JsonNode ev = c.get("event");
        TenantEvent event = new TenantEvent(
                ev.get("eventId").asText(),
                ev.get("eventType").asText(),
                ev.get("tenantId").asText(),
                Instant.parse(ev.get("timestamp").asText()),
                Outcome.fromValue(ev.get("outcome").asText()),
                Fixtures.text(ev, "traceId"),
                Fixtures.text(ev, "actor"),
                Fixtures.text(ev, "resource"),
                Fixtures.map(ev.get("metadata")),
                Fixtures.text(ev, "signature"));

        JsonNode out = MAPPER.readTree(event.toJson());
        JsonNode expect = c.get("expect");

        for (JsonNode key : expect.get("requiredKeys")) {
            assertTrue(out.has(key.asText()), "required key missing: " + key.asText());
        }
        for (JsonNode key : expect.get("absentKeys")) {
            assertFalse(out.has(key.asText()), "key should be omitted: " + key.asText());
        }
        JsonNode values = expect.get("values");
        values.fields().forEachRemaining(e ->
                assertEquals(e.getValue().asText(), out.get(e.getKey()).asText(), "value for " + e.getKey()));
    }
}
