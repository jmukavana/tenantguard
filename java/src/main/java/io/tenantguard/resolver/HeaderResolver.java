package io.tenantguard.resolver;

import io.tenantguard.TenantContext;

import java.util.Optional;

/**
 * Resolves the tenant from an HTTP header (default {@code X-Tenant-ID}). When present, the
 * W3C {@code traceparent} header seeds the context's trace id so audit events correlate.
 */
public final class HeaderResolver implements TenantResolver {

    /** The conventional tenant header (core/spec.md, TenantResolver). */
    public static final String DEFAULT_HEADER = "X-Tenant-ID";

    private final String header;

    /** Uses {@link #DEFAULT_HEADER}. */
    public HeaderResolver() {
        this(DEFAULT_HEADER);
    }

    /** Uses {@code header}; blank falls back to {@link #DEFAULT_HEADER}. */
    public HeaderResolver(String header) {
        this.header = (header == null || header.isBlank()) ? DEFAULT_HEADER : header;
    }

    @Override
    public Optional<TenantContext> resolve(HttpRequestView request) {
        String raw = request.header(header);
        String id = raw == null ? "" : raw.trim();
        if (id.isEmpty()) {
            return Optional.empty();
        }
        return Optional.of(TenantContext.of(id, "", TraceParent.traceId(request.header("traceparent")), null));
    }
}
