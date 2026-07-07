package io.tenantguard.conformance;

import com.fasterxml.jackson.databind.JsonNode;
import io.tenantguard.TenantContext;
import io.tenantguard.TenantContextException;
import org.junit.jupiter.api.DynamicTest;
import org.junit.jupiter.api.TestFactory;

import java.io.IOException;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

/**
 * Runs the shared tenant-context.json fixture against the Java implementation. This is the
 * Java mirror of go/tenantguard/conformance; both must pass the same vectors.
 */
class TenantContextConformanceTest {

    @TestFactory
    Stream<DynamicTest> tenantContext() throws IOException {
        JsonNode root = Fixtures.load("tenant-context.json");
        List<DynamicTest> tests = new ArrayList<>();
        for (JsonNode c : root.get("cases")) {
            tests.add(DynamicTest.dynamicTest(c.get("name").asText(), () -> runCase(c)));
        }
        return tests.stream();
    }

    private void runCase(JsonNode c) {
        JsonNode in = c.get("input");
        JsonNode expect = c.get("expect");

        String tenantId = Fixtures.text(in, "tenantId");
        String tenantName = Fixtures.text(in, "tenantName");
        String traceId = Fixtures.text(in, "traceId");
        Map<String, String> metadata = Fixtures.map(in.get("metadata"));

        if (expect.has("error")) {
            TenantContextException e = assertThrows(TenantContextException.class,
                    () -> TenantContext.of(tenantId, tenantName, traceId, metadata));
            assertEquals(expect.get("error").asText(), e.code());
            return;
        }

        TenantContext tc = TenantContext.of(tenantId, tenantName, traceId, metadata);
        assertEquals(expect.get("valid").asBoolean(), tc.isValid());
        assertEquals(Fixtures.text(expect, "tenantId"), tc.tenantId());
        assertEquals(Fixtures.text(expect, "tenantName"), tc.tenantName());
        assertEquals(Fixtures.text(expect, "traceId"), tc.traceId());
        assertEquals(Fixtures.map(expect.get("metadata")), tc.metadata());
    }
}
