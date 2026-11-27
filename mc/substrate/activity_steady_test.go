package substrate

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
)

// S1: the atomic audit triple. A committed AppendActivity lands the row, its
// 'activity' chain event, and the mapping together; a rolled-back append
// leaves none of the three (DC R-03 — the event never outruns its evidence,
// and the row never lives without its chain).
func TestAppendActivityLandsTheAtomicTriple(t *testing.T) {
	db := chainSpine(t)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := AppendActivity(ctx, tx, ActivityEvent{Actor: "mc", Kind: "probe.committed", Detail: "kept"}); err != nil {
		t.Fatalf("committed append: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	tx2, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := AppendActivity(ctx, tx2, ActivityEvent{Actor: "mc", Kind: "probe.rolled_back", Detail: "gone"}); err != nil {
		t.Fatalf("rolled-back append: %v", err)
	}
	if err := tx2.Rollback(); err != nil {
		t.Fatal(err)
	}

	for _, want := range []struct {
		kind               string
		rows, chains, maps int
	}{
		{"probe.committed", 1, 1, 1},
		{"probe.rolled_back", 0, 0, 0},
	} {
		if n := countRows(t, db, `SELECT COUNT(*) FROM activity WHERE kind = ?`, want.kind); n != want.rows {
			t.Fatalf("%s rows = %d; want %d", want.kind, n, want.rows)
		}
		if n := countRows(t, db, `SELECT COUNT(*) FROM audit_events e
			JOIN activity_chain_map m ON m.event_id = e.event_id
			JOIN activity a ON a.id = m.activity_id
			WHERE a.kind = ? AND e.aggregate_id = 'activity'`, want.kind); n != want.chains {
			t.Fatalf("%s chain rows = %d; want %d", want.kind, n, want.chains)
		}
		if n := countRows(t, db, `SELECT COUNT(*) FROM activity_chain_map m
			JOIN activity a ON a.id = m.activity_id WHERE a.kind = ?`, want.kind); n != want.maps {
			t.Fatalf("%s mapping rows = %d; want %d", want.kind, n, want.maps)
		}
	}
	if err := AppendActivity(ctx, db, ActivityEvent{Actor: "mc", Kind: "probe.outside"}); err == nil {
		t.Fatal("AppendActivity accepted a bare database handle; the append must run on the caller's transaction")
	}
}

// S2: chain shape. Steady-state events carry the backfill's identity rule
// (event id derived from the row id), the field-table payload rendered from
// the STORED row, dense sequences, and digests the independent verifier
// recomputes green — a steady row is indistinguishable from a backfilled one.
func TestAppendActivityChainsLikeTheBackfill(t *testing.T) {
	db := chainSpine(t)
	ctx := context.Background()
	// AppendActivity refuses bare handles (S1), so both appends run on one
	// transaction: the second must chain behind the first's event within the
	// same open write.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	subject := "s-1"
	detail := "first steady row"
	if err := AppendActivity(ctx, tx, ActivityEvent{
		Actor: "arbiter", Kind: "probe.steady.a", Subject: subject, Detail: detail,
	}); err != nil {
		t.Fatalf("steady append one: %v", err)
	}
	if err := AppendActivity(ctx, tx, ActivityEvent{
		Actor: "mc", Kind: "probe.steady.b", Detail: "",
	}); err != nil {
		t.Fatalf("steady append two: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	var firstID int64
	if err := db.QueryRow(`SELECT id FROM activity WHERE kind = 'probe.steady.a'`).Scan(&firstID); err != nil {
		t.Fatal(err)
	}
	var firstSeq int
	var firstEventID, firstPrev string
	if err := db.QueryRow(`SELECT seq, event_id, prev_hash FROM audit_events
		WHERE aggregate_id = 'activity' AND event_id = ?`, activityChainEventID(firstID)).
		Scan(&firstSeq, &firstEventID, &firstPrev); err != nil {
		t.Fatalf("the steady row's chain event is missing: %v", err)
	}
	if firstPrev != GenesisHash(activityAggregateID) && firstSeq != 1 {
		// A fresh spine chains from genesis; a migrated one from its head.
		// Either way the link below must hold.
		t.Logf("steady chain starts at seq %d (a migrated spine continues its backfill head)", firstSeq)
	}
	// The second event binds the first: prev_hash == seq's chain_digest.
	var prevOfSecond, digestOfFirst string
	var seqOfFirst, seqOfSecond int
	if err := db.QueryRow(`SELECT seq, chain_digest FROM audit_events e
		JOIN activity_chain_map m ON m.event_id = e.event_id
		JOIN activity a ON a.id = m.activity_id
		WHERE e.aggregate_id = 'activity' AND a.kind = 'probe.steady.a'`).Scan(&seqOfFirst, &digestOfFirst); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT seq, prev_hash FROM audit_events e
		JOIN activity_chain_map m ON m.event_id = e.event_id
		JOIN activity a ON a.id = m.activity_id
		WHERE e.aggregate_id = 'activity' AND a.kind = 'probe.steady.b'`).Scan(&seqOfSecond, &prevOfSecond); err != nil {
		t.Fatal(err)
	}
	if seqOfSecond != seqOfFirst+1 {
		t.Fatalf("steady sequences are not dense: %d then %d", seqOfFirst, seqOfSecond)
	}
	if prevOfSecond != digestOfFirst {
		t.Fatal("the second steady event does not bind the first's chain digest")
	}
	// The payload renders the stored row: subject present, detail present,
	// the NULL optional columns absent exactly as the field table defines.
	var payloadJSON string
	if err := db.QueryRow(`SELECT payload_json FROM audit_events WHERE aggregate_id = 'activity'
		AND event_id = ?`, activityChainEventID(firstID)).Scan(&payloadJSON); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(payloadJSON), &got); err != nil {
		t.Fatal(err)
	}
	if got["actor"] != "arbiter" || got["kind"] != "probe.steady.a" || got["subject"] != subject || got["detail"] != detail {
		t.Fatalf("steady payload = %v; want the stored values verbatim", got)
	}
	if _, ok := got["project"]; ok {
		t.Fatal("a NULL project must be absent from the payload")
	}
	if _, ok := got["id"]; !ok {
		t.Fatal("the payload must carry the row id (the B08 field table)")
	}
	if id := got["id"]; id != strconv.FormatInt(firstID, 10) {
		t.Fatalf("payload id = %v; want %d", id, firstID)
	}
	// Empty detail is stored and rendered as the empty string (a pointer-
	// free `any` keeps "" distinct from nil).
	var secondID int64
	if err := db.QueryRow(`SELECT id FROM activity WHERE kind = 'probe.steady.b'`).Scan(&secondID); err != nil {
		t.Fatal(err)
	}
	var detailIsNull int
	if err := db.QueryRow(`SELECT detail IS NULL FROM activity WHERE id = ?`, secondID).Scan(&detailIsNull); err != nil {
		t.Fatal(err)
	}
	if detailIsNull != 0 {
		t.Fatal(`the second row stored NULL for Detail: ""; empty is not NULL on the central append`)
	}
	var secondPayload string
	if err := db.QueryRow(`SELECT payload_json FROM audit_events WHERE aggregate_id = 'activity'
		AND event_id = ?`, activityChainEventID(secondID)).Scan(&secondPayload); err != nil {
		t.Fatal(err)
	}
	var gotB map[string]any
	if err := json.Unmarshal([]byte(secondPayload), &gotB); err != nil {
		t.Fatal(err)
	}
	if gotB["detail"] != "" {
		t.Fatalf("second payload detail = %v; want the empty string present", gotB["detail"])
	}
	if _, ok := gotB["subject"]; ok {
		t.Fatal("a NULL subject must be absent from the second payload (nil is not \"\")")
	}
	report, err := VerifyAuditChain(db, activityAggregateID)
	if err != nil {
		t.Fatalf("independent chain verification: %v", err)
	}
	if len(report.Errors) != 0 {
		t.Fatalf("the activity chain does not verify: %v", report.Errors)
	}
}

// S3: the pre-chain arm. A spine without the mapping table (a migration step
// below the sixth) lands row-only, with no error — the sixth step's backfill
// will project the row exactly once. The full-table coverage asserted by the
// sixth-step tests depends on this arm never chaining early.
func TestAppendActivityLandsRowOnlyBeforeTheMechanism(t *testing.T) {
	db := chainSpine(t)
	if _, err := db.Exec(`DROP TABLE activity_chain_map`); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := AppendActivity(ctx, tx, ActivityEvent{Actor: "mc", Kind: "probe.prechain"}); err != nil {
		t.Fatalf("pre-chain append: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM activity WHERE kind = 'probe.prechain'`); n != 1 {
		t.Fatalf("pre-chain row = %d; want 1", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM audit_events WHERE aggregate_id = 'activity'`); n != 0 {
		t.Fatalf("pre-chain append wrote %d chain rows; want 0 (the backfill owns these)", n)
	}
}
