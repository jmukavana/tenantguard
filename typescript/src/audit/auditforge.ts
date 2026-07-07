/**
 * AuditForge delivery client (ecosystem/auditforge/spec.md).
 *
 * {@link AuditForgeHook} signs each event and POSTs it to AuditForge's POST /v1/events. It is
 * the production replacement for {@link LoggingAuditHook}: every event is HMAC-signed with a
 * shared key and appended to the tamper-evident audit log.
 */
import { type AuditHook, TenantEvent, sign } from "./index.js";

const EVENTS_PATH = "/v1/events";

/** Thrown when an event cannot be delivered: rejected, unacknowledged, or undeliverable. */
export class AuditForgeError extends Error {}

/** Internal signal that an attempt failed but is worth retrying. */
class RetryableError extends Error {}

interface IngestAck {
  received: number;
  accepted: string[];
  duplicate: string[];
  rejected: { eventId: string; reason: string }[];
}

export interface AuditForgeHookOptions {
  /** AuditForge base URL, e.g. "https://auditforge.internal". */
  endpoint: string;
  /** Identifies the shared signing key; sent as keyId in every request. */
  keyId: string;
  /** Shared HMAC-SHA256 key used to sign each event. */
  secret: string | Buffer;
  /** Extra attempts after the first on a retryable failure. Default 2. */
  maxRetries?: number;
  /** Per-request timeout in milliseconds. Default 10000. */
  timeoutMs?: number;
  /** Delay before retry attempt n (1-based). Default exponential from 100ms. */
  backoffMs?: (attempt: number) => number;
  /** Injectable fetch, for testing. Defaults to the global fetch. */
  fetch?: typeof fetch;
}

export class AuditForgeHook implements AuditHook {
  private readonly endpoint: string;
  private readonly keyId: string;
  private readonly secret: string | Buffer;
  private readonly maxRetries: number;
  private readonly timeoutMs: number;
  private readonly backoffMs: (attempt: number) => number;
  private readonly fetchFn: typeof fetch;

  constructor(options: AuditForgeHookOptions) {
    this.endpoint = options.endpoint;
    this.keyId = options.keyId;
    this.secret = options.secret;
    this.maxRetries = options.maxRetries ?? 2;
    this.timeoutMs = options.timeoutMs ?? 10_000;
    this.backoffMs = options.backoffMs ?? ((attempt) => 100 * 2 ** (attempt - 1));
    this.fetchFn = options.fetch ?? fetch;
  }

  /**
   * Sign `event`, submit it as a one-event batch, and resolve once AuditForge has stored it.
   * A duplicate (same eventId) is an idempotent success. Rejects with {@link AuditForgeError}
   * if the event is rejected, comes back unacknowledged, or cannot be delivered.
   */
  async publish(event: TenantEvent): Promise<void> {
    const signed = new TenantEvent({
      eventId: event.eventId,
      eventType: event.eventType,
      tenantId: event.tenantId,
      timestamp: event.timestamp,
      outcome: event.outcome,
      traceId: event.traceId,
      actor: event.actor,
      resource: event.resource,
      metadata: event.metadata,
      signature: sign(event, this.secret),
    });
    const body = `{"keyId":${JSON.stringify(this.keyId)},"events":[${signed.toJson()}]}`;

    const ack = await this.post(body);
    this.checkAck(event.eventId, ack);
  }

  /** Deliver `body`, retrying retryable failures up to maxRetries. */
  private async post(body: string): Promise<IngestAck> {
    const url = this.endpoint.replace(/\/+$/, "") + EVENTS_PATH;
    let lastError: unknown;
    for (let attempt = 0; attempt <= this.maxRetries; attempt++) {
      if (attempt > 0) {
        await delay(this.backoffMs(attempt));
      }
      try {
        return await this.attempt(url, body);
      } catch (error) {
        if (!(error instanceof RetryableError)) {
          throw error;
        }
        lastError = error;
      }
    }
    throw new AuditForgeError(
      `giving up after ${this.maxRetries + 1} attempts: ${message(lastError)}`,
    );
  }

  /** Make one POST, throwing RetryableError for failures worth retrying. */
  private async attempt(url: string, body: string): Promise<IngestAck> {
    let response: Response;
    try {
      response = await this.fetchFn(url, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body,
        signal: AbortSignal.timeout(this.timeoutMs),
      });
    } catch (error) {
      throw new RetryableError(`POST failed: ${message(error)}`);
    }

    const text = await response.text();
    if (response.status === 200) {
      try {
        return JSON.parse(text) as IngestAck;
      } catch {
        throw new AuditForgeError(`malformed ack: ${text}`);
      }
    }
    if (response.status >= 500) {
      throw new RetryableError(`server error ${response.status}: ${text}`);
    }
    // 4xx is a transport/auth failure (bad key, malformed batch); retrying won't help.
    throw new AuditForgeError(`request rejected ${response.status}: ${text}`);
  }

  /** Throw if `eventId` was rejected or is absent from every ack partition. */
  private checkAck(eventId: string, ack: IngestAck): void {
    for (const rejection of ack.rejected ?? []) {
      if (rejection.eventId === eventId) {
        throw new AuditForgeError(`event ${eventId} rejected: ${rejection.reason}`);
      }
    }
    if ((ack.accepted ?? []).includes(eventId) || (ack.duplicate ?? []).includes(eventId)) {
      return;
    }
    throw new AuditForgeError(`event ${eventId} missing from ack (received=${ack.received})`);
  }
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function message(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
