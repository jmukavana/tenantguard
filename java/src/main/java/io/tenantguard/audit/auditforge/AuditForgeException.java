package io.tenantguard.audit.auditforge;

/**
 * Thrown when an event cannot be delivered to AuditForge: it was rejected, came back
 * unacknowledged, or could not be sent within the retry budget. Unchecked so it fits the
 * {@code void} {@link io.tenantguard.audit.AuditHook#publish} contract.
 */
public class AuditForgeException extends RuntimeException {

    public AuditForgeException(String message) {
        super(message);
    }

    public AuditForgeException(String message, Throwable cause) {
        super(message, cause);
    }
}
