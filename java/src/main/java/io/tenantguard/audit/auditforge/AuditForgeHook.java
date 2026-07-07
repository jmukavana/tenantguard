package io.tenantguard.audit.auditforge;

import io.tenantguard.audit.AuditHook;
import io.tenantguard.audit.Signing;
import io.tenantguard.audit.TenantEvent;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.List;
import java.util.Objects;
import java.util.function.IntFunction;

/**
 * An {@link AuditHook} that signs each event and POSTs it to AuditForge's POST /v1/events
 * (ecosystem/auditforge/spec.md). It is the production replacement for
 * {@link io.tenantguard.audit.LoggingAuditHook}: every event is HMAC-signed with a shared
 * key and appended to the tamper-evident audit log.
 *
 * <p>A {@code duplicate} acknowledgement is an idempotent success; a {@code rejected} event,
 * or one absent from every ack partition, throws {@link AuditForgeException} so a silent drop
 * can never masquerade as success in a compliance log. Network errors and 5xx responses are
 * retried (4xx are not) up to {@code maxRetries} with backoff.
 */
public final class AuditForgeHook implements AuditHook {

    private static final String EVENTS_PATH = "/v1/events";
    private static final Duration REQUEST_TIMEOUT = Duration.ofSeconds(10);

    private final String endpoint;
    private final String keyId;
    private final byte[] secret;
    private final HttpClient client;
    private final int maxRetries;
    private final IntFunction<Duration> backoff;

    /** Creates a hook with a default HTTP client, two retries, and exponential backoff. */
    public AuditForgeHook(String endpoint, String keyId, byte[] secret) {
        this(endpoint, keyId, secret,
                HttpClient.newBuilder().connectTimeout(REQUEST_TIMEOUT).build(),
                2, AuditForgeHook::defaultBackoff);
    }

    /** Creates a fully configured hook. {@code backoff} maps a 1-based attempt to its delay. */
    public AuditForgeHook(String endpoint, String keyId, byte[] secret, HttpClient client,
                          int maxRetries, IntFunction<Duration> backoff) {
        this.endpoint = Objects.requireNonNull(endpoint, "endpoint");
        this.keyId = Objects.requireNonNull(keyId, "keyId");
        this.secret = Objects.requireNonNull(secret, "secret");
        this.client = Objects.requireNonNull(client, "client");
        this.maxRetries = maxRetries;
        this.backoff = Objects.requireNonNull(backoff, "backoff");
    }

    @Override
    public void publish(TenantEvent event) {
        TenantEvent signed = withSignature(event, Signing.sign(event, secret));
        String body = new IngestRequest(keyId, List.of(signed)).toJson();
        IngestAck ack = post(body);
        checkAck(event.eventId(), ack);
    }

    /** Delivers {@code body}, retrying retryable failures up to {@code maxRetries}. */
    private IngestAck post(String body) {
        Retryable last = null;
        for (int attempt = 0; attempt <= maxRetries; attempt++) {
            if (attempt > 0) {
                sleep(backoff.apply(attempt));
            }
            try {
                return attempt(body);
            } catch (Retryable e) {
                last = e;
            }
        }
        throw new AuditForgeException("giving up after " + (maxRetries + 1) + " attempts", last);
    }

    /** Makes one POST, throwing {@link Retryable} for failures worth retrying. */
    private IngestAck attempt(String body) {
        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create(trimTrailingSlash(endpoint) + EVENTS_PATH))
                .timeout(REQUEST_TIMEOUT)
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(body, StandardCharsets.UTF_8))
                .build();

        HttpResponse<String> response;
        try {
            response = client.send(request, HttpResponse.BodyHandlers.ofString(StandardCharsets.UTF_8));
        } catch (IOException e) {
            throw new Retryable("POST failed: " + e.getMessage(), e);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new AuditForgeException("interrupted during POST", e);
        }

        int status = response.statusCode();
        if (status == 200) {
            return IngestAck.fromJson(response.body());
        }
        if (status >= 500) {
            throw new Retryable("server error " + status + ": " + response.body(), null);
        }
        // 4xx is a transport/auth failure (bad key, malformed batch); retrying won't help.
        throw new AuditForgeException("request rejected " + status + ": " + response.body());
    }

    /** Throws if {@code eventId} was rejected or is absent from every ack partition. */
    private static void checkAck(String eventId, IngestAck ack) {
        for (IngestAck.Rejection r : ack.rejected()) {
            if (r.eventId().equals(eventId)) {
                throw new AuditForgeException("event " + eventId + " rejected: " + r.reason());
            }
        }
        if (ack.accepted().contains(eventId) || ack.duplicate().contains(eventId)) {
            return;
        }
        throw new AuditForgeException(
                "event " + eventId + " missing from ack (received=" + ack.received() + ")");
    }

    private static TenantEvent withSignature(TenantEvent e, String signature) {
        return new TenantEvent(e.eventId(), e.eventType(), e.tenantId(), e.timestamp(), e.outcome(),
                e.traceId(), e.actor(), e.resource(), e.metadata(), signature);
    }

    private static void sleep(Duration delay) {
        try {
            Thread.sleep(delay.toMillis());
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new AuditForgeException("interrupted during retry backoff", e);
        }
    }

    private static Duration defaultBackoff(int attempt) {
        return Duration.ofMillis(100L << (attempt - 1));
    }

    private static String trimTrailingSlash(String url) {
        int end = url.length();
        while (end > 0 && url.charAt(end - 1) == '/') {
            end--;
        }
        return url.substring(0, end);
    }

    /** Internal signal that an attempt failed but is worth retrying. */
    private static final class Retryable extends RuntimeException {
        Retryable(String message, Throwable cause) {
            super(message, cause);
        }
    }
}
