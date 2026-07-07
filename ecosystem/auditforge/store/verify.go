package store

import (
	"github.com/tenantguard/tenantguard/audit"

	"github.com/tenantguard/auditforge/chain"
)

// chainRecord is one stored event plus its position in the tenant's hash chain. Both the
// in-memory and Postgres stores materialize their records into this shape to replay.
type chainRecord struct {
	event    audit.Event
	prevHash string
	hash     string
}

// replayFrom walks records in chain order starting from startHash and reports the first
// inconsistency. A record is consistent when its prevHash equals the running head and its
// stored hash equals chain.Next recomputed from its canonical form — so an edit, insertion,
// or deletion anywhere in the surviving tail surfaces as the first record whose links no
// longer line up. purged is the number of records already removed behind startHash (0 and
// chain.Genesis for a chain that has never been purged).
func replayFrom(startHash string, purged int, records []chainRecord) ChainReport {
	report := ChainReport{Length: purged + len(records), Head: startHash, Intact: true, Purged: purged}
	if n := len(records); n > 0 {
		report.Head = records[n-1].hash
	}

	prev := startHash
	for _, r := range records {
		canonical, err := audit.CanonicalJSON(r.event)
		if err != nil || r.prevHash != prev || chain.Next(r.prevHash, canonical) != r.hash {
			report.Intact = false
			report.BrokenAt = r.event.EventID
			return report
		}
		prev = r.hash
	}
	return report
}

// checkpointStart returns the hash a surviving tail must link from: the checkpoint hash, or
// chain.Genesis when no checkpoint exists (Hash == "").
func checkpointStart(cp chain.Checkpoint) string {
	if cp.Hash == "" {
		return chain.Genesis
	}
	return cp.Hash
}

// brokenCheckpoint reports a chain whose retention checkpoint failed to verify: the purged
// prefix can no longer be trusted, so the whole chain is compromised.
func brokenCheckpoint(cp chain.Checkpoint, records []chainRecord) ChainReport {
	head := cp.Hash
	if n := len(records); n > 0 {
		head = records[n-1].hash
	}
	return ChainReport{
		Length:   cp.Length + len(records),
		Head:     head,
		Intact:   false,
		Purged:   cp.Length,
		BrokenAt: "checkpoint",
	}
}
