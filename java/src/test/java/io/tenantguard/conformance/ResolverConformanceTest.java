package io.tenantguard.conformance;

import com.fasterxml.jackson.databind.JsonNode;
import io.tenantguard.TenantContext;
import io.tenantguard.resolver.FakeRequest;
import io.tenantguard.resolver.HeaderResolver;
import io.tenantguard.resolver.SubdomainResolver;
import org.junit.jupiter.api.DynamicTest;
import org.junit.jupiter.api.TestFactory;

import java.io.IOException;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * Runs the shared resolver fixtures against the Java resolvers. Java mirror of the header
 * and subdomain resolver cases in go/tenantguard/conformance.
 */
class ResolverConformanceTest {

    @TestFactory
    Stream<DynamicTest> headerResolver() throws IOException {
        return cases("resolver-header.json", this::runHeaderCase);
    }

    @TestFactory
    Stream<DynamicTest> subdomainResolver() throws IOException {
        return cases("resolver-subdomain.json", this::runSubdomainCase);
    }

    private void runHeaderCase(JsonNode c) {
        FakeRequest req = new FakeRequest();
        c.get("headers").fields().forEachRemaining(e -> req.header(e.getKey(), e.getValue().asText()));

        Optional<TenantContext> result = new HeaderResolver(Fixtures.text(c, "header")).resolve(req);
        JsonNode expect = c.get("expect");

        if (expect.has("error")) {
            assertEquals("no_tenant", expect.get("error").asText());
            assertTrue(result.isEmpty(), "expected no tenant");
            return;
        }
        assertTrue(result.isPresent(), "expected a tenant");
        assertEquals(Fixtures.text(expect, "tenantId"), result.get().tenantId());
        assertEquals(Fixtures.text(expect, "traceId"), result.get().traceId());
    }

    private void runSubdomainCase(JsonNode c) {
        FakeRequest req = new FakeRequest().host(c.get("host").asText());
        List<String> ignore = Fixtures.list(c.get("ignore"));
        SubdomainResolver resolver =
                new SubdomainResolver(c.get("baseDomain").asText(), ignore.isEmpty() ? null : ignore);

        Optional<TenantContext> result = resolver.resolve(req);
        JsonNode expect = c.get("expect");

        if (expect.has("error")) {
            assertEquals("no_tenant", expect.get("error").asText());
            assertTrue(result.isEmpty(), "expected no tenant");
            return;
        }
        assertTrue(result.isPresent(), "expected a tenant");
        assertEquals(Fixtures.text(expect, "tenantId"), result.get().tenantId());
    }

    private Stream<DynamicTest> cases(String fixture, java.util.function.Consumer<JsonNode> run)
            throws IOException {
        JsonNode root = Fixtures.load(fixture);
        List<DynamicTest> tests = new ArrayList<>();
        for (JsonNode c : root.get("cases")) {
            tests.add(DynamicTest.dynamicTest(c.get("name").asText(), () -> run.accept(c)));
        }
        return tests.stream();
    }
}
