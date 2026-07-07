/** Runs the shared isolation-schema.json fixture against the TypeScript SchemaStrategy. */
import { describe, expect, it } from "vitest";
import { cases } from "./fixtures.js";
import { TenantContext } from "../src/context/index.js";
import { IsolationError, SchemaStrategy } from "../src/isolation/index.js";

describe("isolation-schema conformance", () => {
  for (const c of cases("isolation-schema.json")) {
    it(c.name, () => {
      const tc = TenantContext.create(c.tenantId);
      const strategy = new SchemaStrategy(c.prefix ?? "");
      const expected = c.expect;

      if (expected.error !== undefined) {
        expect(() => strategy.schemaName(tc)).toThrow(IsolationError);
        try {
          strategy.schemaName(tc);
        } catch (e) {
          expect((e as IsolationError).code).toBe(expected.error);
        }
        return;
      }
      expect(strategy.schemaName(tc)).toBe(expected.schema);
    });
  }
});
