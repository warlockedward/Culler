package substrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrBackfillProof identifies a usage backfill whose proof failed inside
// the migration step's transaction: the step rolls back as a unit, so a
// spine never carries a usage table the proof cannot stand behind.
var ErrBackfillProof = errors.New("the usage backfill proof failed")

// maxVersionRows is the backfill selection: each slot's current
// (max source_version) observation row. The migration INSERT and the
// proof must agree on this set.
const maxVersionRows = `
    SELECT o.observation_id, o.payload_digest
    FROM usage_observations o
    WHERE o.source_version = (
        SELECT MAX(o2.source_version) FROM usage_observations o2
        WHERE o2.request_identity = o.request_identity AND o2.event_seq = o.event_seq)`

// VerifyUsageBackfill proves the fifth step's blob backfill before it may
// commit, in the step's own transaction. The three checks are the ticket's
// acceptance sentence: the count (every slot with an observation has exactly
// one current row, nothing more), the multiset digest (the promoted content
// is the selected content, order-blind), and both anti-join directions
// (every v2 row traces to a v1 source; every selected v1 row is present).
// Any failure refuses the step; the originals in the v1 tables are never
// touched by it.
func VerifyUsageBackfill(tx *sql.Tx) error {
	ctx := context.Background()

	var v2Count, slotCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage`).Scan(&v2Count); err != nil {
		return fmt.Errorf("backfill proof read usage count: %w", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM (`+maxVersionRows+`)`).Scan(&slotCount); err != nil {
		return fmt.Errorf("backfill proof read selected slot count: %w", err)
	}
	if v2Count != slotCount {
		return fmt.Errorf("%w: usage carries %d rows, the v1 selection carries %d slots", ErrBackfillProof, v2Count, slotCount)
	}

	v2Digests, err := selectedDigests(ctx, tx, `SELECT payload_digest FROM usage`)
	if err != nil {
		return fmt.Errorf("backfill proof read usage digests: %w", err)
	}
	v1Digests, err := selectedDigests(ctx, tx, `SELECT payload_digest FROM (`+maxVersionRows+`)`)
	if err != nil {
		return fmt.Errorf("backfill proof read selected digests: %w", err)
	}
	if multisetDigest(v2Digests) != multisetDigest(v1Digests) {
		return fmt.Errorf("%w: the usage digests and the selected v1 digests do not form equal multisets", ErrBackfillProof)
	}

	// v2 -> v1: no promoted row may lack its source observation.
	var orphaned int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage u
		WHERE NOT EXISTS (SELECT 1 FROM usage_observations o WHERE o.observation_id = u.source_report_id)`).Scan(&orphaned); err != nil {
		return fmt.Errorf("backfill proof anti-join v2->v1: %w", err)
	}
	if orphaned != 0 {
		return fmt.Errorf("%w: %d usage rows trace to no v1 observation", ErrBackfillProof, orphaned)
	}
	// v1 -> v2: every selected observation must have been promoted.
	var missing int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM (`+maxVersionRows+`) selected
		WHERE NOT EXISTS (SELECT 1 FROM usage u WHERE u.source_report_id = selected.observation_id)`).Scan(&missing); err != nil {
		return fmt.Errorf("backfill proof anti-join v1->v2: %w", err)
	}
	if missing != 0 {
		return fmt.Errorf("%w: %d selected v1 observations were not promoted", ErrBackfillProof, missing)
	}
	return nil
}

func selectedDigests(ctx context.Context, tx *sql.Tx, query string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// multisetDigest is the order-blind identity of a digest set: sorted
// ascending and folded, so the two sides of the proof compare as
// multisets, not as sequences.
func multisetDigest(digests []string) string {
	sorted := append([]string(nil), digests...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	return hex.EncodeToString(sum[:])
}

// usageCurrentWriter is the statement surface the v2 projection runs on
// inside the caller's open transaction.
type usageCurrentWriter interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// UpsertUsageCurrent lands the just-recorded observation as the slot's
// current v2 row in the caller's transaction: the previous current row
// (any older version) is replaced, nothing else is touched. The v1 row
// and its chain event are the evidence; this row is the projection.
func UpsertUsageCurrent(ctx context.Context, q usageCurrentWriter, identity string, in UsageObservationInput, observationID, state, payloadDigest string) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM usage WHERE request_identity = ? AND event_seq = ?`, identity, in.EventSeq); err != nil {
		return fmt.Errorf("replace usage current row: %w", err)
	}
	_, err := q.ExecContext(ctx, `INSERT INTO usage (
		source_report_id, request_identity, local_request_id, provider_request_id,
		event_seq, source_version, origin, usage_family, model_id,
		output_tokens, input_tokens, report_kind, finality, identity_provability,
		state, payload_digest, supersedes_report_id, occurred_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		observationID, identity, in.LocalRequestID, sqlNullable(in.ProviderRequestID),
		in.EventSeq, in.SourceVersion, in.Origin, in.UsageFamily, in.ModelID,
		in.OutputTokens, in.InputTokens, in.ReportKind, in.Finality, in.IdentityProvability,
		state, payloadDigest, sqlNullable(in.SupersedesObservationID),
		canonicalTime(in.OccurredAt))
	if err != nil {
		return fmt.Errorf("insert usage current row: %w", err)
	}
	return nil
}
