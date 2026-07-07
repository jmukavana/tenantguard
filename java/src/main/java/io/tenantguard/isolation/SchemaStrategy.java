package io.tenantguard.isolation;

import io.tenantguard.TenantContext;

import java.sql.Connection;
import java.sql.SQLException;
import java.sql.Statement;
import java.util.regex.Pattern;

/**
 * Schema-based isolation — the recommended default for regulated systems. Each tenant's
 * data lives in its own Postgres schema, and queries run with {@code search_path} scoped
 * to that schema.
 *
 * <p>Because a schema name is interpolated into a {@code SET} statement (which does not
 * accept bind parameters), tenant ids are restricted to a strict allowlist so SQL
 * injection is impossible.
 */
public final class SchemaStrategy {

    private static final Pattern SAFE_IDENT = Pattern.compile("^[A-Za-z0-9_]+$");

    private final String prefix;

    /** @param prefix prepended to the tenant id to form the schema, e.g. {@code "tenant_"} */
    public SchemaStrategy(String prefix) {
        this.prefix = prefix == null ? "" : prefix;
    }

    /**
     * Returns the validated, injection-safe schema name for {@code tc}.
     *
     * @throws IsolationException if the tenant id contains anything outside [A-Za-z0-9_]
     */
    public String schemaName(TenantContext tc) {
        String id = tc.tenantId();
        if (id == null || !SAFE_IDENT.matcher(id).matches()) {
            throw new IsolationException(IsolationException.UNSAFE_TENANT_ID,
                    "tenant id is not a safe schema identifier: " + id);
        }
        return prefix + id;
    }

    /** The SET statement that scopes {@code search_path} to {@code tc}'s schema. */
    String searchPathSql(TenantContext tc) {
        // schemaName matched SAFE_IDENT, so double-quoting it can carry no injection.
        return "SET search_path TO \"" + schemaName(tc) + "\"";
    }

    /**
     * Scopes subsequent statements on {@code connection} to the tenant's schema. Run it at
     * the start of the tenant-scoped transaction/connection; afterward an unqualified
     * {@code SELECT ... FROM widgets} resolves only within the tenant's schema.
     */
    public void bind(Connection connection, TenantContext tc) throws SQLException {
        try (Statement st = connection.createStatement()) {
            st.execute(searchPathSql(tc));
        }
    }
}
