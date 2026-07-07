package io.tenantguard.resolver;

import java.util.HashMap;
import java.util.Locale;
import java.util.Map;

/** Test double for {@link HttpRequestView} with case-insensitive header lookup. */
public final class FakeRequest implements HttpRequestView {

    private final Map<String, String> headers = new HashMap<>();
    private String host;

    public FakeRequest header(String name, String value) {
        headers.put(name.toLowerCase(Locale.ROOT), value);
        return this;
    }

    public FakeRequest host(String host) {
        this.host = host;
        return this;
    }

    @Override
    public String header(String name) {
        return headers.get(name.toLowerCase(Locale.ROOT));
    }

    @Override
    public String host() {
        return host;
    }
}
