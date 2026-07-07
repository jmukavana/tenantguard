package io.tenantguard.audit;

/** The result of an audited operation, as it appears on the wire. */
public enum Outcome {
    ALLOWED("allowed"),
    DENIED("denied"),
    ERROR("error");

    private final String value;

    Outcome(String value) {
        this.value = value;
    }

    /** The lowercase wire value (matches core/schemas/TenantEvent.schema.json). */
    public String value() {
        return value;
    }

    /** Parses a wire value back to an Outcome. */
    public static Outcome fromValue(String value) {
        for (Outcome o : values()) {
            if (o.value.equals(value)) {
                return o;
            }
        }
        throw new IllegalArgumentException("unknown outcome: " + value);
    }
}
