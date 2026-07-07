package io.tenantguard.isolation;

/**
 * Thrown when an isolation strategy refuses to act. {@link #code()} is a stable,
 * language-neutral symbol matching the compliance fixtures.
 */
public final class IsolationException extends RuntimeException {

    /** Tenant id cannot be used as a safe schema identifier. */
    public static final String UNSAFE_TENANT_ID = "unsafe_tenant_id";

    private final String code;

    IsolationException(String code, String message) {
        super(message);
        this.code = code;
    }

    /** The stable symbolic error code. */
    public String code() {
        return code;
    }
}
