/** Runs the shared tenant-event.json fixture against TenantEvent.toJson(). */
import { describe, expect, it } from "vitest";
import { cases } from "./fixtures.js";
import { TenantEvent, type Outcome } from "../src/audit/index.js";

describe("tenant-event conformance", () => {
  for (const c of cases("tenant-event.json")) {
    it(c.name, () => {
      const ev = c.event;
      const event = new TenantEvent({
        eventId: ev.eventId,
        eventType: ev.eventType,
        tenantId: ev.tenantId,
        timestamp: new Date(ev.timestamp),
        outcome: ev.outcome as Outcome,
        traceId: ev.traceId,
        actor: ev.actor,
        resource: ev.resource,
        metadata: ev.metadata,
        signature: ev.signature,
      });

      const out = JSON.parse(event.toJson());
      const expected = c.expect;

      for (const key of expected.requiredKeys) {
        expect(key in out).toBe(true);
      }
      for (const key of expected.absentKeys) {
        expect(key in out).toBe(false);
      }
      for (const [key, value] of Object.entries(expected.values)) {
        expect(out[key]).toBe(value);
      }
    });
  }
});
