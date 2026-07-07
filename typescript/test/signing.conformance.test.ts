/** Runs the shared auditforge-signing.json golden vectors against the signing helpers. */
import { describe, expect, it } from "vitest";
import { load } from "./fixtures.js";
import { TenantEvent, canonicalJson, sign, verify, type Outcome } from "../src/audit/index.js";

const fixture = load("auditforge-signing.json");
const secret: string = fixture.secret;

function buildEvent(ev: any): TenantEvent {
  return new TenantEvent({
    eventId: ev.eventId,
    eventType: ev.eventType,
    tenantId: ev.tenantId,
    timestamp: new Date(ev.timestamp),
    outcome: ev.outcome as Outcome,
    traceId: ev.traceId,
    actor: ev.actor,
    resource: ev.resource,
    metadata: ev.metadata,
  });
}

describe("auditforge-signing conformance", () => {
  for (const c of fixture.cases) {
    it(c.name, () => {
      const event = buildEvent(c.event);

      // Canonical bytes and the HMAC are checked separately so a mismatch localizes to
      // serialization vs. the signing primitive.
      expect(canonicalJson(event)).toBe(c.canonical);
      expect(sign(event, secret)).toBe(c.signature);

      expect(verify(event, secret, c.signature)).toBe(true);
      expect(verify(event, "wrong-secret", c.signature)).toBe(false);
    });
  }
});
