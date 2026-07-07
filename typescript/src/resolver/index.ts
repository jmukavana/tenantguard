/**
 * Tenant resolvers (core/spec.md, TenantResolver).
 *
 * A resolver turns an inbound request into a {@link TenantContext}, returning `null` when
 * the request carries no tenant identifier (the TypeScript form of the fixtures'
 * `no_tenant` outcome). Resolvers work against the small {@link HttpRequestView} interface
 * so the core needn't depend on any web framework; adapters wrap their framework's request
 * into it.
 */
import { TenantContext } from "../context/index.js";

/** The conventional tenant header (core/spec.md, TenantResolver). */
export const DEFAULT_HEADER = "X-Tenant-ID";

const TRACE_ID = /^[0-9a-f]{32}$/;

/** Minimal read-only view of an inbound request that resolvers need. */
export interface HttpRequestView {
  /** The named header's value, or `undefined` if absent. */
  header(name: string): string | undefined;
  /** The request host, possibly including a port. */
  host(): string | undefined;
}

/** Turns a request into a {@link TenantContext}, or `null` when no tenant is present. */
export interface TenantResolver {
  resolve(request: HttpRequestView): TenantContext | null;
}

/** Resolves the tenant from an HTTP header (default `X-Tenant-ID`). */
export class HeaderResolver implements TenantResolver {
  private readonly header: string;

  constructor(header: string = DEFAULT_HEADER) {
    this.header = header || DEFAULT_HEADER;
  }

  resolve(request: HttpRequestView): TenantContext | null {
    const id = (request.header(this.header) ?? "").trim();
    if (!id) {
      return null;
    }
    return TenantContext.create(id, "", traceIdFrom(request.header("traceparent")));
  }
}

/** Resolves the tenant from the left-most label of the request host. */
export class SubdomainResolver implements TenantResolver {
  private readonly baseDomain: string;
  private readonly ignore: string[];

  constructor(baseDomain: string, ignore?: string[]) {
    this.baseDomain = baseDomain ?? "";
    this.ignore = ignore && ignore.length > 0 ? ignore.map((s) => s.toLowerCase()) : ["www"];
  }

  resolve(request: HttpRequestView): TenantContext | null {
    let host = request.host();
    if (!host || !this.baseDomain) {
      return null;
    }
    host = stripPort(host).toLowerCase().replace(/\.$/, "");

    const suffix = "." + this.baseDomain.toLowerCase();
    if (!host.endsWith(suffix)) {
      return null;
    }

    const label = host.slice(0, host.length - suffix.length).split(".", 1)[0];
    if (!label || this.ignore.includes(label)) {
      return null;
    }
    return TenantContext.create(label, "", traceIdFrom(request.header("traceparent")));
  }
}

/** Best-effort W3C trace-id from a traceparent, or "" when missing/malformed. */
function traceIdFrom(traceparent: string | undefined): string {
  if (!traceparent) {
    return "";
  }
  const parts = traceparent.split("-");
  if (parts.length < 4) {
    return "";
  }
  const traceId = parts[1].toLowerCase();
  return TRACE_ID.test(traceId) ? traceId : "";
}

/** Removes a trailing `:port` for a plain host:port (leaves bare hosts untouched). */
function stripPort(host: string): string {
  const first = host.indexOf(":");
  if (first >= 0 && first === host.lastIndexOf(":")) {
    return host.slice(0, first);
  }
  return host;
}
