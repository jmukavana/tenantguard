/**
 * Cross-boundary tenant context propagation (core/spec.md, ContextPropagator).
 *
 * Tenant identity travels as W3C Baggage members under the `tg.*` namespace via
 * OpenTelemetry, so any instrumented hop preserves it. This is the TypeScript counterpart
 * of the Go/Java/Python propagation modules and shares their wire encoding.
 */
import {
  type Context,
  defaultTextMapGetter,
  defaultTextMapSetter,
  propagation,
  ROOT_CONTEXT,
} from "@opentelemetry/api";
import { W3CBaggagePropagator } from "@opentelemetry/core";
import { TenantContext } from "../context/index.js";

export const MEMBER_TENANT_ID = "tg.tenant.id";
export const MEMBER_TENANT_NAME = "tg.tenant.name";
export const MEMBER_TRACE_ID = "tg.trace.id";
export const MEMBER_METADATA = "tg.tenant.meta";

const w3c = new W3CBaggagePropagator();

/** A key/value carrier (HTTP headers, MQ headers). */
export type Carrier = Record<string, string>;

/** Serializes a {@link TenantContext} into and out of a carrier as W3C Baggage. */
export class Propagator {
  /** Write `tc` into `carrier`, preserving any baggage already in `context`. */
  inject(tc: TenantContext, carrier: Carrier, context: Context = ROOT_CONTEXT): void {
    if (!tc || !tc.isValid()) {
      throw new Error("cannot propagate an invalid tenant context");
    }
    let baggage = propagation.getBaggage(context) ?? propagation.createBaggage();
    baggage = baggage.setEntry(MEMBER_TENANT_ID, { value: tc.tenantId });
    if (tc.tenantName) {
      baggage = baggage.setEntry(MEMBER_TENANT_NAME, { value: tc.tenantName });
    }
    if (tc.traceId) {
      baggage = baggage.setEntry(MEMBER_TRACE_ID, { value: tc.traceId });
    }
    if (Object.keys(tc.metadata).length > 0) {
      baggage = baggage.setEntry(MEMBER_METADATA, { value: encodeMetadata(tc.metadata) });
    }
    w3c.inject(propagation.setBaggage(context, baggage), carrier, defaultTextMapSetter);
  }

  /** Reconstruct a TenantContext from `carrier`, or `null` when it carries no tenant. */
  extract(carrier: Carrier, context: Context = ROOT_CONTEXT): TenantContext | null {
    const baggage = propagation.getBaggage(w3c.extract(context, carrier, defaultTextMapGetter));
    const id = baggage?.getEntry(MEMBER_TENANT_ID)?.value;
    if (!id) {
      return null;
    }
    const rawMeta = baggage!.getEntry(MEMBER_METADATA)?.value;
    const metadata = rawMeta ? (JSON.parse(rawMeta) as Record<string, string>) : {};
    return TenantContext.create(
      id,
      baggage!.getEntry(MEMBER_TENANT_NAME)?.value ?? "",
      baggage!.getEntry(MEMBER_TRACE_ID)?.value ?? "",
      metadata,
    );
  }
}

/** Canonical, sorted, compact JSON — byte-compatible with the Go/Java/Python encoders. */
function encodeMetadata(metadata: Readonly<Record<string, string>>): string {
  const sorted: Record<string, string> = {};
  for (const key of Object.keys(metadata).sort()) {
    sorted[key] = metadata[key];
  }
  return JSON.stringify(sorted);
}
