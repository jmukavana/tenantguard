package io.tenantguard.audit.auditforge;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpHandler;
import com.sun.net.httpserver.HttpServer;
import io.tenantguard.audit.Outcome;
import io.tenantguard.audit.Signing;
import io.tenantguard.audit.TenantEvent;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.net.http.HttpClient;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.time.Instant;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Exercises AuditForgeHook against an in-process fake AuditForge (JDK HttpServer). */
class AuditForgeHookTest {

    private static final byte[] SECRET = "tenantguard-shared-secret".getBytes(StandardCharsets.UTF_8);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private HttpServer server;
    private final AtomicInteger calls = new AtomicInteger();
    private volatile String lastMethod;
    private volatile String lastKeyId;
    private volatile TenantEvent received;

    @AfterEach
    void stop() {
        if (server != null) {
            server.stop(0);
        }
    }

    /** Decides the response for a given attempt; state is captured for assertions in the test. */
    private interface Responder {
        void respond(HttpExchange exchange, JsonNode request, int attempt) throws IOException;
    }

    private String start(Responder responder) throws IOException {
        server = HttpServer.create(new InetSocketAddress("localhost", 0), 0);
        server.createContext("/v1/events", (HttpHandler) exchange -> {
            int attempt = calls.incrementAndGet();
            JsonNode request = MAPPER.readTree(exchange.getRequestBody().readAllBytes());
            lastMethod = exchange.getRequestMethod();
            lastKeyId = request.get("keyId").asText();
            received = event(request.get("events").get(0));
            responder.respond(exchange, request, attempt);
        });
        server.start();
        return "http://localhost:" + server.getAddress().getPort();
    }

    private AuditForgeHook hook(String endpoint, int maxRetries) {
        return new AuditForgeHook(endpoint, "k1", SECRET, HttpClient.newHttpClient(),
                maxRetries, attempt -> Duration.ZERO);
    }

    @Test
    void signsAndAccepts() throws IOException {
        String url = start((exchange, request, attempt) ->
                writeJson(exchange, 200, accepted(eventId(request))));

        TenantEvent event = TenantEvent.create("tenant.resolved", "acme", "", Outcome.ALLOWED);
        hook(url, 2).publish(event);

        assertEquals(1, calls.get());
        assertEquals("POST", lastMethod);
        assertEquals("k1", lastKeyId);
        assertFalse(received.signature().isEmpty(), "transported event carries no signature");
        assertTrue(Signing.verify(received, SECRET, received.signature()),
                "server-side signature must verify");
    }

    @Test
    void duplicateIsIdempotentSuccess() throws IOException {
        String url = start((exchange, request, attempt) -> writeJson(exchange, 200,
                "{\"received\":1,\"accepted\":[],\"duplicate\":[\"" + eventId(request) + "\"],\"rejected\":[]}"));

        hook(url, 0).publish(TenantEvent.create("x", "acme", "", Outcome.ALLOWED));
    }

    @Test
    void rejectedIsError() throws IOException {
        String url = start((exchange, request, attempt) -> writeJson(exchange, 200,
                "{\"received\":1,\"accepted\":[],\"duplicate\":[],\"rejected\":"
                        + "[{\"eventId\":\"" + eventId(request) + "\",\"reason\":\"invalid_signature\"}]}"));

        AuditForgeException e = assertThrows(AuditForgeException.class,
                () -> hook(url, 0).publish(TenantEvent.create("x", "acme", "", Outcome.ALLOWED)));
        assertTrue(e.getMessage().contains("invalid_signature"), e.getMessage());
    }

    @Test
    void unacknowledgedIsError() throws IOException {
        String url = start((exchange, request, attempt) ->
                writeJson(exchange, 200, "{\"received\":1,\"accepted\":[],\"duplicate\":[],\"rejected\":[]}"));

        AuditForgeException e = assertThrows(AuditForgeException.class,
                () -> hook(url, 0).publish(TenantEvent.create("x", "acme", "", Outcome.ALLOWED)));
        assertTrue(e.getMessage().contains("missing from ack"), e.getMessage());
    }

    @Test
    void retriesOn5xx() throws IOException {
        String url = start((exchange, request, attempt) -> {
            if (attempt == 1) {
                writeJson(exchange, 503, "unavailable");
            } else {
                writeJson(exchange, 200, accepted(eventId(request)));
            }
        });

        hook(url, 2).publish(TenantEvent.create("x", "acme", "", Outcome.ALLOWED));
        assertEquals(2, calls.get(), "one 503 then one 200");
    }

    @Test
    void doesNotRetry4xx() throws IOException {
        String url = start((exchange, request, attempt) -> writeJson(exchange, 401, "unknown key"));

        AuditForgeException e = assertThrows(AuditForgeException.class,
                () -> hook(url, 3).publish(TenantEvent.create("x", "acme", "", Outcome.ALLOWED)));
        assertTrue(e.getMessage().contains("401"), e.getMessage());
        assertEquals(1, calls.get(), "4xx must not retry");
    }

    @Test
    void exhaustsRetries() throws IOException {
        String url = start((exchange, request, attempt) -> writeJson(exchange, 502, "down"));

        AuditForgeException e = assertThrows(AuditForgeException.class,
                () -> hook(url, 2).publish(TenantEvent.create("x", "acme", "", Outcome.ALLOWED)));
        assertTrue(e.getMessage().contains("giving up after 3 attempts"), e.getMessage());
        assertEquals(3, calls.get(), "initial + 2 retries");
    }

    private static String eventId(JsonNode request) {
        return request.get("events").get(0).get("eventId").asText();
    }

    private static String accepted(String eventId) {
        return "{\"received\":1,\"accepted\":[\"" + eventId + "\"],\"duplicate\":[],\"rejected\":[]}";
    }

    private static TenantEvent event(JsonNode ev) {
        Map<String, String> metadata = new LinkedHashMap<>();
        if (ev.has("metadata")) {
            ev.get("metadata").fields().forEachRemaining(e -> metadata.put(e.getKey(), e.getValue().asText()));
        }
        return new TenantEvent(
                ev.get("eventId").asText(),
                ev.get("eventType").asText(),
                ev.get("tenantId").asText(),
                Instant.parse(ev.get("timestamp").asText()),
                Outcome.fromValue(ev.get("outcome").asText()),
                ev.path("traceId").asText(""),
                ev.path("actor").asText(""),
                ev.path("resource").asText(""),
                metadata,
                ev.path("signature").asText(""));
    }

    private static void writeJson(HttpExchange exchange, int status, String body) throws IOException {
        byte[] out = body.getBytes(StandardCharsets.UTF_8);
        exchange.getResponseHeaders().set("Content-Type", "application/json");
        exchange.sendResponseHeaders(status, out.length);
        try (OutputStream os = exchange.getResponseBody()) {
            os.write(out);
        }
    }
}
