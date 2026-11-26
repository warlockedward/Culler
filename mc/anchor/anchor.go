// Package anchor is the anchoring protocol core (D07, D22): the shz-
// anchor-v1 canonical forms, the checkpoint artifact builder, and the
// offline verifier. The verifier is pure-file: it opens no spine and reads
// no deployment home, so a copied anchors directory verifies anywhere
// (D22 requirement four).
package anchor

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"mc/signnode"
)

// Format identifies the artifact serialization.
const Format = "shz-anchor-checkpoint-v1"

// genesisSeed is the domain-separated zero checkpoint: the first
// checkpoint's prev digest is its hash, so the chain is closed from the
// origin and no checkpoint can silently claim to be first.
const genesisSeed = "shz-anchor-genesis-v1"

// ErrVerify marks an offline verification failure: the artifact or the
// chain did not hold.
var ErrVerify = errors.New("anchor verification failed")

// Head is one aggregate's chain head in the anchored head set (DC §3.4
// verifier output shape).
type Head struct {
	AggregateID string `json:"aggregate_id"`
	SeqMax      int    `json:"seq_max"`
	ChainDigest string `json:"chain_digest"`
}

// Artifact is the Z5 replication unit: the signed Merkle root, the head
// set it was computed from, and the verification metadata. It carries no
// signed content, no project names, and no internal events (D22 sigstore
// discipline) — only what an offline verifier needs.
type Artifact struct {
	Format         string `json:"format"`
	CheckpointSeq  int    `json:"checkpoint_seq"`
	PrevDigest     string `json:"prev_digest"`
	RootDigest     string `json:"root_digest"`
	AlgorithmID    string `json:"algorithm_id"`
	HeadsDigest    string `json:"heads_digest"`
	LeafCount      int    `json:"leaf_count"`
	Heads          []Head `json:"heads"`
	PublishedBy    string `json:"published_by"`
	Carrier        string `json:"carrier"`
	SignRequestID  string `json:"sign_request_id"`
	RequestDigest  string `json:"request_digest"`
	ResultDigest   string `json:"result_digest"`
	KeyID          string `json:"key_id"`
	PublicKeyHex   string `json:"public_key_hex"`
	SignatureHex   string `json:"signature_hex"`
	AttestationHex string `json:"attestation_hex"`
}

// HeadsDigest renders the head set in the canonical order (aggregate id
// ascending, DC §3.4) and digests it. The rendering is also what the
// artifact stores, so the offline verifier re-derives the exact input.
func HeadsDigest(heads []Head) (string, string, error) {
	sorted := make([]Head, len(heads))
	copy(sorted, heads)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].AggregateID < sorted[j].AggregateID })
	lines := make([]string, len(sorted))
	for i, h := range sorted {
		lines[i] = h.AggregateID + "|" + fmt.Sprintf("%d", h.SeqMax) + "|" + h.ChainDigest
	}
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:]), strings.Join(lines, "\n"), nil
}

// CoreCanonical is the shz-anchor-checkpoint-v1 byte string the node signs:
// versioned, frozen field order, single authority (publisher and offline
// verifier rebuild the same bytes).
func CoreCanonical(seq int, prevDigest, rootDigest, algorithmID, headsDigest string) []byte {
	return []byte("shz-anchor-checkpoint-v1\n" + strings.Join([]string{
		fmt.Sprintf("%d", seq), prevDigest, rootDigest, algorithmID, headsDigest,
	}, "\n"))
}

// GenesisPrev is the zero checkpoint's prev digest.
func GenesisPrev() string {
	sum := sha256.Sum256([]byte(genesisSeed))
	return hex.EncodeToString(sum[:])
}

// CoreDigest digests a checkpoint core the way the chain binds it.
func CoreDigest(seq int, prevDigest, rootDigest, algorithmID, headsDigest string) string {
	sum := sha256.Sum256(CoreCanonical(seq, prevDigest, rootDigest, algorithmID, headsDigest))
	return hex.EncodeToString(sum[:])
}

// Build assembles the artifact from a verified receipt and the checkpoint
// core, embedding the public key so the artifact verifies offline without
// any deployment state.
func Build(seq int, prevDigest, rootDigest, algorithmID, headsDigest string, leafCount int, heads []Head,
	publishedBy, carrier, signRequestID string, receipt struct {
		RequestDigest, ResultDigest, KeyID, SignatureHex, AttestationHex, PublicKeyHex string
	}) Artifact {
	return Artifact{
		Format: Format, CheckpointSeq: seq, PrevDigest: prevDigest,
		RootDigest: rootDigest, AlgorithmID: algorithmID, HeadsDigest: headsDigest,
		LeafCount: leafCount, Heads: heads, PublishedBy: publishedBy, Carrier: carrier,
		SignRequestID: signRequestID, RequestDigest: receipt.RequestDigest,
		ResultDigest: receipt.ResultDigest, KeyID: receipt.KeyID,
		PublicKeyHex: receipt.PublicKeyHex, SignatureHex: receipt.SignatureHex,
		AttestationHex: receipt.AttestationHex,
	}
}

// ArtifactBytes renders the artifact as the JSON file body.
func ArtifactBytes(a Artifact) ([]byte, error) {
	return json.MarshalIndent(a, "", "  ")
}

// ReadArtifact loads one artifact file.
func ReadArtifact(path string) (Artifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Artifact{}, fmt.Errorf("read the anchor artifact: %w", err)
	}
	var a Artifact
	if err := json.Unmarshal(data, &a); err != nil {
		return Artifact{}, fmt.Errorf("%w: the anchor artifact is malformed: %v", ErrVerify, err)
	}
	return a, nil
}

// verifyArtifact checks one artifact in isolation: the format, the head set
// binding, the signature under the embedded public key, the attestation
// over the receipt tuple, and the root recomputation from the head set.
// recomputeRoot, when non-nil, re-derives the Merkle root from the head
// set (pure function injected by the caller to avoid an import cycle).
func verifyArtifact(a Artifact, recomputeRoot func([]Head) (string, int, error)) error {
	if a.Format != Format {
		return fmt.Errorf("%w: the artifact format is %q; want %q", ErrVerify, a.Format, Format)
	}
	if a.CheckpointSeq < 1 {
		return fmt.Errorf("%w: the checkpoint sequence %d is not positive", ErrVerify, a.CheckpointSeq)
	}
	if len(a.Heads) == 0 && a.LeafCount != 0 {
		return fmt.Errorf("%w: the empty head set claims %d leaves", ErrVerify, a.LeafCount)
	}
	rendered, err := headsRender(a.Heads)
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(rendered))
	if hex.EncodeToString(sum[:]) != a.HeadsDigest {
		return fmt.Errorf("%w: the head set digest does not match the artifact's head set", ErrVerify)
	}
	public, err := hex.DecodeString(a.PublicKeyHex)
	if err != nil || len(public) != ed25519.PublicKeySize {
		return fmt.Errorf("%w: the artifact public key is malformed", ErrVerify)
	}
	signature, err := hex.DecodeString(a.SignatureHex)
	if err != nil {
		return fmt.Errorf("%w: the artifact signature is malformed", ErrVerify)
	}
	core := CoreCanonical(a.CheckpointSeq, a.PrevDigest, a.RootDigest, a.AlgorithmID, a.HeadsDigest)
	if !ed25519.Verify(ed25519.PublicKey(public), core, signature) {
		return fmt.Errorf("%w: the checkpoint signature failed verification under the artifact's public key", ErrVerify)
	}
	attestation, err := hex.DecodeString(a.AttestationHex)
	if err != nil {
		return fmt.Errorf("%w: the artifact attestation is malformed", ErrVerify)
	}
	if !ed25519.Verify(ed25519.PublicKey(public), attestationCanonical(a), attestation) {
		return fmt.Errorf("%w: the receipt attestation failed verification", ErrVerify)
	}
	if recomputeRoot != nil {
		root, leafCount, err := recomputeRoot(a.Heads)
		if err != nil {
			return fmt.Errorf("%w: the head set does not recompute: %v", ErrVerify, err)
		}
		if root != a.RootDigest || leafCount != a.LeafCount {
			return fmt.Errorf("%w: the root does not match the head set recomputation", ErrVerify)
		}
	}
	return nil
}

func headsRender(heads []Head) (string, error) {
	sorted := make([]Head, len(heads))
	copy(sorted, heads)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].AggregateID < sorted[j].AggregateID })
	for i := 1; i < len(sorted); i++ {
		if sorted[i].AggregateID == sorted[i-1].AggregateID {
			return "", fmt.Errorf("%w: the head set repeats aggregate %q", ErrVerify, sorted[i].AggregateID)
		}
	}
	lines := make([]string, len(sorted))
	for i, h := range sorted {
		lines[i] = h.AggregateID + "|" + fmt.Sprintf("%d", h.SeqMax) + "|" + h.ChainDigest
	}
	return strings.Join(lines, "\n"), nil
}

// attestationCanonical delegates to the D06 single authority: the receipt
// tuple's canonical form lives in one place, so the offline verifier can
// never drift from what the node attested.
func attestationCanonical(a Artifact) []byte {
	return signnode.AttestationCanonical(a.SignRequestID, a.RequestDigest, a.ResultDigest, a.KeyID)
}

// VerifyDir verifies a copied anchors directory offline: every artifact
// must hold in isolation, and the series must be a gapless monotonic chain
// binding each checkpoint to its predecessor's signed core (D22
// requirements two and three). recomputeRoot, when non-nil, additionally
// re-derives each root from its head set.
func VerifyDir(dir string, recomputeRoot func([]Head) (string, int, error)) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read the anchors directory: %w", err)
	}
	var report []string
	previous := GenesisPrev()
	expectSeq := 1
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "checkpoint-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		a, err := ReadArtifact(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		if a.CheckpointSeq != expectSeq {
			return nil, fmt.Errorf("%w: checkpoint %s carries seq %d; want %d (the series must be gapless and monotonic)",
				ErrVerify, entry.Name(), a.CheckpointSeq, expectSeq)
		}
		if a.PrevDigest != previous {
			return nil, fmt.Errorf("%w: checkpoint %d does not chain to its predecessor (anti-rollback check failed)",
				ErrVerify, a.CheckpointSeq)
		}
		if err := verifyArtifact(a, recomputeRoot); err != nil {
			return nil, fmt.Errorf("checkpoint %d: %w", a.CheckpointSeq, err)
		}
		report = append(report, fmt.Sprintf("checkpoint %d ok (root %s, algorithm %s)",
			a.CheckpointSeq, shortDigest(a.RootDigest), a.AlgorithmID))
		previous = CoreDigest(a.CheckpointSeq, a.PrevDigest, a.RootDigest, a.AlgorithmID, a.HeadsDigest)
		expectSeq++
	}
	if expectSeq == 1 {
		return nil, fmt.Errorf("%w: the anchors directory holds no checkpoints", ErrVerify)
	}
	report = append(report, fmt.Sprintf("verified through checkpoint %d", expectSeq-1))
	return report, nil
}

// WriteArtifact stages one artifact file into the anchors directory.
func WriteArtifact(dir string, a Artifact) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create the anchors directory: %w", err)
	}
	body, err := ArtifactBytes(a)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("checkpoint-%d.json", a.CheckpointSeq))
	if err := os.WriteFile(path, append(body, '\n'), 0o600); err != nil {
		return "", fmt.Errorf("write the anchor artifact: %w", err)
	}
	return path, nil
}

// ArtifactPath is the Z5 location of one checkpoint's artifact.
func ArtifactPath(dir string, seq int) string {
	return filepath.Join(dir, fmt.Sprintf("checkpoint-%d.json", seq))
}

func shortDigest(d string) string {
	if len(d) > 12 {
		return d[:12] + "…"
	}
	return d
}
