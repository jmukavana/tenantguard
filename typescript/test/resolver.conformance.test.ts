/** Runs the shared resolver fixtures against the TypeScript resolvers. */
import { describe, expect, it } from "vitest";
import { cases } from "./fixtures.js";
import { FakeRequest } from "./helpers.js";
import { HeaderResolver, SubdomainResolver } from "../src/resolver/index.js";

describe("resolver-header conformance", () => {
  for (const c of cases("resolver-header.json")) {
    it(c.name, () => {
      const request = new FakeRequest({ headers: c.headers ?? {} });
      const result = new HeaderResolver(c.header ?? "").resolve(request);
      const expected = c.expect;

      if (expected.error !== undefined) {
        expect(expected.error).toBe("no_tenant");
        expect(result).toBeNull();
        return;
      }
      expect(result).not.toBeNull();
      expect(result!.tenantId).toBe(expected.tenantId ?? "");
      expect(result!.traceId).toBe(expected.traceId ?? "");
    });
  }
});

describe("resolver-subdomain conformance", () => {
  for (const c of cases("resolver-subdomain.json")) {
    it(c.name, () => {
      const request = new FakeRequest({ host: c.host });
      const result = new SubdomainResolver(c.baseDomain, c.ignore).resolve(request);
      const expected = c.expect;

      if (expected.error !== undefined) {
        expect(expected.error).toBe("no_tenant");
        expect(result).toBeNull();
        return;
      }
      expect(result).not.toBeNull();
      expect(result!.tenantId).toBe(expected.tenantId ?? "");
    });
  }
});
