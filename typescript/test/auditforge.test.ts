/** Exercises AuditForgeHook with an injected fetch standing in for AuditForge. */
import { describe, expect, it } from "vitest";
import { AuditForgeError, AuditForgeHook } from "../src/audit/auditforge.js";
import { TenantEvent, verify, type Outcome } from "../src/audit/index.js";

const SECRET = "tenantguard-shared-secret";

interface Reply {
  status: number;
  body: string;
}

/** A fake fetch that records requests and replies per attempt. */
function fakeFetch(reply: (attempt: number, request: any) => Reply) {
  const state = { calls: 0, url: "", method: "", contentType: "", request: null as any };
  const fn = (async (url: string | URL, init: any): Promise<Response> => {
    state.calls += 1;
    state.url = String(url);
    state.method = init.method;
    state.contentType = init.headers["Content-Type"];
    state.request = JSON.parse(init.body);
    const { status, body } = reply(state.calls, state.request);
    return new Response(body, { status, headers: { "Content-Type": "application/json" } });
  }) as unknown as typeof fetch;
  return { fn, state };
}

function hook(fetchFn: typeof fetch, maxRetries: number): AuditForgeHook {
  return new AuditForgeHook({
    endpoint: "http://auditforge.test",
    keyId: "k1",
    secret: SECRET,
    maxRetries,
    backoffMs: () => 0,
    fetch: fetchFn,
  });
}

function eventFrom(wire: any): TenantEvent {
  return new TenantEvent({
    eventId: wire.eventId,
    eventType: wire.eventType,
    tenantId: wire.tenantId,
    timestamp: new Date(wire.timestamp),
    outcome: wire.outcome as Outcome,
    traceId: wire.traceId,
    actor: wire.actor,
    resource: wire.resource,
    metadata: wire.metadata,
    signature: wire.signature,
  });
}

function ack(partitions: Partial<{ accepted: string[]; duplicate: string[]; rejected: any[] }>): Reply {
  return {
    status: 200,
    body: JSON.stringify({
      received: 1,
      accepted: partitions.accepted ?? [],
      duplicate: partitions.duplicate ?? [],
      rejected: partitions.rejected ?? [],
    }),
  };
}

describe("AuditForgeHook", () => {
  it("signs the event and reports the accepted event as delivered", async () => {
    const { fn, state } = fakeFetch((_attempt, request) =>
      ack({ accepted: [request.events[0].eventId] }),
    );

    const event = TenantEvent.create("tenant.resolved", "acme", "", "allowed");
    await hook(fn, 2).publish(event);

    expect(state.calls).toBe(1);
    expect(state.method).toBe("POST");
    expect(state.url).toBe("http://auditforge.test/v1/events");
    expect(state.contentType).toBe("application/json");
    expect(state.request.keyId).toBe("k1");
    const sent = state.request.events[0];
    expect(sent.signature).toBeTruthy();
    // The server verifies exactly as AuditForge would.
    expect(verify(eventFrom(sent), SECRET, sent.signature)).toBe(true);
  });

  it("treats a duplicate as an idempotent success", async () => {
    const { fn } = fakeFetch((_attempt, request) => ack({ duplicate: [request.events[0].eventId] }));
    await expect(
      hook(fn, 0).publish(TenantEvent.create("x", "acme", "", "allowed")),
    ).resolves.toBeUndefined();
  });

  it("errors when the event is rejected", async () => {
    const { fn } = fakeFetch((_attempt, request) =>
      ack({ rejected: [{ eventId: request.events[0].eventId, reason: "invalid_signature" }] }),
    );
    await expect(hook(fn, 0).publish(TenantEvent.create("x", "acme", "", "allowed"))).rejects.toThrow(
      /invalid_signature/,
    );
  });

  it("errors when the event is unacknowledged", async () => {
    const { fn } = fakeFetch(() => ack({}));
    await expect(hook(fn, 0).publish(TenantEvent.create("x", "acme", "", "allowed"))).rejects.toThrow(
      /missing from ack/,
    );
  });

  it("retries a 5xx then succeeds", async () => {
    const { fn, state } = fakeFetch((attempt, request) =>
      attempt === 1
        ? { status: 503, body: "unavailable" }
        : ack({ accepted: [request.events[0].eventId] }),
    );
    await hook(fn, 2).publish(TenantEvent.create("x", "acme", "", "allowed"));
    expect(state.calls).toBe(2);
  });

  it("does not retry a 4xx", async () => {
    const { fn, state } = fakeFetch(() => ({ status: 401, body: "unknown key" }));
    await expect(hook(fn, 3).publish(TenantEvent.create("x", "acme", "", "allowed"))).rejects.toThrow(
      AuditForgeError,
    );
    expect(state.calls).toBe(1);
  });

  it("gives up after exhausting retries", async () => {
    const { fn, state } = fakeFetch(() => ({ status: 502, body: "down" }));
    await expect(hook(fn, 2).publish(TenantEvent.create("x", "acme", "", "allowed"))).rejects.toThrow(
      /giving up after 3 attempts/,
    );
    expect(state.calls).toBe(3);
  });
});
