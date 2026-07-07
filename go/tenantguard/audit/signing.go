package audit

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// CanonicalJSON returns the exact bytes that are signed for e: the JSON encoding of the
// event with its signature cleared, in the field order fixed by
// core/schemas/TenantEvent.schema.json. This is the cross-language canonical form
// (ecosystem/auditforge/spec.md, "The signing contract"); it must match, byte for byte,
// what the Java/Python/TypeScript TenantEvent serializers produce. HTML escaping is
// disabled so `<`, `>`, and `&` survive verbatim, as they do under JSON.stringify and the
// other languages' encoders.
func CanonicalJSON(e Event) ([]byte, error) {
	e.Signature = ""

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(e); err != nil {
		return nil, err
	}
	// Encode appends a trailing newline that is not part of the canonical form.
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// Sign returns the lowercase-hex HMAC-SHA256 of e's canonical form under secret
// (ecosystem/auditforge/spec.md, "The signing contract"). AuditForge recomputes this on
// ingest and compares it in constant time to detect tampering.
func Sign(e Event, secret []byte) (string, error) {
	canonical, err := CanonicalJSON(e)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(canonical)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// Verify reports whether signature is a valid HMAC-SHA256 over e's canonical form under
// secret. The comparison is constant-time to avoid leaking the expected signature through
// timing. signature is the lowercase-hex form produced by Sign.
func Verify(e Event, secret []byte, signature string) (bool, error) {
	expected, err := Sign(e, secret)
	if err != nil {
		return false, err
	}
	return hmac.Equal([]byte(expected), []byte(signature)), nil
}
