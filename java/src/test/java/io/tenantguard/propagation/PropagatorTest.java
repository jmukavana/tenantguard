package io.tenantguard.propagation;

import io.opentelemetry.api.baggage.Baggage;
import io.opentelemetry.context.Context;
import io.tenantguard.TenantContext;
import org.junit.jupiter.api.Test;

import java.util.HashMap;
import java.util.Map;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

class PropagatorTest {

    private final Propagator propagator = new Propagator();

    @Test
    void roundTripWithMetadata() {
        TenantContext tc = TenantContext.of("acme", "Acme Ltd", "4bf92f3577b34da6a3ce929d0e0e4736",
                Map.of("region", "ke", "tier", "gold"));

        Map<String, String> carrier = new HashMap<>();
        propagator.inject(tc, carrier);
        assertTrue(carrier.containsKey("baggage"), "expected a baggage header");

        TenantContext got = propagator.extract(carrier).orElseThrow();
        assertEquals("acme", got.tenantId());
        assertEquals("Acme Ltd", got.tenantName());
        assertEquals("4bf92f3577b34da6a3ce929d0e0e4736", got.traceId());
        assertEquals(Map.of("region", "ke", "tier", "gold"), got.metadata());
    }

    @Test
    void extractEmptyCarrierYieldsNoTenant() {
        assertTrue(propagator.extract(new HashMap<>()).isEmpty());
    }

    @Test
    void injectInvalidContextThrows() {
        assertThrows(IllegalArgumentException.class, () -> propagator.inject(null, new HashMap<>()));
    }

    @Test
    void injectPreservesExistingBaggage() {
        Context context = Context.root().with(Baggage.builder().put("app.feature", "on").build());
        Map<String, String> carrier = new HashMap<>();

        propagator.inject(context, TenantContext.of("acme", "", "", null), carrier);

        Optional<TenantContext> got = propagator.extract(carrier);
        assertTrue(got.isPresent());
        assertEquals("acme", got.get().tenantId());

        // The pre-existing, non-tenant member must survive.
        Baggage out = Baggage.fromContext(
                io.opentelemetry.api.baggage.propagation.W3CBaggagePropagator.getInstance()
                        .extract(Context.root(), carrier, new MapGetter()));
        assertEquals("on", out.getEntryValue("app.feature"));
        assertFalse(out.getEntryValue("tg.tenant.id") == null);
    }

    /** Minimal getter so the test can re-parse the injected baggage header. */
    static final class MapGetter implements io.opentelemetry.context.propagation.TextMapGetter<Map<String, String>> {
        @Override
        public Iterable<String> keys(Map<String, String> carrier) {
            return carrier.keySet();
        }

        @Override
        public String get(Map<String, String> carrier, String key) {
            return carrier.get(key);
        }
    }
}
