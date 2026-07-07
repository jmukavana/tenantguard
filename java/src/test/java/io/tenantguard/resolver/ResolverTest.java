package io.tenantguard.resolver;

import io.tenantguard.TenantContext;
import org.junit.jupiter.api.Test;

import java.util.List;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * Idiomatic-usage tests for the resolvers and the HttpRequestView port. Behavioral
 * coverage lives in the fixture-driven conformance tests; these document the API.
 */
class ResolverTest {

    @Test
    void headerResolverReadsCustomHeaderAndTraceparent() {
        FakeRequest req = new FakeRequest()
                .header("X-Org", "beta")
                .header("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01");

        Optional<TenantContext> tc = new HeaderResolver("X-Org").resolve(req);

        assertTrue(tc.isPresent());
        assertEquals("beta", tc.get().tenantId());
        assertEquals("4bf92f3577b34da6a3ce929d0e0e4736", tc.get().traceId());
    }

    @Test
    void headerResolverEmptyWhenHeaderMissing() {
        assertTrue(new HeaderResolver().resolve(new FakeRequest()).isEmpty());
    }

    @Test
    void subdomainResolverHonoursCustomIgnoreList() {
        SubdomainResolver resolver = new SubdomainResolver("example.com", List.of("admin", "www"));
        assertTrue(resolver.resolve(new FakeRequest().host("admin.example.com")).isEmpty());
        assertEquals("acme",
                resolver.resolve(new FakeRequest().host("acme.example.com")).orElseThrow().tenantId());
    }
}
