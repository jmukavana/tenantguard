package chain

import (
	"testing"
	"time"
)

func TestCheckpointSignRoundTrips(t *testing.T) {
	secret := []byte("auditforge-checkpoint-key")
	cp := Checkpoint{Length: 2, Hash: goldenHead, Through: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	cp.Signature = SignCheckpoint(cp, secret)

	if !VerifyCheckpoint(cp, secret) {
		t.Fatal("a freshly signed checkpoint must verify")
	}
	if VerifyCheckpoint(cp, []byte("wrong-key")) {
		t.Error("checkpoint verified under the wrong key")
	}
}

func TestCheckpointVerifyDetectsTampering(t *testing.T) {
	secret := []byte("auditforge-checkpoint-key")
	cp := Checkpoint{Length: 2, Hash: goldenHead, Through: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	cp.Signature = SignCheckpoint(cp, secret)

	// Forging a shorter chain (hiding purged records) must break the signature.
	forged := cp
	forged.Length = 1
	if VerifyCheckpoint(forged, secret) {
		t.Error("editing length must invalidate the checkpoint")
	}

	forged = cp
	forged.Hash = goldenSecond
	if VerifyCheckpoint(forged, secret) {
		t.Error("editing hash must invalidate the checkpoint")
	}
}

func TestCheckpointCanonicalNormalizesToUTC(t *testing.T) {
	east := time.FixedZone("EAT", 3*60*60)
	utc := Checkpoint{Length: 1, Hash: "h", Through: time.Date(2026, 1, 1, 3, 0, 0, 0, east)}
	same := Checkpoint{Length: 1, Hash: "h", Through: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	if string(utc.Canonical()) != string(same.Canonical()) {
		t.Fatalf("canonical must normalize zone:\n %s\n %s", utc.Canonical(), same.Canonical())
	}
}
