package io.tenantguard.audit;

/**
 * Publishes tenant audit events to a sink (AuditForge in production; see core/spec.md,
 * Observability &amp; Audit).
 */
public interface AuditHook {

    void publish(TenantEvent event);
}
