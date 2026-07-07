package io.tenantguard.audit;

import java.nio.charset.StandardCharsets;
import java.security.GeneralSecurityException;
import java.security.MessageDigest;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;

/**
 * AuditForge event signing (ecosystem/auditforge/spec.md, "The signing contract"). The
 * canonical form is {@link TenantEvent#toJson()} of the <em>unsigned</em> event; the
 * signature is lowercase-hex HMAC-SHA256(secret, utf8(canonical)). Every TenantGuard
 * language and the AuditForge verifier must reproduce these bytes identically — the shared
 * golden vectors live in testkit/compliance-tests/fixtures/auditforge-signing.json.
 */
public final class Signing {

    private Signing() {
    }

    /**
     * Returns the exact bytes that are signed for {@code event}: its canonical JSON with any
     * signature cleared, so a stale signature never feeds back into the signed form.
     */
    public static String canonicalJson(TenantEvent event) {
        return unsigned(event).toJson();
    }

    /** Returns the lowercase-hex HMAC-SHA256 of {@code event}'s canonical form under {@code secret}. */
    public static String sign(TenantEvent event, byte[] secret) {
        byte[] mac = hmacSha256(secret, canonicalJson(event).getBytes(StandardCharsets.UTF_8));
        return hex(mac);
    }

    /**
     * Reports whether {@code signature} is a valid HMAC-SHA256 over {@code event}'s canonical
     * form under {@code secret}. The comparison is constant-time.
     */
    public static boolean verify(TenantEvent event, byte[] secret, String signature) {
        byte[] expected = sign(event, secret).getBytes(StandardCharsets.US_ASCII);
        return MessageDigest.isEqual(expected, signature.getBytes(StandardCharsets.US_ASCII));
    }

    private static TenantEvent unsigned(TenantEvent e) {
        if (e.signature().isEmpty()) {
            return e;
        }
        return new TenantEvent(e.eventId(), e.eventType(), e.tenantId(), e.timestamp(), e.outcome(),
                e.traceId(), e.actor(), e.resource(), e.metadata(), "");
    }

    private static byte[] hmacSha256(byte[] secret, byte[] message) {
        try {
            Mac mac = Mac.getInstance("HmacSHA256");
            mac.init(new SecretKeySpec(secret, "HmacSHA256"));
            return mac.doFinal(message);
        } catch (GeneralSecurityException e) {
            throw new IllegalStateException("HmacSHA256 is required but unavailable", e);
        }
    }

    private static String hex(byte[] bytes) {
        StringBuilder sb = new StringBuilder(bytes.length * 2);
        for (byte b : bytes) {
            sb.append(Character.forDigit((b >> 4) & 0xF, 16)).append(Character.forDigit(b & 0xF, 16));
        }
        return sb.toString();
    }
}
