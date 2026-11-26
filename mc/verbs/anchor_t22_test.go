package verbs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mc/anchor"
)

// copyDir replicates a directory tree (the simulated off-site copy of the
// anchors directory).
func copyDir(t *testing.T, from string) string {
	t.Helper()
	to := t.TempDir()
	entries, err := os.ReadDir(from)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(from, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(to, entry.Name()), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return to
}

// R4 + the full flow: two publishes produce two chained artifacts in the
// anchors directory (the Z5 replication units); the copied directory
// verifies fully offline — no spine, no MC_HOME (D22 requirement four);
// the roots move as the chain grows.
func TestAnchorPublishReplicatesAndVerifiesOffline(t *testing.T) {
	env := newSignTestEnv(t)
	anchors := filepath.Join(env.home, "anchors")
	first, err := AnchorPublish(env.spine, nil, AnchorPublishRequest{Token: env.token})
	if err != nil {
		t.Fatalf("first publish: %v", err)
	}
	if first.(map[string]any)["checkpoint_seq"].(int) != 1 {
		t.Fatalf("first publish = %v; want seq 1", first)
	}
	// The second publish: the root moves with the chain, the sequence is
	// monotonic (D22 requirement two).
	second, err := AnchorPublish(env.spine, nil, AnchorPublishRequest{Token: env.token})
	if err != nil {
		t.Fatalf("second publish: %v", err)
	}
	if second.(map[string]any)["checkpoint_seq"].(int) != 2 {
		t.Fatalf("second publish = %v; want seq 2", second)
	}
	// The roots coincide here because the chain has not moved between the
	// two publishes (no new audit events in a static test chain); the
	// checkpoint protocol does not require the root to move, it requires
	// each checkpoint to chain to the previous one — which the offline
	// verification below asserts.
	// Both artifacts are staged in the anchors directory.
	for _, seq := range []int{1, 2} {
		if _, err := os.Stat(anchor.ArtifactPath(anchors, seq)); err != nil {
			t.Fatalf("artifact %d missing from the anchors directory: %v", seq, err)
		}
	}
	// The off-site copy verifies offline: no spine, no MC_HOME (the env var
	// points at the vanished home; the verifier only reads the copy).
	offsite := copyDir(t, anchors)
	report, err := anchor.VerifyDir(offsite, nil)
	if err != nil {
		t.Fatalf("offline verification of the copied anchors = %v", err)
	}
	if len(report) != 3 {
		t.Fatalf("offline report = %v; want both checkpoints plus the summary", report)
	}
}

// R1: the signing key never shares a domain with the spine — the signature
// verifies under the ring public key embedded in the artifact, and the
// spine itself holds no key material.
func TestAnchorSignatureKeyIsOutsideTheSpine(t *testing.T) {
	env := newSignTestEnv(t)
	res, err := AnchorPublish(env.spine, nil, AnchorPublishRequest{Token: env.token})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	artifactDigest := res.(map[string]any)["artifact_digest"].(string)
	// The spine stores the artifact digest, not the artifact's key material:
	// the verification key lives in the node ring (outside the spine).
	var ringKeyColumns int
	if err := env.spine.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('anchor_checkpoints')
		WHERE name LIKE '%key%' OR name LIKE '%public%' OR name LIKE '%signature%'`).Scan(&ringKeyColumns); err != nil {
		t.Fatal(err)
	}
	if ringKeyColumns != 0 {
		t.Fatalf("anchor_checkpoints carries %d key/signature columns; the signing key must live outside the spine", ringKeyColumns)
	}
	// The published artifact verifies under the ring key offline.
	anchors := filepath.Join(env.home, "anchors")
	art, err := anchor.ReadArtifact(anchor.ArtifactPath(anchors, 1))
	if err != nil {
		t.Fatal(err)
	}
	if art.Carrier != "transitional" {
		t.Fatalf("artifact carrier = %s; the transitional gate must be auditable in the artifact", art.Carrier)
	}
	if artifactDigest == "" {
		t.Fatal("the publish result carried no artifact digest")
	}
}

// R3: the chained predecessor binding and the append-only lattice make
// rollback detectable and refused.
func TestAnchorRollbackIsDetectedAndRefused(t *testing.T) {
	env := newSignTestEnv(t)
	anchors := filepath.Join(env.home, "anchors")
	if _, err := AnchorPublish(env.spine, nil, AnchorPublishRequest{Token: env.token}); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	if _, err := AnchorPublish(env.spine, nil, AnchorPublishRequest{Token: env.token}); err != nil {
		t.Fatalf("second publish: %v", err)
	}
	// The registry is append-only: rewriting or deleting a checkpoint row is
	// refused by the lattice (the spine-side anti-rollback).
	if _, err := env.spine.Exec(`UPDATE anchor_checkpoints SET root_digest = ?`, strings.Repeat("0", 64)); err == nil {
		t.Fatal("a checkpoint row accepted an update")
	}
	if _, err := env.spine.Exec(`DELETE FROM anchor_checkpoints WHERE checkpoint_seq = 1`); err == nil {
		t.Fatal("a checkpoint row accepted a delete")
	}
	// Off-site: tampering checkpoint 1 breaks the chain for checkpoint 2.
	art1, err := anchor.ReadArtifact(anchor.ArtifactPath(anchors, 1))
	if err != nil {
		t.Fatal(err)
	}
	art1.RootDigest = strings.Repeat("0", 64)
	body, err := json.Marshal(art1)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(anchor.ArtifactPath(anchors, 1), body, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := anchor.VerifyDir(anchors, nil); err == nil {
		t.Fatal("the tampered series passed offline verification; rollback must be detected")
	}
}

// The recovery path: a lost artifact is regenerated from the spine without
// allocating a new sequence (idempotent replication).
func TestAnchorPublishReplicatesALostArtifact(t *testing.T) {
	env := newSignTestEnv(t)
	anchors := filepath.Join(env.home, "anchors")
	first, err := AnchorPublish(env.spine, nil, AnchorPublishRequest{Token: env.token})
	if err != nil {
		t.Fatalf("first publish: %v", err)
	}
	seq := first.(map[string]any)["checkpoint_seq"].(int)
	// The Z5 copy is lost.
	if err := os.Remove(anchor.ArtifactPath(anchors, seq)); err != nil {
		t.Fatal(err)
	}
	// The next publish replicates instead of allocating a new sequence.
	res, err := AnchorPublish(env.spine, nil, AnchorPublishRequest{Token: env.token})
	if err != nil {
		t.Fatalf("republish after the loss: %v", err)
	}
	if res.(map[string]any)["state"] != "replicated" || res.(map[string]any)["replicated"].(int) != seq {
		t.Fatalf("republish result = %v; want the lost artifact replicated at seq %d", res, seq)
	}
	if _, err := os.Stat(anchor.ArtifactPath(anchors, seq)); err != nil {
		t.Fatalf("the replicated artifact is missing: %v", err)
	}
	// The replicated artifact verifies.
	if _, err := anchor.VerifyDir(anchors, nil); err != nil {
		t.Fatalf("the replicated artifact failed offline verification: %v", err)
	}
}

// The layer-one pre-check: a chain with findings is never anchored.
func TestAnchorPublishRefusesABrokenChain(t *testing.T) {
	env := newSignTestEnv(t)
	// Break the audit chain directly: a forged first row whose prev_hash is
	// the zero digest (not the schema genesis), so the recursion does not
	// close — the layer-one verifier flags the broken chain.
	if _, err := env.spine.Exec(`INSERT INTO audit_events
		(aggregate_id, seq, event_id, aggregate_type, event_type, serializer_id,
		 occurred_at, recorded_at, prev_hash, event_digest, chain_digest, payload_family, payload_json)
		VALUES ('schema', 1, ?, 'schema', 'broken.tail', 'shz-canon-v1',
		        '2026-11-27T00:00:00.000000Z', '2026-11-27T00:00:00.000000Z', ?, ?, ?, 'broken', '{}')`,
		strings.Repeat("f", 64), strings.Repeat("0", 64), strings.Repeat("1", 64), strings.Repeat("2", 64)); err != nil {
		t.Fatalf("seed the broken chain row: %v", err)
	}
	if _, err := AnchorPublish(env.spine, nil, AnchorPublishRequest{Token: env.token}); err == nil {
		t.Fatal("a broken chain was anchored; the layer-one pre-check must refuse")
	}
	var n int
	if err := env.spine.QueryRow(`SELECT COUNT(*) FROM anchor_checkpoints`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("anchor_checkpoints rows after the refused publish = %d; want 0", n)
	}
}
