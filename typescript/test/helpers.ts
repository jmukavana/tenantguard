/** Shared test doubles. */
import type { HttpRequestView } from "../src/resolver/index.js";

/** Test double for HttpRequestView with case-insensitive header lookup. */
export class FakeRequest implements HttpRequestView {
  private readonly headers: Record<string, string> = {};
  private readonly hostValue?: string;

  constructor(opts: { headers?: Record<string, string>; host?: string } = {}) {
    for (const [k, v] of Object.entries(opts.headers ?? {})) {
      this.headers[k.toLowerCase()] = v;
    }
    this.hostValue = opts.host;
  }

  header(name: string): string | undefined {
    return this.headers[name.toLowerCase()];
  }

  host(): string | undefined {
    return this.hostValue;
  }
}
