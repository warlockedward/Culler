package anchor

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testKey generates one ed25519 key and its hex forms.
func testKey(t *testing.T) (publicHex string, priv ed25519.PrivateKey) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(priv.Public().(ed25519.PublicKey)), priv
}

func sampleHeads() []Head {
	return []Head{
		{AggregateID: "usage", SeqMax: 4, ChainDigest: repeatHex('a')},
		{AggregateID: "activity", SeqMax: 9, ChainDigest: repeatHex('b')},
		{AggregateID: "schema", SeqMax: 11, ChainDigest: repeatHex('c')},
	}
}

func repeatHex(b byte) string {
	out := make([]byte, 64)
	for i := range out {
		out[i] = b
	}
	return string(out)
}

func signCore(t *testing.T, priv ed25519.PrivateKey, art Artifact) []byte {
	t.Helper()
	return ed25519.Sign(priv, CoreCanonical(art.CheckpointSeq, art.PrevDigest, art.RootDigest, art.AlgorithmID, art.HeadsDigest))
}

// buildTestArtifact builds one fully self-consistent artifact signed under
// the given key.
func buildTestArtifact(t *testing.T, seq int, prev string, publicHex string, priv ed25519.PrivateKey, heads []Head, rootDigest, algorithmID string) Artifact {
	headsDigest, _, err := HeadsDigest(heads)
	if err != nil {
		t.Fatal(err)
	}
	art := Artifact{
		Format: Format, CheckpointSeq: seq, PrevDigest: prev,
		RootDigest: rootDigest, AlgorithmID: algorithmID, HeadsDigest: headsDigest,
		LeafCount: len(heads), Heads: heads, PublishedBy: "p0000000000000000000000000000001",
		Carrier: "transitional", SignRequestID: "s0000000000000000000000000000001",
		RequestDigest: repeatHex('1'), ResultDigest: repeatHex('2'), KeyID: "k-test",
		PublicKeyHex: publicHex,
	}
	art.SignatureHex = hex.EncodeToString(signCore(t, priv, art))
	attestation := ed25519.Sign(priv, attestationCanonicalOf(art))
	art.AttestationHex = hex.EncodeToString(attestation)
	return art
}

func attestationCanonicalOf(a Artifact) []byte {
	return []byte("shz-sign-attestation-v1\n" + a.SignRequestID + "\n" + a.RequestDigest + "\n" + a.ResultDigest + "\n" + a.KeyID)
}

func writeArt(t *testing.T, dir string, art Artifact) {
	t.Helper()
	if _, err := WriteArtifact(dir, art); err != nil {
		t.Fatal(err)
	}
}

// The chain verifies from genesis: two chained checkpoints pass offline.
func TestVerifyDirAcceptsAGenesisChainedSeries(t *testing.T) {
	dir := t.TempDir()
	publicHex, priv := testKey(t)
	heads := sampleHeads()
	art1 := buildTestArtifact(t, 1, GenesisPrev(), publicHex, priv, heads, repeatHex('r'), "shz-merkle-v1")
	writeArt(t, dir, art1)
	prev := CoreDigest(1, art1.PrevDigest, art1.RootDigest, art1.AlgorithmID, art1.HeadsDigest)
	art2 := buildTestArtifact(t, 2, prev, publicHex, priv, heads, repeatHex('s'), "shz-merkle-v1")
	writeArt(t, dir, art2)

	report, err := VerifyDir(dir, nil)
	if err != nil {
		t.Fatalf("VerifyDir = %v; want the series to hold", err)
	}
	if len(report) != 3 {
		t.Fatalf("report lines = %d; want two checkpoints plus the summary", len(report))
	}
}

// R3: tampering any core field of a past checkpoint breaks the chain for
// every later checkpoint — rollback is detected offline.
func TestVerifyDirDetectsRollbackTampering(t *testing.T) {
	dir := t.TempDir()
	publicHex, priv := testKey(t)
	heads := sampleHeads()
	art1 := buildTestArtifact(t, 1, GenesisPrev(), publicHex, priv, heads, repeatHex('r'), "shz-merkle-v1")
	writeArt(t, dir, art1)
	prev := CoreDigest(1, art1.PrevDigest, art1.RootDigest, art1.AlgorithmID, art1.HeadsDigest)
	art2 := buildTestArtifact(t, 2, prev, publicHex, priv, heads, repeatHex('s'), "shz-merkle-v1")
	writeArt(t, dir, art2)

	// Rewrite checkpoint 1 with a different root (rollback attempt), keeping
	// the chain pointers intact: the signature no longer matches.
	tampered := art1
	tampered.RootDigest = repeatHex('0')
	writeArt(t, dir, tampered)
	if _, err := VerifyDir(dir, nil); err == nil {
		t.Fatal("the rollback tamper passed verification; the chain must detect it")
	}
}

// R2: a sequence gap breaks the series.
func TestVerifyDirRejectsSequenceGaps(t *testing.T) {
	dir := t.TempDir()
	publicHex, priv := testKey(t)
	heads := sampleHeads()
	art1 := buildTestArtifact(t, 1, GenesisPrev(), publicHex, priv, heads, repeatHex('r'), "shz-merkle-v1")
	writeArt(t, dir, art1)
	prev := CoreDigest(1, art1.PrevDigest, art1.RootDigest, art1.AlgorithmID, art1.HeadsDigest)
	art3 := buildTestArtifact(t, 3, prev, publicHex, priv, heads, repeatHex('s'), "shz-merkle-v1")
	writeArt(t, dir, art3)
	if _, err := VerifyDir(dir, nil); err == nil {
		t.Fatal("a gapped series passed verification; the sequence must be gapless")
	}
}

// A forged signature is refused even when the chain pointers are consistent.
func TestVerifyDirRejectsForgedSignature(t *testing.T) {
	dir := t.TempDir()
	publicHex, priv := testKey(t)
	_, otherPriv := testKey(t)
	heads := sampleHeads()
	art := buildTestArtifact(t, 1, GenesisPrev(), publicHex, priv, heads, repeatHex('r'), "shz-merkle-v1")
	// Re-sign under a different key while keeping the ring key in the
	// artifact: the signature check must refuse.
	art.SignatureHex = hex.EncodeToString(signCore(t, otherPriv, art))
	writeArt(t, dir, art)
	if _, err := VerifyDir(dir, nil); err == nil {
		t.Fatal("a forged signature passed the offline verification")
	}
}

// The root recomputation hook: a head set that does not re-derive the
// published root is refused offline.
func TestVerifyDirRecomputesTheRootFromTheHeadSet(t *testing.T) {
	dir := t.TempDir()
	publicHex, priv := testKey(t)
	heads := sampleHeads()
	art := buildTestArtifact(t, 1, GenesisPrev(), publicHex, priv, heads, repeatHex('r'), "shz-merkle-v1")
	writeArt(t, dir, art)
	// The recomputation returns a root that disagrees with the artifact:
	// refused.
	fake := func([]Head) (string, int, error) { return repeatHex('0'), len(heads), nil }
	if _, err := VerifyDir(dir, fake); err == nil {
		t.Fatal("a root that does not match the head set recomputation passed")
	}
	// With a recomputation that agrees (any stable function over the same
	// input), the artifact holds.
	honest := func([]Head) (string, int, error) { return art.RootDigest, art.LeafCount, nil }
	if _, err := VerifyDir(dir, honest); err != nil {
		t.Fatalf("VerifyDir with an agreeing recomputation = %v; want it to hold", err)
	}
}

// The artifact carries verification metadata only: no project names, no
// internal event payloads (D22 sigstore discipline).
func TestArtifactCarriesVerificationMetadataOnly(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "anchors")
	publicHex, priv := testKey(t)
	heads := sampleHeads()
	art := buildTestArtifact(t, 1, GenesisPrev(), publicHex, priv, heads, repeatHex('r'), "shz-merkle-v1")
	writeArt(t, dir, art)
	data, err := os.ReadFile(filepath.Join(dir, "checkpoint-1.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"project", "task", "payload", "run_id", "detail"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("the artifact carries the internal vocabulary %q; it must publish the root and metadata only", forbidden)
		}
	}
}

// The genesis constant separates anchor chains from anything else.
func TestGenesisPrevIsDomainSeparated(t *testing.T) {
	if GenesisPrev() == repeatHex('0') {
		t.Fatal("the genesis prev digest equals the zero digest")
	}
	sum := sha256.Sum256([]byte(genesisSeed))
	if hex.EncodeToString(sum[:]) != GenesisPrev() {
		t.Fatal("the genesis prev is not the documented domain constant hash")
	}
}
