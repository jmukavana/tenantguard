package io.tenantguard.resolver;

import java.util.Locale;
import java.util.regex.Pattern;

/** Best-effort extraction of the W3C trace-id from a {@code traceparent} header. */
final class TraceParent {

    private static final Pattern TRACE_ID = Pattern.compile("^[0-9a-f]{32}$");

    private TraceParent() {
    }

    /**
     * Returns the trace-id from a {@code version-traceid-spanid-flags} traceparent, or ""
     * when the header is missing or malformed — so a bad trace header never fails an
     * otherwise valid request.
     */
    static String traceId(String traceparent) {
        if (traceparent == null) {
            return "";
        }
        String[] parts = traceparent.split("-");
        if (parts.length < 4) {
            return "";
        }
        String traceId = parts[1].toLowerCase(Locale.ROOT);
        return TRACE_ID.matcher(traceId).matches() ? traceId : "";
    }
}
