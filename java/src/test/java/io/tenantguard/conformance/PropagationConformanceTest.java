package io.tenantguard.conformance;

import com.fasterxml.jackson.databind.JsonNode;
import io.opentelemetry.api.baggage.Baggage;
import io.opentelemetry.api.baggage.propagation.W3CBaggagePropagator;
import io.opentelemetry.context.Context;
import io.opentelemetry.context.propagation.TextMapGetter;
import io.tenantguard.TenantContext;
import io.tenantguard.propagation.Propagator;
import org.junit.jupiter.api.DynamicTest;
import org.junit.jupiter.api.TestFactory;

import java.io.IOException;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Runs the shared propagation-baggage.json fixture against the Java Propagator. */
class PropagationConformanceTest {

    private static final TextMapGetter<Map<String, String>> GETTER = new TextMapGetter<>() {
        @Override
        public Iterable<String> keys(Map<String, String> carrier) {
            return carrier.keySet();
        }

        @Override
        public String get(Map<String, String> carrier, String key) {
            return carrier.get(key);
        }
    };

    private final Propagator propagator = new Propagator();

    @TestFactory
    Stream<DynamicTest> propagation() throws IOException {
        JsonNode root = Fixtures.load("propagation-baggage.json");
        List<DynamicTest> tests = new ArrayList<>();
        for (JsonNode c : root.get("cases")) {
            tests.add(DynamicTest.dynamicTest(c.get("name").asText(), () -> runCase(c)));
        }
        return tests.stream();
    }

    private void runCase(JsonNode c) {
        JsonNode ctxNode = c.get("context");
        TenantContext tc = TenantContext.of(
                ctxNode.get("tenantId").asText(),
                Fixtures.text(ctxNode, "tenantName"),
                Fixtures.text(ctxNode, "traceId"),
                Fixtures.map(ctxNode.get("metadata")));

        Map<String, String> carrier = new HashMap<>();
        propagator.inject(tc, carrier);

        Baggage bag = Baggage.fromContext(
                W3CBaggagePropagator.getInstance().extract(Context.root(), carrier, GETTER));

        Map<String, String> expectMembers = Fixtures.map(c.get("expectMembers"));
        // Every expected member present with the exact value...
        for (Map.Entry<String, String> e : expectMembers.entrySet()) {
            assertEquals(e.getValue(), bag.getEntryValue(e.getKey()), "member " + e.getKey());
        }
        // ...and no unexpected tg.* member leaks.
        bag.asMap().forEach((key, entry) -> {
            if (key.startsWith("tg.")) {
                assertTrue(expectMembers.containsKey(key), "unexpected tenant member " + key);
            }
        });

        // Round-trip reconstructs an equal context.
        TenantContext got = propagator.extract(carrier).orElseThrow();
        assertEquals(tc.tenantId(), got.tenantId());
        assertEquals(tc.tenantName(), got.tenantName());
        assertEquals(tc.traceId(), got.traceId());
        assertEquals(tc.metadata(), got.metadata());
    }
}
