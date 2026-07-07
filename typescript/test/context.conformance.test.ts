/**
 * Runs the shared tenant-context.json fixture against the TypeScript implementation.
 * TypeScript mirror of the Go/Java/Python conformance harnesses; all must pass the same
 * vectors.
 */
import { describe, expect, it } from "vitest";
import { cases } from "./fixtures.js";
import { TenantContext, TenantContextError } from "../src/context/index.js";

describe("tenant-context conformance", () => {
  for (const c of cases("tenant-context.json")) {
    it(c.name, () => {
      const input = c.input;
      const expected = c.expect;
      const build = () =>
        TenantContext.create(
          input.tenantId ?? "",
          input.tenantName ?? "",
          input.traceId ?? "",
          input.metadata ?? {},
        );

      if (expected.error !== undefined) {
        expect(build).toThrow(TenantContextError);
        try {
          build();
        } catch (e) {
          expect((e as TenantContextError).code).toBe(expected.error);
        }
        return;
      }

      const tc = build();
      expect(tc.isValid()).toBe(expected.valid);
      expect(tc.tenantId).toBe(expected.tenantId ?? "");
      expect(tc.tenantName).toBe(expected.tenantName ?? "");
      expect(tc.traceId).toBe(expected.traceId ?? "");
      expect({ ...tc.metadata }).toEqual(expected.metadata ?? {});
    });
  }
});
