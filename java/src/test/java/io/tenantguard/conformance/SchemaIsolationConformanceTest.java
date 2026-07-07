package io.tenantguard.conformance;

import com.fasterxml.jackson.databind.JsonNode;
import io.tenantguard.TenantContext;
import io.tenantguard.isolation.IsolationException;
import io.tenantguard.isolation.SchemaStrategy;
import org.junit.jupiter.api.DynamicTest;
import org.junit.jupiter.api.TestFactory;

import java.io.IOException;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

/** Runs the shared isolation-schema.json fixture against the Java SchemaStrategy. */
class SchemaIsolationConformanceTest {

    @TestFactory
    Stream<DynamicTest> schemaIsolation() throws IOException {
        JsonNode root = Fixtures.load("isolation-schema.json");
        List<DynamicTest> tests = new ArrayList<>();
        for (JsonNode c : root.get("cases")) {
            tests.add(DynamicTest.dynamicTest(c.get("name").asText(), () -> runCase(c)));
        }
        return tests.stream();
    }

    private void runCase(JsonNode c) {
        TenantContext tc = TenantContext.of(c.get("tenantId").asText(), "", "", null);
        SchemaStrategy strategy = new SchemaStrategy(Fixtures.text(c, "prefix"));
        JsonNode expect = c.get("expect");

        if (expect.has("error")) {
            IsolationException e = assertThrows(IsolationException.class, () -> strategy.schemaName(tc));
            assertEquals(expect.get("error").asText(), e.code());
            return;
        }
        assertEquals(expect.get("schema").asText(), strategy.schemaName(tc));
    }
}
