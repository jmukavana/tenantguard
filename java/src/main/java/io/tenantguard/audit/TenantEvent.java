package io.tenantguard.audit;

import java.security.SecureRandom;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.TreeMap;

/**
 * Structured audit record emitted for every tenant-aware operation (core/spec.md). Its
 * JSON encoding, produced by {@link #toJson()}, is the cross-language wire contract and
 * mirrors core/schemas/TenantEvent.schema.json: required keys are always present, empty
 * optional keys are omitted, and metadata keys are sorted for determinism.
 *
 * <p>Serialization is hand-rolled so the core carries no JSON dependency.
 *
 * @param eventId   unique id / idempotency key (required)
 * @param eventType dotted event name, e.g. {@code tenant.resolved} (required)
 * @param tenantId  tenant the event belongs to (required)
 * @param timestamp event time, serialized as ISO-8601 UTC (required)
 * @param outcome   result of the operation (required)
 * @param traceId   W3C trace-id, or "" to omit
 * @param actor     principal that triggered the operation, or "" to omit
 * @param resource  target of the operation, or "" to omit
 * @param metadata  additional structured context; empty to omit
 * @param signature detached signature, or "" to omit
 */
public record TenantEvent(String eventId, String eventType, String tenantId, Instant timestamp,
                          Outcome outcome, String traceId, String actor, String resource,
                          Map<String, String> metadata, String signature) {

    private static final SecureRandom RANDOM = new SecureRandom();

    /** Validating, normalizing canonical constructor. */
    public TenantEvent {
        Objects.requireNonNull(eventId, "eventId");
        Objects.requireNonNull(eventType, "eventType");
        Objects.requireNonNull(tenantId, "tenantId");
        Objects.requireNonNull(timestamp, "timestamp");
        Objects.requireNonNull(outcome, "outcome");
        traceId = traceId == null ? "" : traceId;
        actor = actor == null ? "" : actor;
        resource = resource == null ? "" : resource;
        signature = signature == null ? "" : signature;
        metadata = metadata == null ? Map.of() : Map.copyOf(metadata);
    }

    /** Builds an event stamped with a fresh id and the current UTC time. */
    public static TenantEvent create(String eventType, String tenantId, String traceId, Outcome outcome) {
        return new TenantEvent(newEventId(), eventType, tenantId, Instant.now(), outcome,
                traceId, "", "", Map.of(), "");
    }

    private static String newEventId() {
        byte[] b = new byte[16];
        RANDOM.nextBytes(b);
        StringBuilder sb = new StringBuilder(32);
        for (byte x : b) {
            sb.append(Character.forDigit((x >> 4) & 0xF, 16)).append(Character.forDigit(x & 0xF, 16));
        }
        return sb.toString();
    }

    /** Serializes this event to canonical JSON (the AuditForge wire form). */
    public String toJson() {
        List<String> members = new ArrayList<>();
        members.add(str("eventId", eventId));
        members.add(str("eventType", eventType));
        members.add(str("tenantId", tenantId));
        members.add(raw("timestamp", quote(timestamp.toString())));
        members.add(str("outcome", outcome.value()));
        if (!traceId.isEmpty()) {
            members.add(str("traceId", traceId));
        }
        if (!actor.isEmpty()) {
            members.add(str("actor", actor));
        }
        if (!resource.isEmpty()) {
            members.add(str("resource", resource));
        }
        if (!metadata.isEmpty()) {
            members.add(raw("metadata", metadataJson()));
        }
        if (!signature.isEmpty()) {
            members.add(str("signature", signature));
        }
        return "{" + String.join(",", members) + "}";
    }

    private String metadataJson() {
        List<String> entries = new ArrayList<>();
        for (Map.Entry<String, String> e : new TreeMap<>(metadata).entrySet()) {
            entries.add(quote(e.getKey()) + ":" + quote(e.getValue()));
        }
        return "{" + String.join(",", entries) + "}";
    }

    private static String str(String key, String value) {
        return quote(key) + ":" + quote(value);
    }

    private static String raw(String key, String rawJson) {
        return quote(key) + ":" + rawJson;
    }

    /** Returns {@code s} as a quoted, escaped JSON string. */
    private static String quote(String s) {
        StringBuilder sb = new StringBuilder(s.length() + 2).append('"');
        for (int i = 0; i < s.length(); i++) {
            char c = s.charAt(i);
            switch (c) {
                case '"' -> sb.append("\\\"");
                case '\\' -> sb.append("\\\\");
                case '\n' -> sb.append("\\n");
                case '\r' -> sb.append("\\r");
                case '\t' -> sb.append("\\t");
                default -> {
                    if (c < 0x20) {
                        sb.append(String.format("\\u%04x", (int) c));
                    } else {
                        sb.append(c);
                    }
                }
            }
        }
        return sb.append('"').toString();
    }
}
