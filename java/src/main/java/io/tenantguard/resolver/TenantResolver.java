package io.tenantguard.resolver;

import io.tenantguard.TenantContext;

import java.util.Optional;

/**
 * Turns an inbound request into a {@link TenantContext} (core/spec.md, TenantResolver).
 *
 * <p>Returns {@link Optional#empty()} when the request carries no tenant identifier — the
 * idiomatic Java form of the fixtures' {@code no_tenant} outcome. The caller (middleware)
 * decides how to reject an unidentified request.
 */
public interface TenantResolver {

    Optional<TenantContext> resolve(HttpRequestView request);
}
