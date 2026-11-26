package main_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func usageJSON(body map[string]any) string {
	encoded, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

// C03 CLI face: the usage command family through the real routing (the
// main.go host family list, commandUsage, and the system spine call path),
// in the frozen two-phase order, with the recovery listing between them.
func TestUsageCommandPhasesAndPending(t *testing.T) {
	spine := initSpine(t)
	env := spineEnv(spine)
	const local = "11111111-2222-3333-4444-555555555555"

	observation := func(seq any) map[string]any {
		return map[string]any{
			"local_request_id": local, "event_seq": seq, "source_version": 1,
			"origin": "local", "usage_family": "openai-chat", "model_id": "probe-model",
			"output_tokens": "3", "input_tokens": "10", "report_kind": "cumulative",
			"finality": "final", "identity_provability": "cursor",
		}
	}
	// Phase two before phase one: a domain refusal, not a use error.
	early := runMC(t, env, usageJSON(observation(1)), "usage", "observation")
	if early.code != 1 || !strings.Contains(early.stdout+early.stderr, "never persisted") {
		t.Fatalf("early observation = code %d: %s %s", early.code, early.stdout, early.stderr)
	}

	// Phase one: the raw fragment lands and takes its first number.
	raw := map[string]any{
		"usage_family": "openai-chat", "model_id": "probe-model",
		"input_tokens": "10", "output_tokens": "3",
		"report_kind": "cumulative", "finality": "final", "identity_provability": "cursor",
	}
	event := runMC(t, env, usageJSON(map[string]any{
		"local_request_id": local, "usage_family": "openai-chat", "raw": raw,
	}), "usage", "event")
	if event.code != 0 || event.json["event_seq"] != float64(1) || event.json["created"] != true {
		t.Fatalf("phase one = code %d: %s %s", event.code, event.stdout, event.stderr)
	}

	// A redelivered native-addressable event re-enters without a new
	// number. It carries its own request identity (the provider request
	// ID), so its slot starts at sequence one.
	native := map[string]any{
		"local_request_id": local, "provider_request_id": "prov-req-1", "native_event_id": "evt-1",
		"usage_family": "openai-chat", "raw": raw,
	}
	first := runMC(t, env, usageJSON(native), "usage", "event")
	if first.code != 0 || first.json["event_seq"] != float64(1) || first.json["created"] != true || first.json["request_identity"] != "prov-req-1" {
		t.Fatalf("native event = code %d: %s %s", first.code, first.stdout, first.stderr)
	}
	replay := runMC(t, env, usageJSON(native), "usage", "event")
	if replay.code != 0 || replay.json["event_seq"] != float64(1) || replay.json["created"] != false {
		t.Fatalf("native replay = code %d: %s %s", replay.code, replay.stdout, replay.stderr)
	}

	// Phase two: the observation lands and its idempotent replay re-enters.
	obsBody := usageJSON(observation(1))
	observed := runMC(t, env, obsBody, "usage", "observation")
	if observed.code != 0 || observed.json["state"] != "reported" || observed.json["created"] != true {
		t.Fatalf("observation = code %d: %s %s", observed.code, observed.stdout, observed.stderr)
	}
	id := observed.json["observation_id"].(string)
	reobs := runMC(t, env, obsBody, "usage", "observation")
	if reobs.code != 0 || reobs.json["created"] != false || reobs.json["observation_id"] != id {
		t.Fatalf("observation replay = code %d: %s %s", reobs.code, reobs.stdout, reobs.stderr)
	}

	// The recovery listing carries exactly the raw event that has no
	// observation yet: the native one in its own identity slot.
	pending := runMC(t, env, `{}`, "usage", "pending")
	if pending.code != 0 {
		t.Fatalf("pending = code %d: %s %s", pending.code, pending.stdout, pending.stderr)
	}
	list := pending.json["pending"].([]any)
	if len(list) != 1 {
		t.Fatalf("pending = %v; want the one unobserved raw event", list)
	}
	entry := list[0].(map[string]any)
	if entry["event_seq"] != float64(1) || entry["provider_request_id"] != "prov-req-1" {
		t.Fatalf("pending entry = %v; want the native event", entry)
	}

	// A malformed request is a use error, exit 2.
	bad := runMC(t, env, usageJSON(map[string]any{
		"local_request_id": local, "usage_family": "not-a-family", "raw": map[string]any{"x": 1},
	}), "usage", "event")
	if bad.code != 2 {
		t.Fatalf("bad family = code %d; want 2: %s %s", bad.code, bad.stdout, bad.stderr)
	}
}
