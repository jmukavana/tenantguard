/** Unit tests for the TenantEvent audit record. */
import { describe, expect, it } from "vitest";
import { TenantEvent } from "../src/audit/index.js";

describe("TenantEvent", () => {
  it("omits empty optionals and formats the timestamp with Z", () => {
    const event = new TenantEvent({
      eventId: "e1",
      eventType: "tenant.resolved",
      tenantId: "acme",
      timestamp: new Date("2026-01-01T00:00:00Z"),
      outcome: "allowed",
    });
    const out = JSON.parse(event.toJson());
    expect(Object.keys(out).sort()).toEqual(
      ["eventId", "eventType", "outcome", "tenantId", "timestamp"].sort(),
    );
    expect(out.timestamp).toBe("2026-01-01T00:00:00Z");
  });

  it("sorts metadata keys", () => {
    const event = new TenantEvent({
      eventId: "e2",
      eventType: "x",
      tenantId: "acme",
      timestamp: new Date("2026-01-01T00:00:00Z"),
      outcome: "denied",
      metadata: { z: "1", a: "2" },
    });
    expect(event.toJson()).toContain('"metadata":{"a":"2","z":"1"}');
  });

  it("create generates a fresh 32-char id", () => {
    const a = TenantEvent.create("x", "acme", "", "allowed");
    const b = TenantEvent.create("x", "acme", "", "allowed");
    expect(a.eventId).toHaveLength(32);
    expect(a.eventId).not.toBe(b.eventId);
  });
});
