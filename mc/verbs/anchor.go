package verbs

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"mc/anchor"
	"mc/identity"
	"mc/signnode"
	"mc/substrate"
)

// The anchoring flow (D07, D22): the layer-one verifier must be clean, the
// head set is computed and committed as a checkpoint bound to the previous
// one, the checkpoint core is signed through the D06 signing path (the key
// lives in the node ring, never in the spine — D22 requirement one), and
// the artifact is staged for Z5 replication. The artifact carries the root
// and verification metadata only (D22 sigstore discipline).
//
// The transitional carrier records itself in every artifact: production
// anchoring under IG §5.4 (carrier B) is an owner infrastructure decision,
// and the key id in each artifact is what makes that gate auditable.

// AnchorPublishRequest publishes one anchoring checkpoint: the layer-one
// verifier runs first (a broken chain is never anchored), then the head
// set is computed, signed through the D06 path, and replicated to the
// anchors directory.
type AnchorPublishRequest struct {
	Token string `json:"token"`
}

// AnchorPublish performs one publish pass. If the latest committed
// checkpoint's artifact is missing from the anchors directory, the pass
// regenerates it from the spine (idempotent replication) instead of
// allocating a new sequence.
func AnchorPublish(db *sql.DB, id *RunIdentity, a AnchorPublishRequest) (any, error) {
	if err := RequireHostScope(id, "anchor publication"); err != nil {
		return nil, err
	}
	home, err := signNodeHome()
	if err != nil {
		return nil, err
	}
	anchorsDir := filepath.Join(home, "anchors")

	// Layer-one pre-check: anchor only a chain the verifier holds clean.
	report, err := substrate.VerifyAuditChain(db, "")
	if err != nil {
		return nil, classify(err)
	}
	if len(report.Errors) != 0 {
		return nil, Domainf("The audit chain holds %d findings; anchor only a clean chain.", len(report.Errors))
	}
	heads := make([]anchor.Head, len(report.Heads))
	for i, h := range report.Heads {
		heads[i] = anchor.Head{AggregateID: h.AggregateID, SeqMax: h.SeqMax, ChainDigest: h.ChainDigest}
	}
	headsDigest, _, err := anchor.HeadsDigest(heads)
	if err != nil {
		return nil, classify(err)
	}
	headsJSON, err := json.Marshal(heads)
	if err != nil {
		return nil, classify(err)
	}
	headsJSONText := string(headsJSON)

	actor, err := sessionActor(context.Background(), db, a.Token)
	if err != nil {
		return nil, classify(err)
	}

	// Recovery branch: the latest committed checkpoint's artifact must be
	// present; regenerate it from the spine when the Z5 copy was lost.
	var latest struct {
		seq           int
		prevDigest    string
		rootDigest    string
		algorithmID   string
		headsDigest   string
		leafCount     int
		headsJSON     string
		publishedBy   string
		signRequestID string
	}
	rowErr := db.QueryRow(`SELECT checkpoint_seq, prev_digest, root_digest, algorithm_id, heads_digest,
		leaf_count, heads_json, published_by, sign_request_id FROM anchor_checkpoints
		ORDER BY checkpoint_seq DESC LIMIT 1`).
		Scan(&latest.seq, &latest.prevDigest, &latest.rootDigest, &latest.algorithmID,
			&latest.headsDigest, &latest.leafCount, &latest.headsJSON, &latest.publishedBy, &latest.signRequestID)
	switch {
	case rowErr == nil:
		path := anchor.ArtifactPath(anchorsDir, latest.seq)
		if _, statErr := os.Stat(path); errors.Is(statErr, fs.ErrNotExist) {
			if err := regenerateAnchorArtifact(db, home, anchorsDir, latest.seq, latest.prevDigest,
				latest.rootDigest, latest.algorithmID, latest.headsDigest, latest.leafCount,
				latest.headsJSON, latest.publishedBy, latest.signRequestID); err != nil {
				return nil, classify(err)
			}
			return map[string]any{"replicated": latest.seq, "state": "replicated"}, nil
		}
	case errors.Is(rowErr, sql.ErrNoRows):
		// First checkpoint: prev binds to the genesis constant.
	default:
		return nil, classify(rowErr)
	}

	seq := 1
	prevDigest := anchor.GenesisPrev()
	if rowErr == nil {
		seq = latest.seq + 1
		prevDigest = anchor.CoreDigest(latest.seq, latest.prevDigest, latest.rootDigest,
			latest.algorithmID, latest.headsDigest)
	}
	rootAlgorithmID, leafCount, rootDigest, rootErr := computeAnchorRoot(heads)
	if rootErr != nil {
		return nil, classify(rootErr)
	}
	coreDigest := anchor.CoreDigest(seq, prevDigest, rootDigest, rootAlgorithmID, headsDigest)

	// Sign the checkpoint core through the D06 path: the capability gate,
	// the node signing, and the receipt verification all run there.
	created, err := SignRequestCreate(db, nil, SignRequestCreateRequest{
		Token: a.Token, RootDigest: coreDigest, AlgorithmID: rootAlgorithmID,
		SerializerID: "shz-anchor-v1", Scope: "anchor", TTLHours: 24,
	})
	if err != nil {
		return nil, classify(err)
	}
	requestID := created.(map[string]any)["request_id"].(string)
	if _, err := SignRequestExecute(db, nil, SignRequestExecuteRequest{
		Token: a.Token, RequestID: requestID,
		PayloadHex: hex.EncodeToString(anchor.CoreCanonical(
			seq, prevDigest, rootDigest, rootAlgorithmID, headsDigest)),
	}); err != nil {
		return nil, classify(err)
	}
	// The signed request row now holds the verified receipt fields.
	var receipt struct {
		SignatureHex *string
		Attestation  *string
		KeyID        *string
		ResultDigest *string
	}
	if err := db.QueryRow(`SELECT signature_hex, receipt_attestation, key_id, result_digest
		FROM sign_requests WHERE request_id = ?`, requestID).
		Scan(&receipt.SignatureHex, &receipt.Attestation, &receipt.KeyID, &receipt.ResultDigest); err != nil {
		return nil, classify(err)
	}
	dir, err := signnode.NodeDir(home)
	if err != nil {
		return nil, classify(err)
	}
	keyID := deref(receipt.KeyID)
	public, err := signnode.PublicKey(dir, keyID)
	if err != nil {
		return nil, Domainf("the anchoring key is unknown to the ring: %v", err)
	}
	artifact := anchor.Build(seq, prevDigest, rootDigest, rootAlgorithmID, headsDigest, leafCount,
		heads, actor, "transitional", requestID, signReceiptFields{
			RequestDigest:  created.(map[string]any)["request_digest"].(string),
			ResultDigest:   deref(receipt.ResultDigest),
			KeyID:          keyID,
			SignatureHex:   deref(receipt.SignatureHex),
			AttestationHex: deref(receipt.Attestation),
			PublicKeyHex:   hex.EncodeToString(public),
		})
	body, err := anchor.ArtifactBytes(artifact)
	if err != nil {
		return nil, classify(err)
	}
	artifactDigest := digestBytes(body)

	committed := false
	if err := inTx(db, func(ctx context.Context, q Q) error {
		if err := identity.BindTrustedContext(ctx, q, actor, false); err != nil {
			return err
		}
		res, err := q.ExecContext(ctx, `INSERT INTO anchor_checkpoints
			(checkpoint_seq, prev_digest, root_digest, algorithm_id, heads_digest, leaf_count,
			 heads_json, published_by, sign_request_id, artifact_digest)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			seq, prevDigest, rootDigest, rootAlgorithmID, headsDigest, leafCount,
			headsJSONText, actor, requestID, artifactDigest)
		if err != nil {
			return Domainf("The anchor checkpoint could not be recorded (sequence moved concurrently).")
		}
		if n, err := res.RowsAffected(); err == nil && n == 1 {
			committed = true
		}
		return recordActivity(ctx, q, actor, "anchor.published", requestID,
			fmt.Sprintf("Anchoring checkpoint %d published (root %s, algorithm %s).", seq, shortRoot(rootDigest), rootAlgorithmID))
	}); err != nil {
		return nil, classify(err)
	}
	if !committed {
		return nil, Domainf("The anchor checkpoint state moved concurrently; publish again.")
	}
	path, err := anchor.WriteArtifact(anchorsDir, artifact)
	if err != nil {
		// The checkpoint is committed; the Z5 copy is recoverable through
		// the publish recovery path.
		return nil, Usagef("The checkpoint %d is committed but its artifact staging failed: %v. Publish again to replicate it.", seq, err)
	}
	return map[string]any{"checkpoint_seq": seq, "state": "published",
		"root_digest": rootDigest, "algorithm_id": rootAlgorithmID,
		"leaf_count": leafCount, "artifact_path": path, "artifact_digest": artifactDigest}, nil
}

// computeAnchorRoot re-derives the Merkle root from the head set through
// the B07 root algorithm (the same pure function the offline verifier
// uses, so the published root and the recomputed one can never disagree
// on the algorithm).
func computeAnchorRoot(heads []anchor.Head) (algorithmID string, leafCount int, rootDigest string, err error) {
	chains := make([]substrate.ChainHead, len(heads))
	for i, h := range heads {
		chains[i] = substrate.ChainHead{AggregateID: h.AggregateID, SeqMax: h.SeqMax, ChainDigest: h.ChainDigest}
	}
	algorithmID, leafCount, rootDigest, err = substrate.ComputeRoot(chains)
	return algorithmID, leafCount, rootDigest, err
}

// regenerateAnchorArtifact rebuilds a lost Z5 artifact from the committed
// spine row and its signing receipt (idempotent replication, no new
// sequence).
func regenerateAnchorArtifact(db *sql.DB, home, anchorsDir string, seq int, prevDigest, rootDigest,
	algorithmID, headsDigest string, leafCount int, headsJSON, publishedBy, signRequestID string) error {
	var heads []anchor.Head
	if err := json.Unmarshal([]byte(headsJSON), &heads); err != nil {
		return Domainf("The committed checkpoint's head set is unreadable: %v", err)
	}
	var row struct {
		RootDigest   string
		AlgorithmID  string
		SerializerID string
		Scope        string
		SourceActor  string
		SignatureHex *string
		Attestation  *string
		KeyID        *string
		ResultDigest *string
	}
	if err := db.QueryRow(`SELECT root_digest, algorithm_id, serializer_id, scope, source_actor,
		signature_hex, receipt_attestation, key_id, result_digest
		FROM sign_requests WHERE request_id = ?`, signRequestID).
		Scan(&row.RootDigest, &row.AlgorithmID, &row.SerializerID, &row.Scope, &row.SourceActor,
			&row.SignatureHex, &row.Attestation, &row.KeyID, &row.ResultDigest); err != nil {
		return Domainf("The checkpoint's signing receipt is unknown: %v", err)
	}
	// The request digest is not persisted, but it is a deterministic function
	// of the stored row — the same function the original create ran.
	requestDigest, err := signnode.RequestDigest(signRequestID, row.RootDigest, row.AlgorithmID,
		row.SerializerID, row.Scope, row.SourceActor)
	if err != nil {
		return Domainf("The checkpoint's request digest could not be re-rendered: %v", err)
	}
	dir, err := signnode.NodeDir(home)
	if err != nil {
		return err
	}
	keyID := deref(row.KeyID)
	public, err := signnode.PublicKey(dir, keyID)
	if err != nil {
		return Domainf("the anchoring key is unknown to the ring: %v", err)
	}
	artifact := anchor.Build(seq, prevDigest, rootDigest, algorithmID, headsDigest, leafCount,
		heads, publishedBy, "transitional", signRequestID, signReceiptFields{
			RequestDigest:  requestDigest,
			ResultDigest:   deref(row.ResultDigest),
			KeyID:          keyID,
			SignatureHex:   deref(row.SignatureHex),
			AttestationHex: deref(row.Attestation),
			PublicKeyHex:   hex.EncodeToString(public),
		})
	_, err = anchor.WriteArtifact(anchorsDir, artifact)
	return err
}

// signReceiptFields is the receipt face the artifact embeds (the D06
// receipt fields plus the ring public key that makes the artifact verify
// offline without any deployment state).
type signReceiptFields struct {
	RequestDigest  string
	ResultDigest   string
	KeyID          string
	SignatureHex   string
	AttestationHex string
	PublicKeyHex   string
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func digestBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func shortRoot(d string) string {
	if len(d) > 12 {
		return d[:12] + "…"
	}
	return d
}
