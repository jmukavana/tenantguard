// Package chain implements AuditForge's per-tenant, tamper-evident hash chain
// (ecosystem/auditforge/spec.md, "Storage model"). Each stored record links to its
// predecessor by hash, so any insertion, deletion, or edit is detectable by replaying the
// chain from the genesis record.
package chain

import (
	"crypto/sha256"
	"encoding/hex"
)

// Genesis is the prevHash of the first record in a tenant's chain (the empty string).
const Genesis = ""

// Next returns a record's chain hash: the lowercase-hex SHA-256 of the predecessor's hash
// concatenated with this record's canonical form. canonical is audit.CanonicalJSON of the
// unsigned event — the very bytes that were signed — so the chain covers the same content
// the signature attests to. prevHash is Genesis for the first record in the chain.
func Next(prevHash string, canonical []byte) string {
	h := sha256.New()
	h.Write([]byte(prevHash))
	h.Write(canonical)
	return hex.EncodeToString(h.Sum(nil))
}
