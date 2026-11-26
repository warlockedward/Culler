package substrate

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

// U15: the C03 producer face against the B03 frozen usage_observation
// vectors. Two claims, separately: the serializer still renders each
// frozen vector byte-identically, and this ticket's payload builder
// reproduces the full-payload vectors' field maps exactly — the decimal
// lexical forms and the absent/present choices survive the round trip.
func TestUsageObservationPayloadVectors(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "design", "vectors", "vectors.json"))
	if err != nil {
		t.Fatalf("read vector suite: %v", err)
	}
	var suite struct {
		SerializerID string `json:"serializer_id"`
		Vectors      []struct {
			ID                   string         `json:"id"`
			Input                map[string]any `json:"input"`
			ExpectedCanonicalHex string         `json:"expected_canonical_hex"`
			ExpectedEventDigest  string         `json:"expected_event_digest"`
			ExpectError          string         `json:"expect_error"`
		} `json:"vectors"`
	}
	if err := json.Unmarshal(data, &suite); err != nil {
		t.Fatalf("parse vector suite: %v", err)
	}
	if suite.SerializerID != canonicalSerializerID {
		t.Fatalf("vector suite targets serializer %s; this implementation is %s", suite.SerializerID, canonicalSerializerID)
	}
	builderHits := 0
	for _, v := range suite.Vectors {
		v := v
		family, _ := v.Input["payload_family"].(string)
		if family != "usage_observation" {
			continue
		}
		t.Run(v.ID, func(t *testing.T) {
			if v.ExpectError != "" {
				return
			}
			canonical, err := EncodeCanonicalEvent(v.Input)
			if err != nil {
				t.Fatalf("want canonical bytes, got %v", err)
			}
			if got := hex.EncodeToString(canonical); got != v.ExpectedCanonicalHex {
				t.Fatalf("canonical bytes diverge:\n got %s\nwant %s", got, v.ExpectedCanonicalHex)
			}
			if got := EventDigest(canonical); got != v.ExpectedEventDigest {
				t.Fatalf("event digest diverges:\n got %s\nwant %s", got, v.ExpectedEventDigest)
			}
			payload, _ := v.Input["payload"].(map[string]any)
			if len(payload) != len(payloadSchemas["usage_observation"]) {
				// An optional-field-absent vector: the serializer face is
				// proven above; the producer face always fills the full
				// table, so the builder comparison does not apply.
				return
			}
			in := UsageObservationInput{}
			in.ModelID, _ = payload["model_id"].(string)
			in.OutputTokens, _ = payload["output_tokens"].(string)
			in.InputTokens, _ = payload["input_tokens"].(string)
			in.LocalRequestID, _ = payload["local_request_id"].(string)
			in.ProviderRequestID, _ = payload["provider_request_id"].(string)
			if in.ProviderRequestID == "unverifiable" {
				// The frozen vectors register the absent provider ID in
				// its explicit form; the wire input carries it empty.
				in.ProviderRequestID = ""
			}
			in.Origin, _ = payload["origin"].(string)
			in.ReportKind, _ = payload["report_kind"].(string)
			in.Finality, _ = payload["finality"].(string)
			in.IdentityProvability, _ = payload["identity_provability"].(string)
			in.UsageFamily, _ = payload["usage_family"].(string)
			seq, err := strconv.ParseInt(payload["event_seq"].(string), 10, 64)
			if err != nil {
				t.Fatalf("vector event_seq %q is not an integer: %v", payload["event_seq"], err)
			}
			version, err := strconv.ParseInt(payload["source_version"].(string), 10, 64)
			if err != nil {
				t.Fatalf("vector source_version %q is not an integer: %v", payload["source_version"], err)
			}
			in.EventSeq = seq
			in.SourceVersion = version
			got := usageObservationPayload(in)
			if !reflect.DeepEqual(got, payload) {
				t.Fatalf("payload builder diverges from the frozen vector:\n got %v\nwant %v", got, payload)
			}
			builderHits++
		})
	}
	if builderHits == 0 {
		t.Fatal("no full-payload usage_observation vector exercised the producer face")
	}
}
