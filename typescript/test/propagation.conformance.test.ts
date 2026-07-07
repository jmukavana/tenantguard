/** Runs the shared propagation-baggage.json fixture against the TypeScript Propagator. */
import { describe, expect, it } from "vitest";
import { defaultTextMapGetter, propagation, ROOT_CONTEXT } from "@opentelemetry/api";
import { W3CBaggagePropagator } from "@opentelemetry/core";
import { cases } from "./fixtures.js";
import { TenantContext } from "../src/context/index.js";
import { Propagator } from "../src/propagation/index.js";

const propagator = new Propagator();
const w3c = new W3CBaggagePropagator();

describe("propagation-baggage conformance", () => {
  for (const c of cases("propagation-baggage.json")) {
    it(c.name, () => {
      const node = c.context;
      const tc = TenantContext.create(
        node.tenantId,
        node.tenantName ?? "",
        node.traceId ?? "",
        node.metadata ?? {},
      );

      const carrier: Record<string, string> = {};
      propagator.inject(tc, carrier);

      const baggage = propagation.getBaggage(w3c.extract(ROOT_CONTEXT, carrier, defaultTextMapGetter));
      const members: Record<string, string> = {};
      for (const [key, entry] of baggage!.getAllEntries()) {
        members[key] = entry.value;
      }

      const expected: Record<string, string> = c.expectMembers;
      for (const [key, value] of Object.entries(expected)) {
        expect(members[key]).toBe(value);
      }
      for (const key of Object.keys(members)) {
        if (key.startsWith("tg.")) {
          expect(key in expected).toBe(true);
        }
      }

      const got = propagator.extract(carrier);
      expect(got).not.toBeNull();
      expect(got!.tenantId).toBe(tc.tenantId);
      expect(got!.tenantName).toBe(tc.tenantName);
      expect(got!.traceId).toBe(tc.traceId);
      expect({ ...got!.metadata }).toEqual({ ...tc.metadata });
    });
  }
});
