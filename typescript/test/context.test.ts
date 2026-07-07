/**
 * Security-critical guard: a TenantContext must not change after construction. Mirrors the
 * Go TestImmutable and its Java/Python equivalents.
 */
import { describe, expect, it } from "vitest";
import {
  EMPTY_TENANT_ID,
  INVALID_TRACE_ID,
  TenantContext,
  TenantContextError,
} from "../src/context/index.js";

describe("TenantContext immutability", () => {
  it("does not leak source-map mutations", () => {
    const src: Record<string, string> = { region: "ke", tier: "gold" };
    const tc = TenantContext.create("acme", "", "", src);

    src.region = "tz";
    src.injected = "evil";

    expect(tc.metadata.region).toBe("ke");
    expect("injected" in tc.metadata).toBe(false);
  });

  it("returned metadata is frozen", () => {
    const tc = TenantContext.create("acme", "", "", { a: "b" });
    expect(() => {
      (tc.metadata as Record<string, string>).x = "y";
    }).toThrow(TypeError);
  });

  it("the instance is frozen", () => {
    const tc = TenantContext.create("acme");
    expect(() => {
      (tc as unknown as { tenantId: string }).tenantId = "beta";
    }).toThrow(TypeError);
  });
});

describe("TenantContext validation", () => {
  it("rejects an empty tenant id", () => {
    try {
      TenantContext.create("");
      throw new Error("expected throw");
    } catch (e) {
      expect(e).toBeInstanceOf(TenantContextError);
      expect((e as TenantContextError).code).toBe(EMPTY_TENANT_ID);
    }
  });

  it("rejects a malformed trace id", () => {
    try {
      TenantContext.create("acme", "", "not-a-trace");
      throw new Error("expected throw");
    } catch (e) {
      expect((e as TenantContextError).code).toBe(INVALID_TRACE_ID);
    }
  });

  it("exposes fields on a valid context", () => {
    const tc = TenantContext.create("acme", "Acme", "4bf92f3577b34da6a3ce929d0e0e4736");
    expect(tc.isValid()).toBe(true);
    expect(tc.tenantId).toBe("acme");
    expect(tc.traceId).toBe("4bf92f3577b34da6a3ce929d0e0e4736");
    expect({ ...tc.metadata }).toEqual({});
  });
});
