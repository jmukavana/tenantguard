package io.tenantguard.resolver;

/**
 * Minimal read-only view of an inbound request that resolvers need. Keeping resolvers
 * against this port (rather than jakarta.servlet or a specific framework) leaves the core
 * dependency-free; adapters wrap their framework's request into an HttpRequestView.
 */
public interface HttpRequestView {

    /** Returns the value of the named header, or {@code null} if absent. */
    String header(String name);

    /** Returns the request host, possibly including a port (e.g. {@code acme.example.com:8443}). */
    String host();
}
