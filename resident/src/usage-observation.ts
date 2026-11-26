// usage-observation.ts — C03: turn provider usage events into
// identity-carrying observations on the spine.
//
// The two-phase order is frozen by PS §4.1⑥: the raw event persists first,
// the derived observation second. A crash between the two leaves the raw
// event recoverable at startup (PS-F1①): re-derive, and the idempotent
// dedupe key keeps the ledger from double-counting.
//
// Nothing here estimates. A fact is recorded only when the family's rule
// extracts it; whatever the rule cannot complete stays raw material.

import type { Exec } from "./types";

export type UsageFamily = "openai-chat" | "openai-responses" | "anthropic" | "google" | "unknown";

export type UsageProvability = "native" | "cursor" | "none";

export interface UsageEventFact {
  family: UsageFamily;
  modelId: string;
  outputTokens: string;
  inputTokens: string;
  reportKind: "cumulative" | "incremental";
  finality: "interim" | "final";
  nativeEventId: string;
  provability: UsageProvability;
  raw: unknown;
  occurredAt?: string;
}

/** A raw usage event that carries no complete fact: it accumulates as blob
 * material only (phase one), never an observation. The stored content is a
 * normalized usage fragment — identity source plus usage content, and never
 * conversation content, which the spine does not accumulate. */
export interface RawUsageEvent {
  family: UsageFamily;
  modelId: string;
  raw: unknown;
  occurredAt?: string;
}

export interface UsageEvents {
  facts: UsageEventFact[];
  rawOnly: RawUsageEvent[];
}

const usageFamilyByPath = (path: string): UsageFamily => {
  if (path.includes("/chat/completions")) return "openai-chat";
  if (path.includes("/responses")) return "openai-responses";
  if (path.includes("/messages")) return "anthropic";
  if (path.includes("generateContent")) return "google";
  return "unknown";
};

export function usageFamilyForOperation(operationPath: string): UsageFamily {
  return usageFamilyByPath(operationPath);
}

function nonNegativeInt(value: unknown): string | null {
  if (typeof value === "string" && /^(0|[1-9][0-9]*)$/.test(value)) return value;
  if (typeof value === "number" && Number.isInteger(value) && value >= 0) return String(value);
  return null;
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value) ? value as Record<string, unknown> : null;
}

function usageShape(body: Record<string, unknown>): Record<string, unknown> | null {
  for (const key of ["usage", "usageMetadata"]) {
    const shaped = asRecord(body[key]);
    if (shaped) return shaped;
  }
  return null;
}

// Conventional count keys, by direction. The family decides the meaning;
// these only locate the numbers inside a usage-shaped object.
function inputCount(usage: Record<string, unknown>): string | null {
  return nonNegativeInt(usage.input_tokens) ?? nonNegativeInt(usage.prompt_tokens) ?? nonNegativeInt(usage.promptTokenCount);
}
function outputCount(usage: Record<string, unknown>): string | null {
  return nonNegativeInt(usage.output_tokens) ?? nonNegativeInt(usage.completion_tokens) ?? nonNegativeInt(usage.candidatesTokenCount);
}

function factFromUsage(family: UsageFamily, usage: Record<string, unknown>, modelId: string, finality: "interim" | "final", nativeEventId: string, occurredAt?: string): UsageEventFact | null {
  const input = inputCount(usage);
  const output = outputCount(usage);
  if (input === null || output === null) return null;
  return {
    family, modelId,
    outputTokens: output, inputTokens: input,
    reportKind: "cumulative", finality,
    nativeEventId,
    provability: nativeEventId === "" ? "none" : "native",
    raw: usage,
    ...(occurredAt ? { occurredAt } : {}),
  };
}

// The per-family stream state. Each family accumulates what its rule can
// say; whatever it cannot say is kept raw-only.
export class UsageStream {
  readonly family: UsageFamily;
  private readonly requestedModel: string;
  private facts: UsageEventFact[] = [];
  private rawOnly: RawUsageEvent[] = [];
  private openAiResponseId = "";
  private openAiFinalSeen = false;
  private anthropicMessageId = "";
  private anthropicModel = "";
  private anthropicInput: string | null = null;
  private anthropicOutput: string | null = null;

  constructor(family: UsageFamily, requestedModel: string) {
    this.family = family;
    this.requestedModel = requestedModel === "" ? "unreported" : requestedModel;
  }

  /** Consume one decoded response event (an SSE data payload or the JSON
   * body). Returns nothing; the terminal events() closes the stream. */
  push(payload: unknown): void {
    const body = asRecord(payload);
    if (body === null) return;
    switch (this.family) {
      case "openai-chat": this.pushOpenAiChat(body); break;
      case "openai-responses": this.pushOpenAiResponses(body); break;
      case "anthropic": this.pushAnthropic(body); break;
      case "google": this.pushGoogle(body); break;
      case "unknown": this.pushUnknown(body); break;
    }
  }

  private pushOpenAiChat(body: Record<string, unknown>): void {
    const usage = usageShape(body);
    if (usage === null) return;
    if (typeof body.id === "string") this.openAiResponseId = body.id;
    const id = this.openAiResponseId;
    const modelId = typeof body.model === "string" ? body.model : this.requestedModel;
    const choices = Array.isArray(body.choices) ? body.choices : [];
    const final = choices.length === 0;
    if (final) {
      // The final chunk is the request's one cumulative snapshot.
      this.openAiFinalSeen = true;
      this.facts.push(this.completeOrRaw(usage, modelId, "final", id));
    } else if (!this.openAiFinalSeen) {
      // Per-chunk usage before the final: interim reports, no per-chunk
      // native identity (the response id is shared, so none-provable).
      const fact = factFromUsage(this.family, usage, modelId, "interim", "");
      if (fact) this.facts.push(fact);
      else this.rawOnly.push({ family: this.family, raw: usage });
    }
  }

  private pushOpenAiResponses(body: Record<string, unknown>): void {
    const type = typeof body.type === "string" ? body.type : "";
    if (type === "response.completed") {
      // The stream event wraps the completed response.
      this.pushResponsesCompleted(asRecord(body.response) ?? body);
      return;
    }
    // The non-streaming body is the completed response itself.
    if (typeof body.id === "string" && body.status === "completed") {
      this.pushResponsesCompleted(body);
    }
  }

  private pushResponsesCompleted(body: Record<string, unknown>): void {
    // Duplicate or late completed events converge on the version rule: the
    // same response id re-enters, it does not re-count.
    const usage = usageShape(body);
    if (usage === null) return;
    const id = typeof body.id === "string" ? body.id : "";
    const modelId = typeof body.model === "string" ? body.model : this.requestedModel;
    const fact = factFromUsage(this.family, usage, modelId, "final", id);
    if (fact) this.facts.push(fact);
    else this.rawOnly.push({ family: this.family, raw: usage });
  }

  private pushAnthropic(body: Record<string, unknown>): void {
    const type = typeof body.type === "string" ? body.type : "";
    if (type === "message_start") {
      const message = asRecord(body.message);
      if (message === null) return;
      this.anthropicMessageId = typeof message.id === "string" ? message.id : "";
      this.anthropicModel = typeof message.model === "string" ? message.model : this.requestedModel;
      const usage = usageShape(message);
      if (usage !== null) this.anthropicInput = inputCount(usage);
      return;
    }
    if (type === "message_delta" || type === "message_stop") {
      const usage = usageShape(body);
      if (usage !== null) this.anthropicOutput = outputCount(usage);
      return;
    }
  }

  private pushGoogle(body: Record<string, unknown>): void {
    const usage = usageShape(body);
    if (usage === null) return;
    const modelId = typeof body.responseModel === "string" ? body.responseModel
      : typeof body.modelVersion === "string" ? body.modelVersion : this.requestedModel;
    const fact = factFromUsage(this.family, usage, modelId, "final", "");
    if (fact) this.facts.push(fact);
    else this.rawOnly.push({ family: this.family, raw: usage });
  }

  private pushUnknown(body: Record<string, unknown>): void {
    // No family rule: locate a usage-shaped object, extract only when both
    // counts are present under conventional keys, and keep it interim —
    // undecidable, never confirmed. Anything less complete stays raw-only.
    const usage = usageShape(body);
    if (usage === null) return;
    const fact = factFromUsage(this.family, usage, this.requestedModel, "interim", "");
    if (fact) this.facts.push(fact);
    else this.rawOnly.push({ family: this.family, modelId: this.requestedModel, raw: usage });
  }

  private completeOrRaw(usage: Record<string, unknown>, modelId: string, finality: "interim" | "final", nativeEventId: string): UsageEventFact {
    const fact = factFromUsage(this.family, usage, modelId, finality, nativeEventId);
    if (fact) return fact;
    this.rawOnly.push({ family: this.family, modelId, raw: usage });
    // A fact with empty token strings never reaches the spine; the caller
    // filters it out in events().
    return { family: this.family, modelId, outputTokens: "", inputTokens: "", reportKind: "cumulative", finality, nativeEventId, provability: "none", raw: usage };
  }

  /** Close the stream: the family's terminal facts, including the
   * anthropic single-request cumulative snapshot. interrupted marks a
   * stream that ended before its terminal event. */
  events(interrupted = false): UsageEvents {
    if (this.family === "anthropic") this.finishAnthropic(interrupted);
    if (this.family === "openai-chat") this.finishOpenAiChat(interrupted);
    const facts = this.facts.filter(fact => fact.outputTokens !== "" && fact.inputTokens !== "");
    return { facts, rawOnly: this.rawOnly };
  }

  private finishOpenAiChat(interrupted: boolean): void {
    if (this.openAiFinalSeen) return;
    // A completed request without its final usage chunk records its
    // accumulated non-zero value in the UNKNOWN channel: never a zero,
    // never a confirmed final.
    const last = [...this.facts].reverse().find(fact => fact.reportKind === "cumulative");
    if (!last) return;
    if (interrupted && last.finality === "interim" && last.provability === "none") return;
    this.facts.push({ ...last, finality: interrupted ? "interim" : "final", nativeEventId: "", provability: "none" });
  }

  private finishAnthropic(interrupted: boolean): void {
    if (this.anthropicInput === null || this.anthropicOutput === null) {
      if (this.anthropicInput !== null || this.anthropicOutput !== null) {
        // A partial anthropic fact (one side only) is raw material, not an
        // observation: the frozen payload needs both counts.
        this.rawOnly.push({ family: this.family, modelId: this.anthropicModel || this.requestedModel, raw: { input_tokens: this.anthropicInput, output_tokens: this.anthropicOutput } });
      }
      return;
    }
    // The request converges to one cumulative snapshot: input fixed at
    // start, output at the last delta. An interrupted stream never claims
    // its snapshot is final.
    this.facts = this.facts.filter(fact => fact.finality !== "interim" || fact.family !== "anthropic");
    const usage = { input_tokens: this.anthropicInput, output_tokens: this.anthropicOutput };
    const finality: "interim" | "final" = interrupted ? "interim" : "final";
    this.facts.push(factFromUsage(this.family, usage, this.anthropicModel, finality, this.anthropicMessageId) ?? {
      family: this.family, modelId: this.anthropicModel,
      outputTokens: this.anthropicOutput, inputTokens: this.anthropicInput,
      reportKind: "cumulative", finality,
      nativeEventId: this.anthropicMessageId,
      provability: this.anthropicMessageId === "" ? "none" : "native",
      raw: usage,
    });
  }
}

// ---- persistence ----------------------------------------------------------

// The durable phase-one record is a normalized usage fragment: the identity
// source plus the usage content, and nothing else. It is small enough to be
// cheap, carries no conversation content, and is exactly what phase two and
// startup recovery re-derive from. The provider's raw response body is never
// persisted.
function factFragment(fact: UsageEventFact): Record<string, unknown> {
  return {
    usage_family: fact.family,
    model_id: fact.modelId,
    input_tokens: fact.inputTokens,
    output_tokens: fact.outputTokens,
    report_kind: fact.reportKind,
    finality: fact.finality,
    identity_provability: fact.provability,
    ...(fact.nativeEventId === "" ? {} : { native_event_id: fact.nativeEventId }),
  };
}

function rawOnlyFragment(raw: RawUsageEvent): Record<string, unknown> {
  const usage = asRecord(raw.raw) ?? {};
  return { usage_family: raw.family, model_id: raw.modelId, ...usage };
}

function usageWire(fragment: Record<string, unknown>, localRequestId: string, providerRequestId: string, nativeEventId: string, occurredAt?: string): Uint8Array {
  const body: Record<string, unknown> = {
    local_request_id: localRequestId,
    usage_family: fragment.usage_family,
    raw: fragment,
    ...(providerRequestId !== "" ? { provider_request_id: providerRequestId } : {}),
    ...(nativeEventId !== "" ? { native_event_id: nativeEventId } : {}),
    ...(occurredAt ? { occurred_at: occurredAt } : {}),
  };
  return new TextEncoder().encode(JSON.stringify(body));
}

function observationWire(fact: UsageEventFact, localRequestId: string, providerRequestId: string, eventSeq: number, sourceVersion: number): Uint8Array {
  return new TextEncoder().encode(JSON.stringify({
    local_request_id: localRequestId,
    provider_request_id: providerRequestId,
    event_seq: eventSeq,
    source_version: sourceVersion,
    origin: providerRequestId === "" ? "local" : "provider",
    usage_family: fact.family,
    model_id: fact.modelId,
    output_tokens: fact.outputTokens,
    input_tokens: fact.inputTokens,
    report_kind: fact.reportKind,
    finality: fact.finality,
    identity_provability: fact.provability,
    ...(fact.occurredAt ? { occurred_at: fact.occurredAt } : {}),
  }));
}

async function call(runMc: Exec, args: string[], body: Uint8Array): Promise<Record<string, unknown> | null> {
  const result = await runMc(args, body);
  if (result.exitCode !== 0) return null;
  try {
    return JSON.parse(result.stdout) as Record<string, unknown>;
  } catch {
    return null;
  }
}

/** Persist one collected event stream in the frozen two-phase order. A
 * phase-two failure leaves the raw fragment in place for startup recovery; it
 * never retries phase one (the number would re-issue). */
export async function persistUsageEvents(runMc: Exec, localRequestId: string, providerRequestId: string, events: UsageEvents): Promise<number> {
  let persisted = 0;
  for (const fact of events.facts) {
    const phase1 = await call(runMc, ["usage", "event"], usageWire(factFragment(fact), localRequestId, providerRequestId, fact.nativeEventId, fact.occurredAt));
    const seq = typeof phase1?.event_seq === "number" ? phase1.event_seq : null;
    if (seq === null) continue;
    persisted += 1;
    const phase2 = await call(runMc, ["usage", "observation"], observationWire(fact, localRequestId, providerRequestId, seq, 1));
    if (phase2 === null) {
      // The raw fragment stays on the spine; startup recovery re-derives
      // this observation from it. Nothing is re-numbered.
      break;
    }
  }
  for (const raw of events.rawOnly) {
    const phase1 = await call(runMc, ["usage", "event"], usageWire(rawOnlyFragment(raw), localRequestId, providerRequestId, "", raw.occurredAt));
    if (phase1 !== null) persisted += 1;
  }
  return persisted;
}

// Rebuild a complete fact from a stored fragment. A fragment that cannot be
// completed (a partial, raw-only record) returns null and stays raw material.
export function factFromFragment(family: UsageFamily, fragment: Record<string, unknown>): UsageEventFact | null {
  const modelId = typeof fragment.model_id === "string" ? fragment.model_id : "";
  const reportKind = fragment.report_kind === "cumulative" || fragment.report_kind === "incremental" ? fragment.report_kind : null;
  const finality = fragment.finality === "interim" || fragment.finality === "final" ? fragment.finality : null;
  const provability = fragment.identity_provability === "native" || fragment.identity_provability === "cursor" || fragment.identity_provability === "none"
    ? fragment.identity_provability : null;
  const input = nonNegativeInt(fragment.input_tokens);
  const output = nonNegativeInt(fragment.output_tokens);
  if (modelId === "" || reportKind === null || finality === null || provability === null || input === null || output === null) return null;
  const nativeEventId = typeof fragment.native_event_id === "string" ? fragment.native_event_id : "";
  return {
    family, modelId, outputTokens: output, inputTokens: input,
    reportKind, finality, nativeEventId,
    provability,
    raw: { input_tokens: input, output_tokens: output },
  };
}

/** Startup recovery (PS-F1①): re-derive observations from raw fragments that
 * never got one. The dedupe key makes a redelivered derivation a no-op, so
 * recovery can run without double-counting. */
export async function recoverPendingUsage(runMc: Exec, limit = 200): Promise<number> {
  const probe = await call(runMc, ["usage", "pending"], new TextEncoder().encode(JSON.stringify({ limit })));
  const pending = Array.isArray(probe?.pending) ? probe.pending as Record<string, unknown>[] : [];
  let recovered = 0;
  for (const entry of pending) {
    const localRequestId = typeof entry.local_request_id === "string" ? entry.local_request_id : "";
    const providerRequestId = typeof entry.provider_request_id === "string" ? entry.provider_request_id : "";
    const family = typeof entry.usage_family === "string" ? (entry.usage_family as UsageFamily) : "unknown";
    const eventSeq = typeof entry.event_seq === "number" ? entry.event_seq : null;
    if (localRequestId === "" || eventSeq === null) continue;
    const fragment = asRecord(entry.raw);
    if (fragment === null) continue;
    const fact = factFromFragment(family, fragment);
    if (fact === null) continue;
    const phase2 = await call(runMc, ["usage", "observation"], observationWire(fact, localRequestId, providerRequestId, eventSeq, 1));
    if (phase2 !== null) recovered += 1;
  }
  return recovered;
}
