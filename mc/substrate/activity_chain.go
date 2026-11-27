package substrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// ErrActivityBackfillProof identifies an activity-to-chain backfill whose
// proof failed inside the migration step's transaction: the step rolls back
// as a unit, so a spine never carries a chain projection its proof cannot
// stand behind.
var ErrActivityBackfillProof = errors.New("the activity backfill proof failed")

const activityAggregateID = "activity"

// spineTimeToCanonical converts the activity table's storage clock
// ("2006-01-02 15:04:05", UTC) to the chain's canonical time form. The
// conversion is deterministic, so a backfilled payload always re-encodes to
// the same bytes.
func spineTimeToCanonical(value string) (string, error) {
	t, err := time.Parse("2006-01-02 15:04:05", value)
	if err != nil {
		return "", fmt.Errorf("convert activity time %q: %w", value, err)
	}
	return t.UTC().Format("2006-01-02T15:04:05.000000Z"), nil
}

// activityChainEventID gives one activity row's chain event a stable
// identity: the same row always maps to the same event, which is what makes
// the backfill idempotent and the mapping table auditable.
func activityChainEventID(activityID int64) string {
	sum := sha256.Sum256([]byte("shz-activity-chain-v1|" + strconv.FormatInt(activityID, 10)))
	return hex.EncodeToString(sum[:])
}

// activityPayload renders one activity row against the B08 field table: the
// row's own values, verbatim — including a legacy actor, which the chain
// records as stored, never re-authored (IG §7). A NULL column is ABSENT from
// the payload.
func activityPayload(id int64, actor, kind, createdAt string, subject, detail, dispatchKey, dispatchRequestID, dispatchResult, project *string) (map[string]any, error) {
	canonical, err := spineTimeToCanonical(createdAt)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"id":         strconv.FormatInt(id, 10),
		"actor":      actor,
		"kind":       kind,
		"created_at": canonical,
	}
	for _, optional := range []struct {
		name  string
		value *string
	}{
		{"subject", subject},
		{"detail", detail},
		{"dispatch_key", dispatchKey},
		{"dispatch_request_id", dispatchRequestID},
		{"dispatch_result", dispatchResult},
		{"project", project},
	} {
		if optional.value != nil {
			payload[optional.name] = *optional.value
		}
	}
	return payload, nil
}

// migrateActivityToChain is the sixth step's data hook: it projects every
// activity row into the activity aggregate's chain, in id order, and records
// the row-to-event mapping. The chain digests cannot be computed in SQL, so
// the movement runs here, in the step's transaction, and the step's proof
// (VerifyActivityBackfill) covers it before anything commits. The activity
// table is read, never written.
func migrateActivityToChain(tx *sql.Tx) error {
	ctx := context.Background()
	rows, err := tx.QueryContext(ctx, `SELECT id, actor, kind, created_at, subject, detail, dispatch_key,
		dispatch_request_id, dispatch_result, project FROM activity ORDER BY id`)
	if err != nil {
		return fmt.Errorf("read activity for the chain backfill: %w", err)
	}
	defer rows.Close()

	seq := 1
	prev := GenesisHash(activityAggregateID)
	for rows.Next() {
		var id int64
		var actor, kind, createdAt string
		var subject, detail, dispatchKey, dispatchRequestID, dispatchResult, project sql.NullString
		if err := rows.Scan(&id, &actor, &kind, &createdAt, &subject, &detail, &dispatchKey, &dispatchRequestID, &dispatchResult, &project); err != nil {
			return fmt.Errorf("scan activity row %d: %w", seq, err)
		}
		payload, err := activityPayload(id, actor, kind, createdAt, nullStringPtr(subject), nullStringPtr(detail), nullStringPtr(dispatchKey), nullStringPtr(dispatchRequestID), nullStringPtr(dispatchResult), nullStringPtr(project))
		if err != nil {
			return fmt.Errorf("render activity payload for row %d: %w", seq, err)
		}
		eventID := activityChainEventID(id)
		event := map[string]any{
			"serializer_id":  canonicalSerializerID,
			"event_id":       eventID,
			"aggregate_id":   activityAggregateID,
			"aggregate_type": activityAggregateID,
			"event_type":     kind,
			"occurred_at":    payload["created_at"],
			"recorded_at":    canonNow(),
			"prev_hash":      prev,
			"payload_family": "activity",
			"payload":        payload,
		}
		// Pre-compute the chain head locally so the next event can carry its
		// predecessor without a re-query; AppendAuditEvent re-verifies the
		// same relation against the stored rows and refuses on mismatch.
		withSeq := make(map[string]any, len(event)+1)
		for key, value := range event {
			withSeq[key] = value
		}
		withSeq["seq"] = strconv.Itoa(seq)
		canonical, err := EncodeCanonicalEvent(withSeq)
		if err != nil {
			return fmt.Errorf("encode activity chain event %d: %w", seq, err)
		}
		nextPrev, err := ChainDigest(prev, EventDigest(canonical))
		if err != nil {
			return fmt.Errorf("chain activity event %d: %w", seq, err)
		}
		if err := AppendAuditEvent(tx, event); err != nil {
			return fmt.Errorf("append activity chain event %d: %w", seq, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO activity_chain_map (activity_id, event_id) VALUES (?, ?)`, id, eventID); err != nil {
			return fmt.Errorf("map activity row %d to its chain event: %w", id, err)
		}
		prev = nextPrev
		seq++
	}
	return rows.Err()
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

// shortEventID trims an event id to its leading bytes for proof messages.
func shortEventID(eventID string) string {
	if len(eventID) > 12 {
		return eventID[:12] + "…"
	}
	return eventID
}

// VerifyActivityBackfill proves the sixth step's projection before it may
// commit, in the step's own transaction. Three checks, matching the ticket's
// acceptance sentence: coverage (every activity row is mapped and chained,
// one to one), the mapping's reachability (every mapped event id exists in
// the chain), and order (the chain's sequence is the activity id order).
// Any failure refuses the step; the activity table is never written by it.
func VerifyActivityBackfill(tx *sql.Tx) error {
	ctx := context.Background()
	var mapCount, activityCount, chainCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_chain_map`).Scan(&mapCount); err != nil {
		return fmt.Errorf("activity proof read mapping count: %w", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM activity`).Scan(&activityCount); err != nil {
		return fmt.Errorf("activity proof read activity count: %w", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE aggregate_id = ?`, activityAggregateID).Scan(&chainCount); err != nil {
		return fmt.Errorf("activity proof read chain count: %w", err)
	}
	if mapCount != activityCount || chainCount != activityCount {
		return fmt.Errorf("%w: mapping %d, chain %d, activity %d rows; want all equal", ErrActivityBackfillProof, mapCount, chainCount, activityCount)
	}

	// Reachability: no mapped event id may point at nothing.
	var orphaned int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_chain_map m
		WHERE NOT EXISTS (SELECT 1 FROM audit_events e WHERE e.event_id = m.event_id AND e.aggregate_id = ?)`,
		activityAggregateID).Scan(&orphaned); err != nil {
		return fmt.Errorf("activity proof anti-join: %w", err)
	}
	if orphaned != 0 {
		return fmt.Errorf("%w: %d mapping rows trace to no chain event", ErrActivityBackfillProof, orphaned)
	}

	// Order: the chain's sequence must be the activity id order, event for
	// event. Both sides reduced to their event-id sequences and compared.
	mapRows, err := tx.QueryContext(ctx, `SELECT m.event_id FROM activity_chain_map m ORDER BY m.activity_id ASC`)
	if err != nil {
		return fmt.Errorf("activity proof read mapping order: %w", err)
	}
	defer mapRows.Close()
	var mapOrder []string
	for mapRows.Next() {
		var eventID string
		if err := mapRows.Scan(&eventID); err != nil {
			return fmt.Errorf("activity proof scan mapping order: %w", err)
		}
		mapOrder = append(mapOrder, eventID)
	}
	if err := mapRows.Err(); err != nil {
		return fmt.Errorf("activity proof iterate mapping order: %w", err)
	}
	chainRows, err := tx.QueryContext(ctx, `SELECT event_id FROM audit_events WHERE aggregate_id = ? ORDER BY seq ASC`, activityAggregateID)
	if err != nil {
		return fmt.Errorf("activity proof read chain order: %w", err)
	}
	defer chainRows.Close()
	chainOrder := []string{}
	for chainRows.Next() {
		var eventID string
		if err := chainRows.Scan(&eventID); err != nil {
			return fmt.Errorf("activity proof scan chain order: %w", err)
		}
		chainOrder = append(chainOrder, eventID)
	}
	if err := chainRows.Err(); err != nil {
		return fmt.Errorf("activity proof iterate chain order: %w", err)
	}
	if len(mapOrder) != len(chainOrder) {
		return fmt.Errorf("%w: mapping order has %d events, chain order has %d", ErrActivityBackfillProof, len(mapOrder), len(chainOrder))
	}
	for i := range mapOrder {
		if mapOrder[i] != chainOrder[i] {
			return fmt.Errorf("%w: chain position %d carries %s, the activity order requires %s", ErrActivityBackfillProof, i+1, shortEventID(chainOrder[i]), shortEventID(mapOrder[i]))
		}
	}
	return nil
}

// ErrActivitySurface identifies an AppendActivity call that is not running
// on an open transaction surface. The row, its chain event, and the mapping
// commit or vanish as one unit (DC R-03), so the append is refused on a bare
// database handle rather than split across implicit transactions.
var ErrActivitySurface = errors.New("activity append needs an open transaction")

// ActivityEvent is one steady-state activity row against the central append.
// An optional column carries the same Go value the replaced SQL bound: nil
// stores NULL, a string stores the string, an int64 lands under the column's
// TEXT affinity exactly as the old placeholder did. The field is deliberately
// `any` — the central API re-binds the call's value without re-formatting it,
// which is what makes the 35-site rewiring a zero-rewrite at the byte level.
type ActivityEvent struct {
	Actor             string
	Kind              string
	Subject           any
	Detail            any
	Project           any
	DispatchKey       any
	DispatchRequestID any
	DispatchResult    any
}

// AppendActivity is the single writer of activity rows for the spine: it
// lands the row, its 'activity' aggregate chain event, and the row-to-event
// mapping on the caller's open transaction, in that order. The chain event
// is rendered by the same authority the sixth step's backfill used
// (activityPayload, activityChainEventID), so a steady-state row is
// indistinguishable from a backfilled one — same event identity, same
// payload bytes, same chain linkage. The central API is the coverage gate's
// only exempted writer: a new activity-row INSERT anywhere else in the
// module turns the gate red.
func AppendActivity(ctx context.Context, q auditor, ev ActivityEvent) error {
	switch q.(type) {
	case *sql.Tx, *sql.Conn:
	default:
		return fmt.Errorf("%w: got %T", ErrActivitySurface, q)
	}
	res, err := q.ExecContext(ctx, `INSERT INTO activity
		(actor, kind, subject, detail, project, dispatch_key, dispatch_request_id, dispatch_result)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		ev.Actor, ev.Kind, ev.Subject, ev.Detail, ev.Project,
		ev.DispatchKey, ev.DispatchRequestID, ev.DispatchResult)
	if err != nil {
		return fmt.Errorf("append activity row %s: %w", ev.Kind, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("read the new activity id: %w", err)
	}
	// The pre-chain world: a step recorded before the sixth step lands has no
	// mapping table to map into, and the sixth step's backfill projects every
	// row that exists at its time — so these rows reach the chain exactly
	// once, through the backfill, never twice. A runtime spine always carries
	// the mechanism; the row-only arm belongs to migration history alone.
	{
		var maps int
		probe, err := q.QueryContext(ctx, `SELECT COUNT(*) FROM sqlite_master
			WHERE type = 'table' AND name = 'activity_chain_map'`)
		if err != nil {
			return fmt.Errorf("probe the activity chain mechanism: %w", err)
		}
		if probe.Next() {
			if err := probe.Scan(&maps); err != nil {
				probe.Close()
				return fmt.Errorf("scan the activity chain probe: %w", err)
			}
		}
		if err := probe.Err(); err != nil {
			probe.Close()
			return fmt.Errorf("iterate the activity chain probe: %w", err)
		}
		probe.Close()
		if maps == 0 {
			return nil
		}
	}
	// The row is read back, never trusted from the call: the payload renders
	// the stored bytes, so the chain records what the spine holds — the same
	// rule the backfill follows.
	var (
		actor, kind, createdAt         string
		subject, detail, project       sql.NullString
		dispatchKey, dispatchRequestID sql.NullString
		dispatchResult                 sql.NullString
	)
	rows, err := q.QueryContext(ctx, `SELECT actor, kind, created_at, subject, detail,
		project, dispatch_key, dispatch_request_id, dispatch_result
		FROM activity WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("read back activity row %d: %w", id, err)
	}
	defer rows.Close()
	if !rows.Next() {
		return fmt.Errorf("the new activity row %d is not readable", id)
	}
	if err := rows.Scan(&actor, &kind, &createdAt, &subject, &detail,
		&project, &dispatchKey, &dispatchRequestID, &dispatchResult); err != nil {
		return fmt.Errorf("scan activity row %d: %w", id, err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate activity row %d: %w", id, err)
	}
	payload, err := activityPayload(id, actor, kind, createdAt,
		nullStringPtr(subject), nullStringPtr(detail), nullStringPtr(dispatchKey),
		nullStringPtr(dispatchRequestID), nullStringPtr(dispatchResult), nullStringPtr(project))
	if err != nil {
		return fmt.Errorf("render activity payload for row %d: %w", id, err)
	}
	prev, err := activityChainPrev(ctx, q)
	if err != nil {
		return err
	}
	eventID := activityChainEventID(id)
	if err := AppendAuditEvent(q, map[string]any{
		"serializer_id":  canonicalSerializerID,
		"event_id":       eventID,
		"aggregate_id":   activityAggregateID,
		"aggregate_type": activityAggregateID,
		"event_type":     kind,
		"occurred_at":    payload["created_at"],
		"recorded_at":    canonNow(),
		"prev_hash":      prev,
		"payload_family": "activity",
		"payload":        payload,
	}); err != nil {
		return fmt.Errorf("append activity chain event for row %d: %w", id, err)
	}
	if _, err := q.ExecContext(ctx, `INSERT INTO activity_chain_map (activity_id, event_id) VALUES (?, ?)`,
		id, eventID); err != nil {
		return fmt.Errorf("map activity row %d to its chain event: %w", id, err)
	}
	return nil
}

// activityChainPrev resolves the predecessor the next 'activity' aggregate
// event must carry: the stored chain_digest of the current head, or the
// aggregate's genesis before the first row.
func activityChainPrev(ctx context.Context, q auditor) (string, error) {
	var headSeq int
	rows, err := q.QueryContext(ctx,
		`SELECT COALESCE(MAX(seq), 0) FROM audit_events WHERE aggregate_id = ?`, activityAggregateID)
	if err != nil {
		return "", fmt.Errorf("read the activity chain head: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return "", fmt.Errorf("read the activity chain head: no result row")
	}
	if err := rows.Scan(&headSeq); err != nil {
		return "", fmt.Errorf("scan the activity chain head: %w", err)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if headSeq == 0 {
		return GenesisHash(activityAggregateID), nil
	}
	rows, err = q.QueryContext(ctx,
		`SELECT chain_digest FROM audit_events WHERE aggregate_id = ? AND seq = ?`, activityAggregateID, headSeq)
	if err != nil {
		return "", fmt.Errorf("read the activity chain head digest: %w", err)
	}
	defer rows.Close()
	var digest string
	if !rows.Next() {
		return "", fmt.Errorf("the activity chain has no head at seq %d", headSeq)
	}
	if err := rows.Scan(&digest); err != nil {
		return "", fmt.Errorf("scan the activity chain head digest: %w", err)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return digest, nil
}
