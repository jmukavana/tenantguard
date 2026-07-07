package io.tenantguard;

/**
 * Thrown when a {@link TenantContext} cannot be constructed. The {@link #code()} is a
 * stable, language-neutral symbol (e.g. {@code "empty_tenant_id"}) matching the codes in
 * the shared compliance fixtures, so the conformance harness can assert on it without
 * depending on the message text.
 */
public final class TenantContextException extends RuntimeException {

    /** Tenant id was null or empty. */
    public static final String EMPTY_TENANT_ID = "empty_tenant_id";
    /** Trace id was present but not a valid W3C trace-id. */
    public static final String INVALID_TRACE_ID = "invalid_trace_id";

    private final String code;

    TenantContextException(String code, String message) {
        super(message);
        this.code = code;
    }

    /** The stable symbolic error code. */
    public String code() {
        return code;
    }
}
