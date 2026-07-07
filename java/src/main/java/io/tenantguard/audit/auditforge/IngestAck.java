package io.tenantguard.audit.auditforge;

import io.tenantguard.internal.Json;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/**
 * The POST /v1/events response (ecosystem/auditforge/schemas/IngestAck.schema.json). It
 * partitions the submitted batch into accepted, duplicate, and rejected events.
 */
public record IngestAck(int received, List<String> accepted, List<String> duplicate,
                        List<Rejection> rejected) {

    /** One rejected event: its id and why it was not stored. */
    public record Rejection(String eventId, String reason) {
    }

    /** Parses an ack from AuditForge's JSON response. */
    public static IngestAck fromJson(String json) {
        if (!(Json.parse(json) instanceof Map<?, ?> obj)) {
            throw new AuditForgeException("malformed ack: not a JSON object");
        }
        Object received = obj.get("received");
        return new IngestAck(
                received instanceof Number n ? n.intValue() : 0,
                strings(obj.get("accepted")),
                strings(obj.get("duplicate")),
                rejections(obj.get("rejected")));
    }

    private static List<String> strings(Object node) {
        List<String> out = new ArrayList<>();
        if (node instanceof List<?> list) {
            for (Object item : list) {
                out.add(String.valueOf(item));
            }
        }
        return out;
    }

    private static List<Rejection> rejections(Object node) {
        List<Rejection> out = new ArrayList<>();
        if (node instanceof List<?> list) {
            for (Object item : list) {
                if (item instanceof Map<?, ?> r) {
                    out.add(new Rejection(String.valueOf(r.get("eventId")), String.valueOf(r.get("reason"))));
                }
            }
        }
        return out;
    }
}
