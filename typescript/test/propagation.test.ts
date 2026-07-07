/** Unit tests for the W3C Baggage propagator. */
import { describe, expect, it } from "vitest";
import { defaultTextMapGetter, propagation, ROOT_CONTEXT } from "@opentelemetry/api";
import { W3CBaggagePropagator } from "@opentelemetry/core";
import { TenantContext } from "../src/context/index.js";
import { Propagator } from "../src/propagation/index.js";

const w3c = new W3CBaggagePropagator();

describe("Propagator", () => {
  it("round-trips with metadata", () => {
    const tc = TenantContext.create("acme", "Acme Ltd", "4bf92f3577b34da6a3ce929d0e0e4736", {
      region: "ke",
      tier: "gold",
    });
    const carrier: Record<string, string> = {};
    new Propagator().inject(tc, carrier);
    expect(carrier.baggage).toBeDefined();

    const got = new Propagator().extract(carrier);
    expect(got).not.toBeNull();
    expect(got!.tenantId).toBe("acme");
    expect(got!.tenantName).toBe("Acme Ltd");
    expect(got!.traceId).toBe("4bf92f3577b34da6a3ce929d0e0e4736");
    expect({ ...got!.metadata }).toEqual({ region: "ke", tier: "gold" });
  });

  it("returns null when the carrier holds no tenant", () => {
    expect(new Propagator().extract({})).toBeNull();
  });

  it("throws on an invalid context", () => {
    expect(() => new Propagator().inject(null as unknown as TenantContext, {})).toThrow();
  });

  it("preserves pre-existing baggage", () => {
    const context = propagation.setBaggage(
      ROOT_CONTEXT,
      propagation.createBaggage({ "app.feature": { value: "on" } }),
    );
    const carrier: Record<string, string> = {};
    new Propagator().inject(TenantContext.create("acme"), carrier, context);

    const baggage = propagation.getBaggage(w3c.extract(ROOT_CONTEXT, carrier, defaultTextMapGetter));
    expect(baggage!.getEntry("app.feature")?.value).toBe("on");
    expect(baggage!.getEntry("tg.tenant.id")?.value).toBe("acme");
  });
});
