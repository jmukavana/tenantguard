package io.tenantguard;

import java.util.Map;
import java.util.regex.Pattern;

/**
 * Immutable identity of the tenant an operation runs for (core/spec.md, ADR-004).
 *
 * <p>Implemented as a record, so it is immutable, value-comparable, and cannot be
 * mutated after construction &mdash; the property that prevents tenant hopping. The
 * canonical constructor validates its inputs and defensively copies the metadata map into
 * an unmodifiable one, so neither the caller's later mutation of the source map nor a
 * caller mutating the map returned by {@link #metadata()} can alter the context.
 *
 * <p>Prefer the {@link #of} factory; it reads the same as the other language bindings.
 *
 * @param tenantId   stable tenant identifier &mdash; the isolation boundary key (required)
 * @param tenantName human-readable name; informational only, never an isolation key
 * @param traceId    W3C trace-id (32 lowercase hex chars) or empty
 * @param metadata   opaque tenant attributes; not trusted for isolation decisions
 */
public record TenantContext(String tenantId, String tenantName, String traceId,
                            Map<String, String> metadata) {

    private static final Pattern TRACE_ID = Pattern.compile("^[0-9a-f]{32}$");

    /** Validating, normalizing canonical constructor. */
    public TenantContext {
        if (tenantId == null || tenantId.isEmpty()) {
            throw new TenantContextException(TenantContextException.EMPTY_TENANT_ID,
                    "tenant id must not be empty");
        }
        traceId = traceId == null ? "" : traceId;
        if (!traceId.isEmpty() && !TRACE_ID.matcher(traceId).matches()) {
            throw new TenantContextException(TenantContextException.INVALID_TRACE_ID,
                    "trace id must be 32 lowercase hex characters");
        }
        tenantName = tenantName == null ? "" : tenantName;
        metadata = metadata == null ? Map.of() : Map.copyOf(metadata);
    }

    /**
     * Constructs an immutable TenantContext.
     *
     * @throws TenantContextException if {@code tenantId} is empty or {@code traceId} is
     *                                present but malformed
     */
    public static TenantContext of(String tenantId, String tenantName, String traceId,
                                   Map<String, String> metadata) {
        return new TenantContext(tenantId, tenantName, traceId, metadata);
    }

    /** Whether this context carries a tenant id. Always true for a constructed instance. */
    public boolean isValid() {
        return tenantId != null && !tenantId.isEmpty();
    }
}
