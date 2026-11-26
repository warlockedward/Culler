import { expect, test } from "bun:test";
import type { Exec, ExecResult } from "./types";
import { UsageStream, persistUsageEvents, recoverPendingUsage, usageFamilyForOperation, type UsageEvents } from "./usage-observation";

const sse = (value: unknown) => `data: ${JSON.stringify(value)}\n\n`;
const stream = (family: string, model: string, ...payloads: unknown[]) => {
  const usage = new UsageStream(usageFamilyForOperation(family), model);
  for (const payload of payloads) usage.push(payload);
  return usage.events();
};

function fakeMc(handler: (argv: string[], body: unknown) => ExecResult): { exec: Exec; calls: { argv: string[]; body: unknown }[] } {
  const calls: { argv: string[]; body: unknown }[] = [];
  const exec: Exec = async (argv, stdin) => {
    const body = stdin === undefined ? undefined : JSON.parse(new TextDecoder().decode(stdin));
    calls.push({ argv, body });
    return handler(argv, body);
  };
  return { exec, calls };
}

const ok = (value: unknown): ExecResult => ({ exitCode: 0, stdout: JSON.stringify(value), stderr: "" });
const fail = (): ExecResult => ({ exitCode: 1, stdout: "", stderr: "refused" });

test("operation paths map to their usage families", () => {
  expect(usageFamilyForOperation("/v1/chat/completions")).toBe("openai-chat");
  expect(usageFamilyForOperation("/v1/responses")).toBe("openai-responses");
  expect(usageFamilyForOperation("/v1/messages")).toBe("anthropic");
  expect(usageFamilyForOperation("/v1beta/models/gemini-2:generateContent")).toBe("google");
  expect(usageFamilyForOperation("/v1/something-else")).toBe("unknown");
});

test("openai-chat final chunk is the request's one cumulative native snapshot", () => {
  const events = stream("/v1/chat/completions", "probe-model",
    JSON.parse(sse({ id: "chatcmpl-1", choices: [{ text: "partial" }], usage: { prompt_tokens: 10, completion_tokens: 3 } }).slice(5, -2)),
    JSON.parse(sse({ id: "chatcmpl-1", choices: [], usage: { prompt_tokens: 15, completion_tokens: 7 } }).slice(5, -2)),
  );
  expect(events.rawOnly).toEqual([]);
  expect(events.facts).toHaveLength(2);
  expect(events.facts[0]).toMatchObject({ finality: "interim", provability: "none", outputTokens: "3", inputTokens: "10" });
  expect(events.facts[1]).toMatchObject({ finality: "final", provability: "native", nativeEventId: "chatcmpl-1", outputTokens: "7", inputTokens: "15" });
});

test("openai-chat without its final chunk records the non-zero value as unknown", () => {
  const events = stream("/v1/chat/completions", "probe-model",
    JSON.parse(sse({ id: "chatcmpl-1", choices: [{ text: "partial" }], usage: { prompt_tokens: 10, completion_tokens: 3 } }).slice(5, -2)),
  );
  const last = events.facts.at(-1)!;
  expect(last).toMatchObject({ finality: "final", provability: "none", outputTokens: "3", inputTokens: "10" });
  // The interrupted variant never claims its missing final.
  const interrupted = new UsageStream("openai-chat", "probe-model");
  interrupted.push(JSON.parse(sse({ id: "chatcmpl-1", choices: [{ text: "partial" }], usage: { prompt_tokens: 10, completion_tokens: 3 } }).slice(5, -2)));
  const tail = interrupted.events(true).facts.at(-1)!;
  expect(tail).toMatchObject({ finality: "interim", provability: "none" });
});

test("openai-responses duplicate completed events converge, they do not re-count", () => {
  const completed = { type: "response.completed", response: { id: "resp-1", model: "probe-model", usage: { input_tokens: 12, output_tokens: 5 } } };
  const events = stream("/v1/responses", "probe-model", completed, completed);
  expect(events.facts).toHaveLength(2);
  for (const fact of events.facts) expect(fact).toMatchObject({ finality: "final", provability: "native", nativeEventId: "resp-1" });
});

test("anthropic converges one request to its cumulative snapshot", () => {
  const events = stream("/v1/messages", "probe-model",
    JSON.parse(sse({ type: "message_start", message: { id: "msg-1", model: "probe-model", usage: { input_tokens: 20 } } }).slice(5, -2)),
    JSON.parse(sse({ type: "message_delta", usage: { output_tokens: 4 } }).slice(5, -2)),
    JSON.parse(sse({ type: "message_delta", usage: { output_tokens: 9 } }).slice(5, -2)),
    JSON.parse(sse({ type: "message_stop" }).slice(5, -2)),
  );
  expect(events.facts).toHaveLength(1);
  expect(events.facts[0]).toMatchObject({ finality: "final", provability: "native", nativeEventId: "msg-1", inputTokens: "20", outputTokens: "9", reportKind: "cumulative" });
});

test("anthropic interrupted at the last delta never claims its snapshot final", () => {
  const usage = new UsageStream("anthropic", "probe-model");
  usage.push(JSON.parse(sse({ type: "message_start", message: { id: "msg-1", usage: { input_tokens: 20 } } }).slice(5, -2)));
  usage.push(JSON.parse(sse({ type: "message_delta", usage: { output_tokens: 4 } }).slice(5, -2)));
  const events = usage.events(true);
  expect(events.facts).toHaveLength(1);
  expect(events.facts[0]).toMatchObject({ finality: "interim", nativeEventId: "msg-1" });
});

test("anthropic with one side only stays raw material", () => {
  const events = stream("/v1/messages", "probe-model",
    JSON.parse(sse({ type: "message_start", message: { id: "msg-1", usage: { input_tokens: 20 } } }).slice(5, -2)),
  );
  expect(events.facts).toEqual([]);
  expect(events.rawOnly).toHaveLength(1);
});

test("google usageMetadata needs both counts to become an observation", () => {
  const complete = stream("/v1beta/models/gemini-2:generateContent", "probe-model",
    JSON.parse(sse({ usageMetadata: { promptTokenCount: 11, candidatesTokenCount: 6 } }).slice(5, -2)));
  expect(complete.facts).toHaveLength(1);
  expect(complete.facts[0]).toMatchObject({ outputTokens: "6", inputTokens: "11", provability: "none" });
  const partial = stream("/v1beta/models/gemini-2:generateContent", "probe-model",
    JSON.parse(sse({ usageMetadata: { promptTokenCount: 11 } }).slice(5, -2)));
  expect(partial.facts).toEqual([]);
  expect(partial.rawOnly).toHaveLength(1);
});

test("the unknown family only reports complete usage shapes, never confirmed", () => {
  const shaped = stream("/v1/custom-endpoint", "probe-model",
    JSON.parse(sse({ usage: { input_tokens: 2, output_tokens: 1 } }).slice(5, -2)));
  expect(shaped.facts).toHaveLength(1);
  expect(shaped.facts[0]).toMatchObject({ family: "unknown", finality: "interim", provability: "none" });
  const empty = stream("/v1/custom-endpoint", "probe-model",
    JSON.parse(sse({ candidates: [] }).slice(5, -2)));
  expect(empty.facts).toEqual([]);
  expect(empty.rawOnly).toEqual([]);
});

test("token counts keep their lexical form through the wire", () => {
  const events = stream("/v1/chat/completions", "probe-model",
    JSON.parse(sse({ id: "chatcmpl-1", choices: [], usage: { prompt_tokens: "15", completion_tokens: 7 } }).slice(5, -2)),
  );
  const fact = events.facts.at(-1)!;
  expect(fact.inputTokens).toBe("15");
  expect(fact.outputTokens).toBe("7");
});

test("the response model wins, the requested model is the fact fallback", () => {
  const events = stream("/v1/chat/completions", "requested-model",
    JSON.parse(sse({ id: "chatcmpl-1", model: "served-model", choices: [], usage: { prompt_tokens: 1, completion_tokens: 1 } }).slice(5, -2)),
  );
  expect(events.facts.at(-1)!.modelId).toBe("served-model");
  const fallback = stream("/v1/chat/completions", "requested-model",
    JSON.parse(sse({ id: "chatcmpl-1", choices: [], usage: { prompt_tokens: 1, completion_tokens: 1 } }).slice(5, -2)),
  );
  expect(fallback.facts.at(-1)!.modelId).toBe("requested-model");
});

test("persistence runs the frozen two-phase order per event", async () => {
  const { exec, calls } = fakeMc((argv, body) => {
    if (argv[1] === "event") return ok({ request_identity: (body as { local_request_id: string }).local_request_id, event_seq: calls.filter(c => c.argv[1] === "event").length, created: true });
    if (argv[1] === "observation") return ok({ observation_id: "ab".repeat(32), state: "reported", created: true });
    return fail();
  });
  const events: UsageEvents = {
    facts: [
      { family: "openai-chat", modelId: "m", outputTokens: "3", inputTokens: "10", reportKind: "cumulative", finality: "final", nativeEventId: "chatcmpl-1", provability: "native", raw: { prompt_tokens: 10, completion_tokens: 3 } },
    ],
    rawOnly: [{ family: "google", raw: { promptTokenCount: 1 } }],
  };
  const persisted = await persistUsageEvents(exec, "local-1", "prov-1", events);
  expect(persisted).toBe(2);
  const shapes = calls.map(call => call.argv.slice(0, 2).join(" "));
  expect(shapes).toEqual(["usage event", "usage observation", "usage event"]);
  const observation = calls[1].body as Record<string, unknown>;
  expect(observation.event_seq).toBe(1);
  expect(observation.origin).toBe("provider");
  expect(observation.output_tokens).toBe("3");
});

test("a failed phase one is never followed by phase two", async () => {
  const { exec, calls } = fakeMc(() => fail());
  const events: UsageEvents = {
    facts: [{ family: "openai-chat", modelId: "m", outputTokens: "3", inputTokens: "10", reportKind: "cumulative", finality: "final", nativeEventId: "", provability: "none", raw: {} }],
    rawOnly: [],
  };
  const persisted = await persistUsageEvents(exec, "local-1", "", events);
  expect(persisted).toBe(0);
  expect(calls).toHaveLength(1);
  expect(calls[0].argv).toEqual(["usage", "event"]);
});

test("recovery re-derives pending fragments and a clean spine recovers nothing", async () => {
  const { exec, calls } = fakeMc((argv) => {
    if (argv[1] === "pending") return ok({ pending: [
      { local_request_id: "local-1", usage_family: "openai-chat", event_seq: 1,
        raw: { usage_family: "openai-chat", model_id: "m1", input_tokens: "10", output_tokens: "3", report_kind: "cumulative", finality: "final", identity_provability: "none" } },
      { local_request_id: "local-2", provider_request_id: "prov-2", usage_family: "google", event_seq: 2,
        raw: { usage_family: "google", model_id: "m2", input_tokens: "4", output_tokens: "2", report_kind: "cumulative", finality: "final", identity_provability: "none" } },
      { local_request_id: "local-3", usage_family: "google", event_seq: 3,
        raw: { usage_family: "google", model_id: "m3", promptTokenCount: 4 } },
    ] });
    if (argv[1] === "observation") return ok({ observation_id: "cd".repeat(32), state: "reported", created: true });
    return fail();
  });
  const recovered = await recoverPendingUsage(exec);
  // The partial fragment (local-3) cannot complete: it stays raw material.
  expect(recovered).toBe(2);
  const observations = calls.filter(call => call.argv[1] === "observation");
  expect(observations).toHaveLength(2);
  expect(observations[0].body).toMatchObject({ local_request_id: "local-1", origin: "local", event_seq: 1, identity_provability: "none", model_id: "m1" });
  expect(observations[1].body).toMatchObject({ local_request_id: "local-2", origin: "provider", event_seq: 2, model_id: "m2" });
  const clean = fakeMc((argv) => argv[1] === "pending" ? ok({ pending: [] }) : fail());
  expect(await recoverPendingUsage(clean.exec)).toBe(0);
  expect(clean.calls.filter(call => call.argv[1] !== "pending")).toHaveLength(0);
});
