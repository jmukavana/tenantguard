package io.tenantguard.audit.auditforge;

import io.tenantguard.audit.TenantEvent;
import io.tenantguard.internal.Json;

import java.util.List;

/**
 * The POST /v1/events body (ecosystem/auditforge/schemas/IngestRequest.schema.json): a
 * {@code keyId} plus a batch of signed events. Serialized by hand so the core carries no
 * JSON dependency; each event is emitted via its own {@link TenantEvent#toJson()}.
 */
public record IngestRequest(String keyId, List<TenantEvent> events) {

    /** Serializes this request to the canonical wire JSON. */
    public String toJson() {
        StringBuilder sb = new StringBuilder("{");
        sb.append(Json.quote("keyId")).append(':').append(Json.quote(keyId)).append(',');
        sb.append(Json.quote("events")).append(":[");
        for (int i = 0; i < events.size(); i++) {
            if (i > 0) {
                sb.append(',');
            }
            sb.append(events.get(i).toJson());
        }
        return sb.append("]}").toString();
    }
}
