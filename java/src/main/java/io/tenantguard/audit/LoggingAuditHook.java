package io.tenantguard.audit;

/**
 * Development {@link AuditHook} that writes events via the JDK {@link System.Logger} (no
 * logging dependency). Replace with the AuditForge client in production.
 */
public final class LoggingAuditHook implements AuditHook {

    private static final System.Logger LOG = System.getLogger("io.tenantguard.audit");

    @Override
    public void publish(TenantEvent event) {
        LOG.log(System.Logger.Level.INFO, () -> "tenant.audit " + event.toJson());
    }
}
