// Package keyring resolves a keyId to the shared secret AuditForge uses to verify event
// signatures (ecosystem/auditforge/spec.md, "The signing contract"). keyId is untrusted
// input: an unknown key must fail verification, never default to some other secret.
package keyring

// Keyring maps a keyId to its signing secret.
type Keyring interface {
	// Secret returns the secret for keyID and whether the key is known.
	Secret(keyID string) ([]byte, bool)
}

// Map is an in-memory Keyring backed by a keyId->secret map.
type Map map[string][]byte

// Secret returns the secret registered for keyID, or ok=false when the key is unknown.
func (m Map) Secret(keyID string) ([]byte, bool) {
	secret, ok := m[keyID]
	return secret, ok
}
