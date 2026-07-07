package io.tenantguard.resolver;

import io.tenantguard.TenantContext;

import java.util.List;
import java.util.Locale;
import java.util.Optional;

/**
 * Resolves the tenant from the left-most label of the request host, e.g.
 * {@code acme.example.com} &rarr; {@code acme} when the base domain is {@code example.com}.
 */
public final class SubdomainResolver implements TenantResolver {

    private final String baseDomain;
    private final List<String> ignore;

    /** Ignores {@code www} by default. */
    public SubdomainResolver(String baseDomain) {
        this(baseDomain, null);
    }

    /**
     * @param baseDomain the shared apex, e.g. {@code example.com}
     * @param ignore     subdomain labels that are not tenants; null/empty means {@code ["www"]}
     */
    public SubdomainResolver(String baseDomain, List<String> ignore) {
        this.baseDomain = baseDomain == null ? "" : baseDomain;
        this.ignore = (ignore == null || ignore.isEmpty()) ? List.of("www") : List.copyOf(ignore);
    }

    @Override
    public Optional<TenantContext> resolve(HttpRequestView request) {
        String host = request.host();
        if (host == null || baseDomain.isEmpty()) {
            return Optional.empty();
        }
        host = stripPort(host).toLowerCase(Locale.ROOT);
        if (host.endsWith(".")) {
            host = host.substring(0, host.length() - 1);
        }

        String suffix = "." + baseDomain.toLowerCase(Locale.ROOT);
        if (!host.endsWith(suffix)) {
            return Optional.empty();
        }

        String sub = host.substring(0, host.length() - suffix.length());
        int dot = sub.indexOf('.');
        String label = dot < 0 ? sub : sub.substring(0, dot);
        if (label.isEmpty() || ignored(label)) {
            return Optional.empty();
        }
        return Optional.of(TenantContext.of(label, "", TraceParent.traceId(request.header("traceparent")), null));
    }

    /** Removes a trailing {@code :port} for a plain host:port (leaves bare hosts untouched). */
    private static String stripPort(String host) {
        int first = host.indexOf(':');
        if (first >= 0 && first == host.lastIndexOf(':')) {
            return host.substring(0, first);
        }
        return host;
    }

    private boolean ignored(String label) {
        for (String i : ignore) {
            if (i.equalsIgnoreCase(label)) {
                return true;
            }
        }
        return false;
    }
}
