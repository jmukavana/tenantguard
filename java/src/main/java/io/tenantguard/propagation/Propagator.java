package io.tenantguard.propagation;

import io.opentelemetry.api.baggage.Baggage;
import io.opentelemetry.api.baggage.BaggageBuilder;
import io.opentelemetry.api.baggage.propagation.W3CBaggagePropagator;
import io.opentelemetry.context.Context;
import io.opentelemetry.context.propagation.TextMapGetter;
import io.opentelemetry.context.propagation.TextMapPropagator;
import io.opentelemetry.context.propagation.TextMapSetter;
import io.tenantguard.TenantContext;
import io.tenantguard.internal.Json;

import java.util.Map;
import java.util.Optional;

/**
 * Carries an immutable {@link TenantContext} across service, queue, and thread boundaries
 * (core/spec.md, ContextPropagator). Tenant identity travels as W3C Baggage members under
 * the {@code tg.*} namespace, so any OpenTelemetry-instrumented hop preserves it. This is
 * the Java counterpart of the Go propagation package and shares its wire encoding.
 */
public final class Propagator {

    static final String MEMBER_TENANT_ID = "tg.tenant.id";
    static final String MEMBER_TENANT_NAME = "tg.tenant.name";
    static final String MEMBER_TRACE_ID = "tg.trace.id";
    static final String MEMBER_METADATA = "tg.tenant.meta";

    private static final TextMapPropagator W3C = W3CBaggagePropagator.getInstance();

    private static final TextMapSetter<Map<String, String>> MAP_SETTER =
            (carrier, key, value) -> {
                if (carrier != null) {
                    carrier.put(key, value);
                }
            };

    private static final TextMapGetter<Map<String, String>> MAP_GETTER = new TextMapGetter<>() {
        @Override
        public Iterable<String> keys(Map<String, String> carrier) {
            return carrier.keySet();
        }

        @Override
        public String get(Map<String, String> carrier, String key) {
            return carrier == null ? null : carrier.get(key);
        }
    };

    /**
     * Writes {@code tc} into {@code carrier} as W3C Baggage, preserving any baggage already
     * present in {@code context}.
     *
     * @throws IllegalArgumentException if {@code tc} is null or invalid
     */
    public <C> void inject(Context context, TenantContext tc, C carrier, TextMapSetter<C> setter) {
        if (tc == null || !tc.isValid()) {
            throw new IllegalArgumentException("cannot propagate an invalid tenant context");
        }
        BaggageBuilder builder = Baggage.fromContext(context).toBuilder();
        builder.put(MEMBER_TENANT_ID, tc.tenantId());
        if (!tc.tenantName().isEmpty()) {
            builder.put(MEMBER_TENANT_NAME, tc.tenantName());
        }
        if (!tc.traceId().isEmpty()) {
            builder.put(MEMBER_TRACE_ID, tc.traceId());
        }
        if (!tc.metadata().isEmpty()) {
            builder.put(MEMBER_METADATA, Json.objectOf(tc.metadata()));
        }
        W3C.inject(context.with(builder.build()), carrier, setter);
    }

    /** Reconstructs a TenantContext from {@code carrier}, or empty when it carries no tenant. */
    public <C> Optional<TenantContext> extract(C carrier, TextMapGetter<C> getter) {
        Baggage bag = Baggage.fromContext(W3C.extract(Context.root(), carrier, getter));
        String id = bag.getEntryValue(MEMBER_TENANT_ID);
        if (id == null || id.isEmpty()) {
            return Optional.empty();
        }
        Map<String, String> metadata = null;
        String meta = bag.getEntryValue(MEMBER_METADATA);
        if (meta != null && !meta.isEmpty()) {
            metadata = Json.parseObject(meta);
        }
        return Optional.of(TenantContext.of(id,
                bag.getEntryValue(MEMBER_TENANT_NAME),
                bag.getEntryValue(MEMBER_TRACE_ID),
                metadata));
    }

    // --- Map conveniences (also the framework-agnostic default entry points) ---

    /** Injects into a map carrier (e.g. MQ headers) starting from the root context. */
    public void inject(TenantContext tc, Map<String, String> carrier) {
        inject(Context.root(), tc, carrier, MAP_SETTER);
    }

    /** Injects into a map carrier, preserving baggage already in {@code context}. */
    public void inject(Context context, TenantContext tc, Map<String, String> carrier) {
        inject(context, tc, carrier, MAP_SETTER);
    }

    /** Extracts from a map carrier. */
    public Optional<TenantContext> extract(Map<String, String> carrier) {
        return extract(carrier, MAP_GETTER);
    }
}
