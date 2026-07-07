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

// replayChain walks records in chain order and reports the first inconsistency. A record is
// consistent when its prevHash equals the running head and its stored hash equals
// chain.Next recomputed from its canonical form — so an edit, insertion, or deletion anywhere
// in the chain surfaces as the first record whose links no longer line up.
func replayChain(records []chainRecord) ChainReport {
	report := ChainReport{Length: len(records), Head: chain.Genesis, Intact: true}
	if n := len(records); n > 0 {
		report.Head = records[n-1].hash
	}

	prev := chain.Genesis
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
