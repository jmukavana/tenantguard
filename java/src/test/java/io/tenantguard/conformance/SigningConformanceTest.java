package io.tenantguard.conformance;

import com.fasterxml.jackson.databind.JsonNode;
import io.tenantguard.audit.Signing;
import io.tenantguard.audit.TenantEvent;
import org.junit.jupiter.api.DynamicTest;
import org.junit.jupiter.api.TestFactory;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Runs the shared auditforge-signing.json golden vectors against {@link Signing}. */
class SigningConformanceTest {

    @TestFactory
    Stream<DynamicTest> signing() throws IOException {
        JsonNode root = Fixtures.load("auditforge-signing.json");
        byte[] secret = root.get("secret").asText().getBytes(StandardCharsets.UTF_8);
        List<DynamicTest> tests = new ArrayList<>();
        for (JsonNode c : root.get("cases")) {
            tests.add(DynamicTest.dynamicTest(c.get("name").asText(), () -> runCase(c, secret)));
        }
        return tests.stream();
    }

    private void runCase(JsonNode c, byte[] secret) {
        TenantEvent event = Fixtures.event(c.get("event"));

        // Canonical bytes and the HMAC are checked separately so a mismatch localizes to
        // serialization vs. the signing primitive.
        assertEquals(c.get("canonical").asText(), Signing.canonicalJson(event), "canonical form");
        assertEquals(c.get("signature").asText(), Signing.sign(event, secret), "signature");

        assertTrue(Signing.verify(event, secret, c.get("signature").asText()),
                "verify rejected the golden signature");
        assertFalse(Signing.verify(event, "wrong-secret".getBytes(StandardCharsets.UTF_8),
                c.get("signature").asText()), "verify accepted the wrong secret");
    }
}
