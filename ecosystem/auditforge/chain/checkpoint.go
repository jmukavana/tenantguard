package chain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// Checkpoint records where a tenant's chain was truncated by retention purging. Everything up
// to and including the record hashing to Hash — Length records in total — has been removed;
// the surviving tail's first record must link to Hash instead of Genesis. The Signature (HMAC
// with an AuditForge key a database attacker does not hold) is what makes the checkpoint
// itself tamper-evident: without it, an attacker could delete records and forge a checkpoint
// to hide the gap.
type Checkpoint struct {
	// Length is the cumulative number of records purged through this checkpoint.
	Length int `json:"length"`
	// Hash is the chain head at the cut — the prevHash the surviving tail links to.
	Hash string `json:"hash"`
	// Through is the retention cutoff: every purged record was at or older than this.
	Through time.Time `json:"through"`
	// Signature is the lowercase-hex HMAC-SHA256 over the canonical checkpoint.
	Signature string `json:"-"`
}

// Canonical is the exact byte string signed for c: its length, hash, and cutoff in fixed
// order, the cutoff normalized to UTC. Hash is hex and Through is RFC 3339, so neither needs
// JSON escaping.
func (c Checkpoint) Canonical() []byte {
	return []byte(fmt.Sprintf(`{"length":%d,"hash":%q,"through":%q}`,
		c.Length, c.Hash, c.Through.UTC().Format(time.RFC3339Nano)))
}

// SignCheckpoint returns the lowercase-hex HMAC-SHA256 of c's canonical form under secret.
func SignCheckpoint(c Checkpoint, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(c.Canonical())
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyCheckpoint reports whether c.Signature is a valid HMAC-SHA256 over c under secret.
func VerifyCheckpoint(c Checkpoint, secret []byte) bool {
	expected := SignCheckpoint(c, secret)
	return hmac.Equal([]byte(expected), []byte(c.Signature))
}
