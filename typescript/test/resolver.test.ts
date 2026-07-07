/**
 * Idiomatic-usage tests for the resolvers. Behavioral coverage lives in the fixture
 * conformance tests; these document the API.
 */
import { describe, expect, it } from "vitest";
import { FakeRequest } from "./helpers.js";
import { HeaderResolver, SubdomainResolver } from "../src/resolver/index.js";

describe("HeaderResolver", () => {
  it("reads a custom header and the traceparent", () => {
    const request = new FakeRequest({
      headers: {
        "X-Org": "beta",
        traceparent: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
      },
    });
    const tc = new HeaderResolver("X-Org").resolve(request);
    expect(tc).not.toBeNull();
    expect(tc!.tenantId).toBe("beta");
    expect(tc!.traceId).toBe("4bf92f3577b34da6a3ce929d0e0e4736");
  });

  it("returns null when the header is missing", () => {
    expect(new HeaderResolver().resolve(new FakeRequest())).toBeNull();
  });
});

describe("SubdomainResolver", () => {
  it("honours a custom ignore list", () => {
    const resolver = new SubdomainResolver("example.com", ["admin", "www"]);
    expect(resolver.resolve(new FakeRequest({ host: "admin.example.com" }))).toBeNull();
    expect(
      resolver.resolve(new FakeRequest({ host: "acme.example.com" }))!.tenantId,
    ).toBe("acme");
  });
});
