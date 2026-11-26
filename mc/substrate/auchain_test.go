package substrate

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	sqlite3 "modernc.org/sqlite/lib"
)

// storedChainRow is the digest face of one stored chain row.
type storedChainRow struct {
	Seq         int
	EventID     string
	PrevHash    string
	EventDigest string
	ChainDigest string
}

func lastChainRow(t *testing.T, db *sql.DB, aggregateID string) storedChainRow {
	t.Helper()
	var r storedChainRow
	err := db.QueryRow(`SELECT seq, event_id, prev_hash, event_digest, chain_digest
		FROM audit_events WHERE aggregate_id = ? ORDER BY seq DESC LIMIT 1`, aggregateID).
		Scan(&r.Seq, &r.EventID, &r.PrevHash, &r.EventDigest, &r.ChainDigest)
	if err != nil {
		t.Fatalf("read last chain row of %s: %v", aggregateID, err)
	}
	return r
}

func appendProbeEvent(t *testing.T, db *sql.DB, aggregateID, aggregateType, prevHash string) {
	t.Helper()
	eventID, err := NewEventID()
	if err != nil {
		t.Fatalf("allocate event id: %v", err)
	}
	now := canonNow()
	err = AppendAuditEvent(db, map[string]any{
		"serializer_id":  canonicalSerializerID,
		"event_id":       eventID,
		"aggregate_id":   aggregateID,
		"aggregate_type": aggregateType,
		"event_type":     "probe",
		"occurred_at":    now,
		"recorded_at":    now,
		"prev_hash":      prevHash,
		"payload_family": "probe_order",
		"payload":        map[string]any{"zeta": "z", "alpha": "a"},
	})
	if err != nil {
		t.Fatalf("append probe event to %s: %v", aggregateID, err)
	}
}

// isConstraintAbort reports whether the driver error is a SQLite
// constraint rejection (CHECK failure or trigger RAISE(ABORT)).
func isConstraintAbort(err error) bool {
	var failure interface{ Code() int }
	return errors.As(err, &failure) && failure.Code()&0xff == sqlite3.SQLITE_CONSTRAINT
}

// chainSpine provisions a fresh current-schema spine stamped at the current
// identity, ready for chain operations.
func chainSpine(t *testing.T) *sql.DB {
	t.Helper()
	db := provisionProductionSpine(t, filepath.Join(t.TempDir(), "spine.db"), testUUID, CurrentSchemaID)
	t.Cleanup(func() { db.Close() })
	return db
}

// W3: the genesis value separates aggregate types (domain separation,
// anti chain-swap), pinning the vector-derived constants. A row whose
// aggregate_type carries another type's genesis is detected by the
// verifier.
func TestGenesisHashSeparatesAggregateTypes(t *testing.T) {
	pins := map[string]string{
		"run":       "8c5260b2de7f556777b9503d40a1d9dc40ce046c3df57edd33de6e02fcfee339",
		"usage":     "c94c5e0f22a13654ccfae58306170ab32468b172e7d69c285ab58fc9b1955107",
		"task":      "90af12d7c6a42f5d2a801b7d72f9bbcd72ad579de86d7f6bac04099dc8610436",
		"budget":    "66608b081fc9aef2c25d71061890e9181b452026f40601269e78b8a7653c72dd",
		"admission": "0239c1ebdc4bb9e517e18fbf3ac53f8b392beb8d2a2355fd755096800789f8f2",
	}
	for aggregateType, want := range pins {
		if got := GenesisHash(aggregateType); got != want {
			t.Fatalf("GenesisHash(%q) = %s; want the vector-derived %s", aggregateType, got, want)
		}
	}
	if GenesisHash("run") == GenesisHash("task") {
		t.Fatal("genesis values collide across aggregate types")
	}

	// The cross-type transplant: a seq-1 row of type "task" carrying the
	// "run" genesis passes every shape CHECK but breaks relation 1.
	db := chainSpine(t)
	_, err := db.Exec(`INSERT INTO audit_events (
		aggregate_id, seq, event_id, aggregate_type, event_type, serializer_id,
		occurred_at, recorded_at, prev_hash, event_digest, chain_digest,
		payload_family, payload_json
	) VALUES (?, 1, ?, 'task', 'probe', 'shz-canon-v1',
		'2026-11-19T12:00:05.000000Z', '2026-11-19T12:00:05.000002Z', ?, ?, ?,
		'probe_order', '{"zeta":"z","alpha":"a"}')`,
		"transplanted", strings.Repeat("a", 64), GenesisHash("run"),
		strings.Repeat("b", 64), strings.Repeat("c", 64))
	if err != nil {
		t.Fatalf("insert forged transplant row: %v", err)
	}
	report, err := VerifyAuditChain(db, "transplanted")
	if err != nil {
		t.Fatalf("verify = %v", err)
	}
	broken := false
	for _, e := range report.Errors {
		if e.Kind == "broken-chain" && strings.Contains(e.Message, "does not match the chain state") {
			broken = true
		}
	}
	if !broken {
		t.Fatalf("verify findings = %+v; want the broken-chain detection of the transplanted genesis", report.Errors)
	}
}

// W4: three events on one aggregate — each hop's chain_digest recomputes
// closed and the prev_hash double relation holds at every row.
func TestChainRecursionClosesAcrossHops(t *testing.T) {
	db := chainSpine(t)

	prev := GenesisHash("run")
	rows := []storedChainRow{}
	for i := 1; i <= 3; i++ {
		appendProbeEvent(t, db, "agg-1", "run", prev)
		r := lastChainRow(t, db, "agg-1")
		if r.Seq != i {
			t.Fatalf("event %d sits at seq %d", i, r.Seq)
		}
		rows = append(rows, r)
		prev = r.ChainDigest
	}
	for i, r := range rows {
		wantPrev := GenesisHash("run")
		if i > 0 {
			wantPrev = rows[i-1].ChainDigest
		}
		if r.PrevHash != wantPrev {
			t.Fatalf("row %d prev_hash = %s; want %s", i+1, r.PrevHash, wantPrev)
		}
		wantCD, err := ChainDigest(r.PrevHash, r.EventDigest)
		if err != nil {
			t.Fatalf("recompute hop %d: %v", i+1, err)
		}
		if wantCD != r.ChainDigest {
			t.Fatalf("row %d chain_digest = %s; want %s", i+1, r.ChainDigest, wantCD)
		}
	}
	report, err := VerifyAuditChain(db, "")
	if err != nil {
		t.Fatalf("verify = %v", err)
	}
	if len(report.Errors) != 0 {
		t.Fatalf("verify findings on a clean chain: %+v", report.Errors)
	}
	if len(report.Heads) != 1 || report.Heads[0].SeqMax != 3 || report.Heads[0].ChainDigest != rows[2].ChainDigest {
		t.Fatalf("heads = %+v; want one head at seq 3", report.Heads)
	}
}

// W5: seq allocation is max+1 under the transaction's serialization — no
// gaps, no duplicates — and the seq_prefix trigger refuses a direct hole.
func TestSeqAllocationIsGaplessAndTriggerRefusesHoles(t *testing.T) {
	db := chainSpine(t)

	prev := GenesisHash("run")
	for i := 1; i <= 3; i++ {
		appendProbeEvent(t, db, "seq-agg", "run", prev)
		r := lastChainRow(t, db, "seq-agg")
		if r.Seq != i {
			t.Fatalf("event %d allocated seq %d", i, r.Seq)
		}
		prev = r.ChainDigest
	}

	// Concurrent appends to distinct aggregates under parallel BEGIN
	// IMMEDIATE: every aggregate lands at exactly seq 1.
	const workers = 8
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			errs <- OpenAggregate(db, fmt.Sprintf("conc-%d", n), "run", "mc", "")
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent open failed: %v", err)
		}
	}
	for i := 0; i < workers; i++ {
		r := lastChainRow(t, db, fmt.Sprintf("conc-%d", i))
		if r.Seq != 1 {
			t.Fatalf("concurrent aggregate %d sits at seq %d; want 1", i, r.Seq)
		}
	}

	// Negative control: a direct INSERT that skips the prefix is refused.
	_, err := db.Exec(`INSERT INTO audit_events (
		aggregate_id, seq, event_id, aggregate_type, event_type, serializer_id,
		occurred_at, recorded_at, prev_hash, event_digest, chain_digest,
		payload_family, payload_json
	) VALUES (?, 5, ?, 'run', 'probe', 'shz-canon-v1',
		'2026-11-19T12:00:05.000000Z', '2026-11-19T12:00:05.000002Z', ?, ?, ?,
		'probe_order', '{"zeta":"z","alpha":"a"}')`,
		"hole-agg", strings.Repeat("d", 64), strings.Repeat("e", 64),
		strings.Repeat("f", 64), strings.Repeat("0", 64))
	if err == nil {
		t.Fatal("seq hole insert committed; want the seq_prefix trigger abort")
	}
	if !isConstraintAbort(err) {
		t.Fatalf("seq hole insert returned %v; want the constraint abort", err)
	}
}

// W6: the append-only triggers — UPDATE and DELETE on audit_events ABORT.
func TestAuditEventsAreAppendOnly(t *testing.T) {
	db := chainSpine(t)
	appendProbeEvent(t, db, "agg-1", "run", GenesisHash("run"))

	_, err := db.Exec(`UPDATE audit_events SET event_type = 'forged' WHERE aggregate_id = 'agg-1'`)
	if err == nil {
		t.Fatal("UPDATE on audit_events committed; want the append-only trigger abort")
	}
	if !isConstraintAbort(err) {
		t.Fatalf("UPDATE returned %v; want the constraint abort", err)
	}
	_, err = db.Exec(`DELETE FROM audit_events WHERE aggregate_id = 'agg-1'`)
	if err == nil {
		t.Fatal("DELETE on audit_events committed; want the append-only trigger abort")
	}
	if !isConstraintAbort(err) {
		t.Fatalf("DELETE returned %v; want the constraint abort", err)
	}

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE aggregate_id = 'agg-1'`).Scan(&n); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if n != 1 {
		t.Fatalf("rows after the refused mutations = %d; want 1 untouched", n)
	}
}

// W7: a forged row that passes every shape CHECK but breaks the chain is
// reported by the verifier, and the other aggregate still yields its head.
func TestVerifyDetectsBrokenChain(t *testing.T) {
	db := chainSpine(t)
	appendProbeEvent(t, db, "good-1", "run", GenesisHash("run"))
	appendProbeEvent(t, db, "bad-1", "run", GenesisHash("run"))
	first := lastChainRow(t, db, "bad-1")

	// A seq-2 row whose prev_hash is shape-legal but not the stored head's
	// chain_digest: relation 1 fails.
	_, err := db.Exec(`INSERT INTO audit_events (
		aggregate_id, seq, event_id, aggregate_type, event_type, serializer_id,
		occurred_at, recorded_at, prev_hash, event_digest, chain_digest,
		payload_family, payload_json
	) VALUES ('bad-1', 2, ?, 'run', 'probe', 'shz-canon-v1',
		'2026-11-19T12:00:06.000000Z', '2026-11-19T12:00:06.000002Z', ?, ?, ?,
		'probe_order', '{"zeta":"z","alpha":"a"}')`,
		strings.Repeat("7", 64), strings.Repeat("8", 64),
		strings.Repeat("9", 64), strings.Repeat("a", 63)+"b")
	if err != nil {
		t.Fatalf("insert forged row: %v", err)
	}

	report, err := VerifyAuditChain(db, "")
	if err != nil {
		t.Fatalf("verify = %v", err)
	}
	kinds := map[string]bool{}
	for _, e := range report.Errors {
		if e.AggregateID == "bad-1" {
			kinds[e.Kind] = true
		}
		if e.AggregateID == "good-1" {
			t.Fatalf("the clean aggregate carries a finding: %+v", e)
		}
	}
	if !kinds["broken-chain"] {
		t.Fatalf("findings = %+v; want the broken-chain detection on bad-1", report.Errors)
	}
	if len(report.Heads) != 2 {
		t.Fatalf("heads = %+v; want both aggregates to still yield their head", report.Heads)
	}
	if first.ChainDigest == "" {
		t.Fatal("the clean row's chain digest is empty")
	}
}

// W8: a stored event_digest that does not match its recomputation is the
// fork / corruption signal, reported without disturbing the closed
// recursion of the row.
func TestVerifyDetectsDigestMismatch(t *testing.T) {
	db := chainSpine(t)
	appendProbeEvent(t, db, "m-1", "run", GenesisHash("run"))
	first := lastChainRow(t, db, "m-1")

	// Build the true seq-2 event, then store it with a forged
	// event_digest: the recursion still closes over the true digest, so
	// exactly the digest-mismatch finding fires.
	eventID, err := NewEventID()
	if err != nil {
		t.Fatalf("allocate event id: %v", err)
	}
	now := canonNow()
	ev := map[string]any{
		"serializer_id":  canonicalSerializerID,
		"event_id":       eventID,
		"aggregate_id":   "m-1",
		"aggregate_type": "run",
		"seq":            "2",
		"event_type":     "probe",
		"occurred_at":    now,
		"recorded_at":    now,
		"prev_hash":      first.ChainDigest,
		"payload_family": "probe_order",
		"payload":        map[string]any{"zeta": "z", "alpha": "a"},
	}
	canonical, err := EncodeCanonicalEvent(ev)
	if err != nil {
		t.Fatalf("encode true event: %v", err)
	}
	trueED := EventDigest(canonical)
	trueCD, err := ChainDigest(first.ChainDigest, trueED)
	if err != nil {
		t.Fatalf("recompute true chain digest: %v", err)
	}
	forgedED := trueED
	if forgedED[0] == 'b' {
		forgedED = "a" + forgedED[1:]
	} else {
		forgedED = "b" + forgedED[1:]
	}
	_, err = db.Exec(`INSERT INTO audit_events (
		aggregate_id, seq, event_id, aggregate_type, event_type, serializer_id,
		occurred_at, recorded_at, prev_hash, event_digest, chain_digest,
		payload_family, payload_json
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"m-1", 2, eventID, "run", "probe", canonicalSerializerID,
		now, now, first.ChainDigest, forgedED, trueCD,
		"probe_order", `{"zeta":"z","alpha":"a"}`)
	if err != nil {
		t.Fatalf("insert digest-forged row: %v", err)
	}

	report, err := VerifyAuditChain(db, "m-1")
	if err != nil {
		t.Fatalf("verify = %v", err)
	}
	kinds := map[string]bool{}
	for _, e := range report.Errors {
		kinds[e.Kind] = true
	}
	if !kinds["digest-mismatch"] {
		t.Fatalf("findings = %+v; want the digest-mismatch detection", report.Errors)
	}
	if kinds["broken-chain"] {
		t.Fatalf("findings = %+v; the closed recursion must not report broken-chain", report.Errors)
	}
}

// W10: OpenAggregate records the lifecycle first row under the genesis
// value; a second open of the same aggregate is refused; the payload is
// the registered-pending-vectors minimal shape.
func TestOpenAggregateRegistersLifecycleOnly(t *testing.T) {
	db := chainSpine(t)
	if err := OpenAggregate(db, "run-42", "run", "mc", ""); err != nil {
		t.Fatalf("open aggregate: %v", err)
	}
	r := lastChainRow(t, db, "run-42")
	if r.Seq != 1 || r.PrevHash != GenesisHash("run") {
		t.Fatalf("first row = seq %d prev_hash %s; want seq 1 under the run genesis", r.Seq, r.PrevHash)
	}
	var payloadJSON, family string
	if err := db.QueryRow(`SELECT payload_json, payload_family FROM audit_events WHERE aggregate_id = 'run-42'`).Scan(&payloadJSON, &family); err != nil {
		t.Fatalf("read payload: %v", err)
	}
	if family != "aggregate_open" || !strings.Contains(payloadJSON, `"actor":"mc"`) {
		t.Fatalf("payload = %q under family %q; want the minimal aggregate_open shape", payloadJSON, family)
	}
	report, err := VerifyAuditChain(db, "")
	if err != nil {
		t.Fatalf("verify = %v", err)
	}
	if len(report.Errors) != 0 {
		t.Fatalf("verify findings on the open aggregate: %+v", report.Errors)
	}
	if err := OpenAggregate(db, "run-42", "run", "mc", ""); !errors.Is(err, ErrPrevHashMismatch) {
		t.Fatalf("second open = %v; want the prev_hash refusal", err)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM audit_events WHERE aggregate_id = 'run-42'`); n != 1 {
		t.Fatalf("rows after the refused second open = %d; want 1", n)
	}
}

// W11: a spine at the second step's target identity takes S3 through S8; the
// chain table lands and the spine passes the schema check.
func TestMigrateAppliesAuditChainStepFromSecondStepBaseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spine.db")
	db := provisionProductionSpine(t, path, testUUID, b02BaselineIdentity)
	defer db.Close()
	stripAuditChain(t, db)
	stripUsageTables(t, db)
	registry := ProductionMigrationRegistry()
	recordStep(t, db, registry[0])
	recordStep(t, db, registry[1])

	report, err := Migrate(db, registry, CurrentSchemaID)
	if err != nil {
		t.Fatalf("migrate = %v", err)
	}
	if !report.Migrated || len(report.Applied) != 7 || report.Applied[0] != "add-audit-events-chain" || report.Applied[1] != "add-usage-observations" || report.Applied[2] != "migrate-usage-to-v2" || report.Applied[3] != "migrate-activity-to-chain" || report.Applied[4] != "add-identity-foundation" || report.Applied[5] != "add-legacy-actor-sealing" || report.Applied[6] != "add-degraded-grants" {
		t.Fatalf("report = %+v; want the third through ninth steps applied", report)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'audit_events'`); n != 1 {
		t.Fatalf("audit_events present = %d; want 1", n)
	}
	var stamped string
	if err := db.QueryRow(`SELECT schema_id FROM meta WHERE id = 1`).Scan(&stamped); err != nil {
		t.Fatal(err)
	}
	if stamped != CurrentSchemaID {
		t.Fatalf("meta.schema_id = %s; want %s", stamped, CurrentSchemaID)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM audit_events WHERE aggregate_id = 'schema'`); n != 7 {
		t.Fatalf("schema aggregate chain rows = %d; want 7 (S3 starts the chain, S4-S9 extend it)", n)
	}
	if err := CheckSchema(db); err != nil {
		t.Fatalf("post-migration schema check: %v", err)
	}
}

// W12: the mapping row — the step that creates the chain records its own
// migration event in the same transaction, and the event recomputes
// closed.
func TestMigrationStepRecordsClosedChainRow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spine.db")
	db := provisionProductionSpine(t, path, testUUID, preLedgerBaselineIdentity)
	defer db.Close()
	stripLedger(t, db)
	stripAuditChain(t, db)
	stripUsageTables(t, db)
	revertHealthAlertTrigger(t, db)

	registry := ProductionMigrationRegistry()
	report, err := Migrate(db, registry, CurrentSchemaID)
	if err != nil {
		t.Fatalf("migrate = %v", err)
	}
	if len(report.Applied) != 9 {
		t.Fatalf("report = %+v; want the full chain applied", report)
	}

	var aggregateID, eventType, family, prevHash, payloadJSON string
	var seq int
	err = db.QueryRow(`SELECT aggregate_id, seq, event_type, payload_family, prev_hash, payload_json FROM audit_events WHERE aggregate_id = 'schema' AND seq = 1`).
		Scan(&aggregateID, &seq, &eventType, &family, &prevHash, &payloadJSON)
	if err != nil {
		t.Fatalf("read the S3 migration chain row: %v", err)
	}
	if aggregateID != "schema" || seq != 1 || eventType != "schema.migration.step" || family != "migration" {
		t.Fatalf("migration row = (%s, %d, %s, %s); want (schema, 1, schema.migration.step, migration)", aggregateID, seq, eventType, family)
	}
	if prevHash != GenesisHash("schema") {
		t.Fatalf("prev_hash = %s; want the schema genesis", prevHash)
	}
	for _, want := range []string{
		`"step_name":"add-audit-events-chain"`,
		`"from_schema_id":"` + b02BaselineIdentity + `"`,
		`"to_schema_id":"` + b07BaselineIdentity + `"`,
		`"version":"3"`,
	} {
		if !strings.Contains(payloadJSON, want) {
			t.Fatalf("payload %s lacks %s", payloadJSON, want)
		}
	}
	if err := db.QueryRow(`SELECT aggregate_id, seq, payload_json FROM audit_events WHERE aggregate_id = 'schema' AND seq = 2`).
		Scan(&aggregateID, &seq, &payloadJSON); err != nil {
		t.Fatalf("read the S4 migration chain row: %v", err)
	}
	if aggregateID != "schema" || seq != 2 {
		t.Fatalf("S4 migration row = (%s, %d); want (schema, 2)", aggregateID, seq)
	}
	for _, want := range []string{
		`"step_name":"add-usage-observations"`,
		`"from_schema_id":"` + b07BaselineIdentity + `"`,
		`"to_schema_id":"` + c03UsageIdentity + `"`,
		`"version":"4"`,
	} {
		if !strings.Contains(payloadJSON, want) {
			t.Fatalf("S4 payload %s lacks %s", payloadJSON, want)
		}
	}
	if err := db.QueryRow(`SELECT aggregate_id, seq, payload_json FROM audit_events WHERE aggregate_id = 'schema' AND seq = 3`).
		Scan(&aggregateID, &seq, &payloadJSON); err != nil {
		t.Fatalf("read the S5 migration chain row: %v", err)
	}
	if aggregateID != "schema" || seq != 3 {
		t.Fatalf("S5 migration row = (%s, %d); want (schema, 3)", aggregateID, seq)
	}
	for _, want := range []string{
		`"step_name":"migrate-usage-to-v2"`,
		`"from_schema_id":"` + c03UsageIdentity + `"`,
		`"to_schema_id":"` + b09ChainIdentity + `"`,
		`"version":"5"`,
	} {
		if !strings.Contains(payloadJSON, want) {
			t.Fatalf("S5 payload %s lacks %s", payloadJSON, want)
		}
	}
	if err := db.QueryRow(`SELECT aggregate_id, seq, payload_json FROM audit_events WHERE aggregate_id = 'schema' AND seq = 4`).
		Scan(&aggregateID, &seq, &payloadJSON); err != nil {
		t.Fatalf("read the S6 migration chain row: %v", err)
	}
	if aggregateID != "schema" || seq != 4 {
		t.Fatalf("S6 migration row = (%s, %d); want (schema, 4)", aggregateID, seq)
	}
	for _, want := range []string{
		`"step_name":"migrate-activity-to-chain"`,
		`"from_schema_id":"` + b09ChainIdentity + `"`,
		`"to_schema_id":"` + b08MapIdentity + `"`,
		`"version":"6"`,
	} {
		if !strings.Contains(payloadJSON, want) {
			t.Fatalf("S6 payload %s lacks %s", payloadJSON, want)
		}
	}
	if err := db.QueryRow(`SELECT aggregate_id, seq, payload_json FROM audit_events WHERE aggregate_id = 'schema' AND seq = 5`).
		Scan(&aggregateID, &seq, &payloadJSON); err != nil {
		t.Fatalf("read the S7 migration chain row: %v", err)
	}
	if aggregateID != "schema" || seq != 5 {
		t.Fatalf("S7 migration row = (%s, %d); want (schema, 5)", aggregateID, seq)
	}
	for _, want := range []string{
		`"step_name":"add-identity-foundation"`,
		`"from_schema_id":"` + b08MapIdentity + `"`,
		`"to_schema_id":"` + d01FoundationIdentity + `"`,
		`"version":"7"`,
	} {
		if !strings.Contains(payloadJSON, want) {
			t.Fatalf("S7 payload %s lacks %s", payloadJSON, want)
		}
	}
	if err := db.QueryRow(`SELECT aggregate_id, seq, payload_json FROM audit_events WHERE aggregate_id = 'schema' AND seq = 6`).
		Scan(&aggregateID, &seq, &payloadJSON); err != nil {
		t.Fatalf("read the S8 migration chain row: %v", err)
	}
	if aggregateID != "schema" || seq != 6 {
		t.Fatalf("S8 migration row = (%s, %d); want (schema, 6)", aggregateID, seq)
	}
	for _, want := range []string{
		`"step_name":"add-legacy-actor-sealing"`,
		`"from_schema_id":"` + d01FoundationIdentity + `"`,
		`"to_schema_id":"` + d03SealIdentity + `"`,
		`"version":"8"`,
	} {
		if !strings.Contains(payloadJSON, want) {
			t.Fatalf("S8 payload %s lacks %s", payloadJSON, want)
		}
	}
	if err := db.QueryRow(`SELECT aggregate_id, seq, payload_json FROM audit_events WHERE aggregate_id = 'schema' AND seq = 7`).
		Scan(&aggregateID, &seq, &payloadJSON); err != nil {
		t.Fatalf("read the S9 migration chain row: %v", err)
	}
	if aggregateID != "schema" || seq != 7 {
		t.Fatalf("S9 migration row = (%s, %d); want (schema, 7)", aggregateID, seq)
	}
	for _, want := range []string{
		`"step_name":"add-degraded-grants"`,
		`"from_schema_id":"` + d03SealIdentity + `"`,
		`"to_schema_id":"` + CurrentSchemaID + `"`,
		`"version":"9"`,
	} {
		if !strings.Contains(payloadJSON, want) {
			t.Fatalf("S9 payload %s lacks %s", payloadJSON, want)
		}
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM audit_events WHERE aggregate_id = 'schema'`); n != 7 {
		t.Fatalf("schema chain rows = %d; want the S3 through S9 mapping rows (S1 and S2 ran before the table existed)", n)
	}

	verifierReport, err := VerifyAuditChain(db, "")
	if err != nil {
		t.Fatalf("verify = %v", err)
	}
	if len(verifierReport.Errors) != 0 {
		t.Fatalf("the migration row does not recompute closed: %+v", verifierReport.Errors)
	}
}
