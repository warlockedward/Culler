package substrate

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// W1: the shz-canon-v1 Go serializer against the registered vector suite.
// The vectors are the frozen cross-implementation contract: any divergence
// in canonical bytes or digest is a FAIL, never a tolerance.
func TestCanonVectors(t *testing.T) {
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
	if len(suite.Vectors) == 0 {
		t.Fatal("vector suite is empty")
	}
	for _, v := range suite.Vectors {
		v := v
		t.Run(v.ID, func(t *testing.T) {
			canonical, err := EncodeCanonicalEvent(v.Input)
			if v.ExpectError != "" {
				if err == nil {
					t.Fatalf("expect a NOT_CONFIRMED refusal, got canonical bytes %s", hex.EncodeToString(canonical))
				}
				if !errors.Is(err, ErrNotConfirmed) {
					t.Fatalf("refusal is %v; want ErrNotConfirmed", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("want canonical bytes, got %v", err)
			}
			if got := hex.EncodeToString(canonical); got != v.ExpectedCanonicalHex {
				t.Fatalf("canonical bytes diverge:\n got %s\nwant %s", got, v.ExpectedCanonicalHex)
			}
			if got := EventDigest(canonical); got != v.ExpectedEventDigest {
				t.Fatalf("event digest diverges:\n got %s\nwant %s", got, v.ExpectedEventDigest)
			}
		})
	}
}

// W2: the shz-merkle-v1 Go root against the registered root vector suite,
// including the explicit-refusal error cases.
func TestMerkleVectors(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "design", "vectors", "root_vectors.json"))
	if err != nil {
		t.Fatalf("read root vector suite: %v", err)
	}
	var suite struct {
		AlgorithmID string `json:"algorithm_id"`
		Cases       []struct {
			ID           string  `json:"id"`
			Entries      [][]any `json:"entries"`
			ExpectedRoot string  `json:"expected_root"`
			ExpectError  string  `json:"expect_error"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &suite); err != nil {
		t.Fatalf("parse root vector suite: %v", err)
	}
	if len(suite.Cases) == 0 {
		t.Fatal("root vector suite is empty")
	}
	for _, c := range suite.Cases {
		c := c
		t.Run(c.ID, func(t *testing.T) {
			heads := make([]ChainHead, 0, len(c.Entries))
			for _, e := range c.Entries {
				id, _ := e[0].(string)
				seq := int(e[1].(float64))
				digest, _ := e[2].(string)
				heads = append(heads, ChainHead{AggregateID: id, SeqMax: seq, ChainDigest: digest})
			}
			algorithmID, leafCount, rootHex, err := ComputeRoot(heads)
			if c.ExpectError != "" {
				if err == nil {
					t.Fatalf("expect a refusal, got root %s", rootHex)
				}
				if !strings.Contains(err.Error(), c.ExpectError) {
					t.Fatalf("refusal message %q does not carry %q", err.Error(), c.ExpectError)
				}
				return
			}
			if err != nil {
				t.Fatalf("want a root, got %v", err)
			}
			if algorithmID != suite.AlgorithmID {
				t.Fatalf("algorithm id = %s; want %s", algorithmID, suite.AlgorithmID)
			}
			if leafCount != len(c.Entries) {
				t.Fatalf("leaf count = %d; want %d", leafCount, len(c.Entries))
			}
			if rootHex != c.ExpectedRoot {
				t.Fatalf("root diverges:\n got %s\nwant %s", rootHex, c.ExpectedRoot)
			}
		})
	}
}
