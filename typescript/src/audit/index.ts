/**
 * Tenant audit events and hooks (core/spec.md, Observability & Audit).
 */
import { randomBytes } from "node:crypto";

/** The result of an audited operation, as it appears on the wire. */
export type Outcome = "allowed" | "denied" | "error";

export interface TenantEventInit {
  eventId: string;
  eventType: string;
  tenantId: string;
  timestamp: Date;
  outcome: Outcome;
  traceId?: string;
  actor?: string;
  resource?: string;
  metadata?: Record<string, string>;
  signature?: string;
}

/**
 * Structured audit record. {@link toJson} is the cross-language wire contract and mirrors
 * core/schemas/TenantEvent.schema.json: required keys are always present, empty optional
 * keys are omitted, and metadata keys are sorted for determinism.
 */
export class TenantEvent {
  readonly eventId: string;
  readonly eventType: string;
  readonly tenantId: string;
  readonly timestamp: Date;
  readonly outcome: Outcome;
  readonly traceId: string;
  readonly actor: string;
  readonly resource: string;
  readonly metadata: Readonly<Record<string, string>>;
  readonly signature: string;

  constructor(init: TenantEventInit) {
    this.eventId = init.eventId;
    this.eventType = init.eventType;
    this.tenantId = init.tenantId;
    this.timestamp = init.timestamp;
    this.outcome = init.outcome;
    this.traceId = init.traceId ?? "";
    this.actor = init.actor ?? "";
    this.resource = init.resource ?? "";
    this.metadata = Object.freeze({ ...(init.metadata ?? {}) });
    this.signature = init.signature ?? "";
    Object.freeze(this);
  }

  /** Build an event stamped with a fresh id and the current time. */
  static create(eventType: string, tenantId: string, traceId: string, outcome: Outcome): TenantEvent {
    return new TenantEvent({
      eventId: randomBytes(16).toString("hex"),
      eventType,
      tenantId,
      timestamp: new Date(),
      outcome,
      traceId,
    });
  }

  /** Serialize to canonical JSON (the AuditForge wire form). */
  toJson(): string {
    const obj: Record<string, unknown> = {
      eventId: this.eventId,
      eventType: this.eventType,
      tenantId: this.tenantId,
      timestamp: isoZ(this.timestamp),
      outcome: this.outcome,
    };
    if (this.traceId) obj.traceId = this.traceId;
    if (this.actor) obj.actor = this.actor;
    if (this.resource) obj.resource = this.resource;
    const keys = Object.keys(this.metadata);
    if (keys.length > 0) {
      const sorted: Record<string, string> = {};
      for (const k of keys.sort()) sorted[k] = this.metadata[k];
      obj.metadata = sorted;
    }
    if (this.signature) obj.signature = this.signature;
    return JSON.stringify(obj);
  }
}

/** Publishes tenant audit events to a sink (AuditForge in production). */
export interface AuditHook {
  publish(event: TenantEvent): void;
}

/** Development {@link AuditHook} that logs events. Replace with the AuditForge client. */
export class LoggingAuditHook implements AuditHook {
  publish(event: TenantEvent): void {
    console.info("tenant.audit", event.toJson());
  }
}

/** RFC 3339 / ISO-8601 using 'Z', without milliseconds when zero (matches Go/Java/Python). */
function isoZ(date: Date): string {
  return date.toISOString().replace(/\.\d{3}Z$/, "Z");
}
