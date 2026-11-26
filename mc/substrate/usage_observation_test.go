package substrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

const (
	c03UUIDA = "11111111-2222-3333-4444-555555555555"
	c03UUIDB = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	c03UUIDC = "00000000-1111-2222-3333-444444444444"
)

// usageTx opens one transaction on the test spine; the usage paths run on
// the caller's transaction exactly as production runs them inside inTx.
func usageTx(t *testing.T, db *sql.DB) *sql.Tx {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin usage transaction: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	return tx
}

func usageNow() time.Time { return time.Date(2026, 11, 26, 8, 30, 12, 0, time.UTC) }

func rawUsageEvent(local, provider, native, family string) UsageEventInput {
	return UsageEventInput{
		LocalRequestID:    local,
		ProviderRequestID: provider,
		NativeEventID:     native,
		UsageFamily:       family,
		RawJSON:           `{"input_tokens":10,"output_tokens":3}`,
		OccurredAt:        usageNow(),
	}
}

func baseObservation() UsageObservationInput {
	return UsageObservationInput{
		LocalRequestID:      c03UUIDA,
		EventSeq:            1,
		SourceVersion:       1,
		Origin:              "local",
		UsageFamily:         "openai-chat",
		ModelID:             "probe-model",
		OutputTokens:        "3",
		InputTokens:         "10",
		ReportKind:          "cumulative",
		Finality:            "interim",
		IdentityProvability: "none",
		OccurredAt:          usageNow(),
	}
}

// U1: the sequence number is assigned on first persistence, max+1 within
// the request slot, and the identity follows the provider/local split.
func TestUsageEventSeqAssignedOnFirstPersistence(t *testing.T) {
	db := chainSpine(t)
	tx := usageTx(t, db)
	ctx := context.Background()
	for i := 1; i <= 3; i++ {
		in := rawUsageEvent(c03UUIDA, "prov-req-1", "", "openai-chat")
		result, err := RecordUsageEvent(ctx, tx, in)
		if err != nil {
			t.Fatalf("record event %d: %v", i, err)
		}
		if result.EventSeq != int64(i) || !result.Created {
			t.Fatalf("event %d = seq %d created %v; want seq %d created", i, result.EventSeq, result.Created, i)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

// U2: a redelivered input addressable through a native event ID or cursor
// re-enters its row and takes no new number; an unaddressable delivery
// numbers normally.
func TestUsageEventRedeliveryReentersWithoutRenumbering(t *testing.T) {
	db := chainSpine(t)
	tx := usageTx(t, db)
	ctx := context.Background()
	first, err := RecordUsageEvent(ctx, tx, rawUsageEvent(c03UUIDA, "prov-req-1", "native-evt-1", "openai-chat"))
	if err != nil {
		t.Fatalf("first record: %v", err)
	}
	if !first.Created || first.EventSeq != 1 {
		t.Fatalf("first = %+v; want created at seq 1", first)
	}
	again, err := RecordUsageEvent(ctx, tx, rawUsageEvent(c03UUIDA, "prov-req-1", "native-evt-1", "openai-chat"))
	if err != nil {
		t.Fatalf("redelivery: %v", err)
	}
	if again.Created || again.EventSeq != first.EventSeq {
		t.Fatalf("redelivery = %+v; want re-entry at seq %d", again, first.EventSeq)
	}
	next, err := RecordUsageEvent(ctx, tx, rawUsageEvent(c03UUIDA, "prov-req-1", "", "openai-chat"))
	if err != nil {
		t.Fatalf("unaddressable delivery: %v", err)
	}
	if !next.Created || next.EventSeq != 2 {
		t.Fatalf("unaddressable = %+v; want a new number at seq 2", next)
	}
}

// U3: phase two before phase one is refused: an observation may not exist
// without its raw event.
func TestUsageObservationRequiresRawEvent(t *testing.T) {
	db := chainSpine(t)
	tx := usageTx(t, db)
	_, err := RecordUsageObservation(context.Background(), tx, baseObservation())
	if !errors.Is(err, ErrUsageEventUnknown) {
		t.Fatalf("error = %v; want ErrUsageEventUnknown", err)
	}
}

// U4: the identity-origin red line: a provider-native claim without a
// provider request ID is refused, and a local origin carrying one is
// refused.
func TestUsageObservationOriginRedLine(t *testing.T) {
	db := chainSpine(t)
	tx := usageTx(t, db)
	ctx := context.Background()
	if _, err := RecordUsageEvent(ctx, tx, rawUsageEvent(c03UUIDA, "", "", "openai-chat")); err != nil {
		t.Fatalf("record raw event: %v", err)
	}
	providerClaim := baseObservation()
	providerClaim.Origin = "provider"
	if _, err := RecordUsageObservation(ctx, tx, providerClaim); !errors.Is(err, ErrUsageOriginClaim) {
		t.Fatalf("provider claim without provider id: error = %v; want ErrUsageOriginClaim", err)
	}
	localWithProviderID := baseObservation()
	localWithProviderID.ProviderRequestID = "prov-req-1"
	if _, err := RecordUsageObservation(ctx, tx, localWithProviderID); !errors.Is(err, ErrUsageOriginClaim) {
		t.Fatalf("local origin carrying provider id: error = %v; want ErrUsageOriginClaim", err)
	}
}

// U5: a redelivered same-version claim is idempotent: same row, same chain
// evidence, no second append.
func TestUsageObservationIdempotentReredelivery(t *testing.T) {
	db := chainSpine(t)
	tx := usageTx(t, db)
	ctx := context.Background()
	if _, err := RecordUsageEvent(ctx, tx, rawUsageEvent(c03UUIDA, "", "", "openai-chat")); err != nil {
		t.Fatalf("record raw event: %v", err)
	}
	in := baseObservation()
	in.IdentityProvability = "cursor"
	first, err := RecordUsageObservation(ctx, tx, in)
	if err != nil {
		t.Fatalf("first observation: %v", err)
	}
	if !first.Created || first.State != "reported" {
		t.Fatalf("first = %+v; want created and reported", first)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit first observation: %v", err)
	}
	replay := usageTx(t, db)
	again, err := RecordUsageObservation(ctx, replay, in)
	if err != nil {
		t.Fatalf("redelivery: %v", err)
	}
	if again.Created || again.ObservationID != first.ObservationID || again.State != first.State {
		t.Fatalf("redelivery = %+v; want re-entry %+v", again, first)
	}
	if err := replay.Commit(); err != nil {
		t.Fatalf("commit redelivery: %v", err)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM usage_observations`); n != 1 {
		t.Fatalf("observation rows = %d; want 1", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM audit_events WHERE aggregate_id = 'usage'`); n != 2 {
		t.Fatalf("usage chain rows = %d; want 2 (open + one observation)", n)
	}
}

// U6: a content conflict at a fixed version is refused, never
// last-write-wins: the stored row keeps its content and digest.
func TestUsageObservationVersionCollisionRefused(t *testing.T) {
	db := chainSpine(t)
	tx := usageTx(t, db)
	ctx := context.Background()
	if _, err := RecordUsageEvent(ctx, tx, rawUsageEvent(c03UUIDA, "", "", "openai-chat")); err != nil {
		t.Fatalf("record raw event: %v", err)
	}
	in := baseObservation()
	if _, err := RecordUsageObservation(ctx, tx, in); err != nil {
		t.Fatalf("first observation: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit first observation: %v", err)
	}
	conflictTx := usageTx(t, db)
	conflict := in
	conflict.OutputTokens = "99"
	if _, err := RecordUsageObservation(ctx, conflictTx, conflict); !errors.Is(err, ErrUsageVersionCollision) {
		t.Fatalf("collision = %v; want ErrUsageVersionCollision", err)
	}
	var stored string
	if err := db.QueryRow(`SELECT output_tokens FROM usage_observations WHERE source_version = 1`).Scan(&stored); err != nil {
		t.Fatalf("read stored content: %v", err)
	}
	if stored != "3" {
		t.Fatalf("stored content = %s; want the original 3", stored)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM usage_observations`); n != 1 {
		t.Fatalf("observation rows = %d; want 1", n)
	}
}

// U7: events whose sameness cannot be proven never enter the reported
// channel: provability=none lands in the unknown state.
func TestUsageObservationNoneProvabilityGoesUnknown(t *testing.T) {
	db := chainSpine(t)
	tx := usageTx(t, db)
	ctx := context.Background()
	if _, err := RecordUsageEvent(ctx, tx, rawUsageEvent(c03UUIDA, "", "", "openai-chat")); err != nil {
		t.Fatalf("record raw event: %v", err)
	}
	result, err := RecordUsageObservation(ctx, tx, baseObservation())
	if err != nil {
		t.Fatalf("observation: %v", err)
	}
	if result.State != "unknown" {
		t.Fatalf("state = %s; want unknown", result.State)
	}
}

// U8: version increments supersede within the slot and stale versions stay;
// a second conflicting terminal claim is flagged reconciliation, while a
// converging terminal claim stays reported.
func TestUsageObservationSupersedesAndConflictingFinal(t *testing.T) {
	db := chainSpine(t)
	tx := usageTx(t, db)
	ctx := context.Background()
	if _, err := RecordUsageEvent(ctx, tx, rawUsageEvent(c03UUIDA, "", "", "anthropic")); err != nil {
		t.Fatalf("record raw event: %v", err)
	}
	base := baseObservation()
	base.UsageFamily = "anthropic"

	v1 := base
	v1.SourceVersion = 1
	v1.OutputTokens = "5"
	v1.IdentityProvability = "cursor"
	if _, err := RecordUsageObservation(ctx, tx, v1); err != nil {
		t.Fatalf("v1: %v", err)
	}
	v2 := v1
	v2.SourceVersion = 2
	v2.Finality = "final"
	v2.OutputTokens = "10"
	if result, err := RecordUsageObservation(ctx, tx, v2); err != nil {
		t.Fatalf("v2: %v", err)
	} else if result.State != "reported" {
		t.Fatalf("v2 state = %s; want reported", result.State)
	}
	conflictingFinal := v2
	conflictingFinal.SourceVersion = 3
	conflictingFinal.OutputTokens = "12"
	result, err := RecordUsageObservation(ctx, tx, conflictingFinal)
	if err != nil {
		t.Fatalf("conflicting final: %v", err)
	}
	if result.State != "reconciliation" {
		t.Fatalf("conflicting final state = %s; want reconciliation", result.State)
	}
	convergingFinal := v2
	convergingFinal.SourceVersion = 4
	convergingFinal.OutputTokens = "12"
	if result, err := RecordUsageObservation(ctx, tx, convergingFinal); err != nil {
		t.Fatalf("converging final: %v", err)
	} else if result.State != "reported" {
		t.Fatalf("converging final state = %s; want reported", result.State)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM usage_observations`); n != 4 {
		t.Fatalf("observation rows = %d; want 4 (all versions kept)", n)
	}
	var v2Stored string
	if err := db.QueryRow(`SELECT output_tokens FROM usage_observations WHERE source_version = 2`).Scan(&v2Stored); err != nil {
		t.Fatalf("read v2 content: %v", err)
	}
	if v2Stored != "10" {
		t.Fatalf("v2 content = %s; want the unmodified 10", v2Stored)
	}
}

// U9: supersedes must reference an existing observation.
func TestUsageObservationSupersedesUnknownRefused(t *testing.T) {
	db := chainSpine(t)
	tx := usageTx(t, db)
	ctx := context.Background()
	if _, err := RecordUsageEvent(ctx, tx, rawUsageEvent(c03UUIDA, "", "", "openai-chat")); err != nil {
		t.Fatalf("record raw event: %v", err)
	}
	in := baseObservation()
	in.SupersedesObservationID = "0000000000000000000000000000000000000000000000000000000000000000"
	if _, err := RecordUsageObservation(ctx, tx, in); !errors.Is(err, ErrUsageSupersedesUnknown) {
		t.Fatalf("error = %v; want ErrUsageSupersedesUnknown", err)
	}
}

// U10: the usage tables keep the append-only discipline.
func TestUsageTablesAreAppendOnly(t *testing.T) {
	db := chainSpine(t)
	tx := usageTx(t, db)
	ctx := context.Background()
	if _, err := RecordUsageEvent(ctx, tx, rawUsageEvent(c03UUIDA, "", "", "openai-chat")); err != nil {
		t.Fatalf("record raw event: %v", err)
	}
	in := baseObservation()
	in.IdentityProvability = "cursor"
	if _, err := RecordUsageObservation(ctx, tx, in); err != nil {
		t.Fatalf("observation: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := db.Exec(`UPDATE usage_events SET raw_json = ? WHERE request_identity = ?`, `{}`, c03UUIDA); err == nil || !isConstraintAbort(err) {
		t.Fatalf("usage_events UPDATE did not abort: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM usage_events WHERE request_identity = ?`, c03UUIDA); err == nil || !isConstraintAbort(err) {
		t.Fatalf("usage_events DELETE did not abort: %v", err)
	}
	if _, err := db.Exec(`UPDATE usage_observations SET state = ? WHERE local_request_id = ?`, "reported", c03UUIDA); err == nil || !isConstraintAbort(err) {
		t.Fatalf("usage_observations UPDATE did not abort: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM usage_observations WHERE local_request_id = ?`, c03UUIDA); err == nil || !isConstraintAbort(err) {
		t.Fatalf("usage_observations DELETE did not abort: %v", err)
	}
}

// U11: the usage aggregate opens with its first observation, in the same
// transaction, and the chain verifies clean afterwards.
func TestUsageAggregateOpensWithFirstObservation(t *testing.T) {
	db := chainSpine(t)
	tx := usageTx(t, db)
	ctx := context.Background()
	if _, err := RecordUsageEvent(ctx, tx, rawUsageEvent(c03UUIDA, "", "", "openai-chat")); err != nil {
		t.Fatalf("record raw event: %v", err)
	}
	in := baseObservation()
	in.IdentityProvability = "cursor"
	if _, err := RecordUsageObservation(ctx, tx, in); err != nil {
		t.Fatalf("observation: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	var openType, openPrev string
	if err := db.QueryRow(`SELECT event_type, prev_hash FROM audit_events WHERE aggregate_id = 'usage' AND seq = 1`).Scan(&openType, &openPrev); err != nil {
		t.Fatalf("read first usage chain row: %v", err)
	}
	if openType != "aggregate_open" {
		t.Fatalf("first usage chain row = %s; want aggregate_open", openType)
	}
	if openPrev != GenesisHash("usage") {
		t.Fatalf("open prev_hash = %s; want the usage genesis", openPrev)
	}
	var obsType string
	if err := db.QueryRow(`SELECT event_type FROM audit_events WHERE aggregate_id = 'usage' AND seq = 2`).Scan(&obsType); err != nil {
		t.Fatalf("read second usage chain row: %v", err)
	}
	if obsType != "usage.observation" {
		t.Fatalf("second usage chain row = %s; want usage.observation", obsType)
	}
	report, err := VerifyAuditChain(db, "usage")
	if err != nil {
		t.Fatalf("verify usage chain: %v", err)
	}
	if len(report.Errors) != 0 {
		t.Fatalf("usage chain findings = %+v; want none", report.Errors)
	}
	if len(report.Heads) != 1 || report.Heads[0].SeqMax != 2 {
		t.Fatalf("usage chain heads = %+v; want one head at seq 2", report.Heads)
	}
}

// U12: the state and its chain evidence commit or roll back together: a
// rolled-back phase two leaves no observation and no chain row.
func TestUsageObservationRollsBackAtomically(t *testing.T) {
	db := chainSpine(t)
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	ctx := context.Background()
	if _, err := RecordUsageEvent(ctx, tx, rawUsageEvent(c03UUIDA, "", "", "openai-chat")); err != nil {
		t.Fatalf("record raw event: %v", err)
	}
	in := baseObservation()
	in.IdentityProvability = "cursor"
	if _, err := RecordUsageObservation(ctx, tx, in); err != nil {
		t.Fatalf("observation: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM usage_observations`); n != 0 {
		t.Fatalf("observation rows after rollback = %d; want 0", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM audit_events WHERE aggregate_id = 'usage'`); n != 0 {
		t.Fatalf("usage chain rows after rollback = %d; want 0", n)
	}
}

// U13: the fourth and fifth migration steps cross a B07-baseline spine to
// this release: the usage tables land and the backfill step proves itself,
// the ledger and the stamp follow.
func TestMigrateAppliesUsageStepFromB07Baseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spine.db")
	db := provisionProductionSpine(t, path, testUUID, b07BaselineIdentity)
	defer db.Close()
	stripUsageTables(t, db)
	registry := ProductionMigrationRegistry()
	recordStep(t, db, registry[0])
	recordStep(t, db, registry[1])
	recordStep(t, db, registry[2])

	report, err := Migrate(db, registry, CurrentSchemaID)
	if err != nil {
		t.Fatalf("migrate = %v", err)
	}
	if !report.Migrated || len(report.Applied) != 6 || report.Applied[0] != "add-usage-observations" || report.Applied[1] != "migrate-usage-to-v2" || report.Applied[2] != "migrate-activity-to-chain" || report.Applied[3] != "add-identity-foundation" || report.Applied[4] != "add-legacy-actor-sealing" || report.Applied[5] != "add-degraded-grants" {
		t.Fatalf("report = %+v; want the fourth through ninth steps applied", report)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name IN ('usage_events','usage_observations','usage')`); n != 3 {
		t.Fatalf("usage tables present = %d; want 3", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM schema_history`); n != 9 {
		t.Fatalf("ledger rows = %d; want 9", n)
	}
	var stamped string
	if err := db.QueryRow(`SELECT schema_id FROM meta WHERE id = 1`).Scan(&stamped); err != nil {
		t.Fatal(err)
	}
	if stamped != CurrentSchemaID {
		t.Fatalf("meta.schema_id = %s; want %s", stamped, CurrentSchemaID)
	}
	if err := CheckSchema(db); err != nil {
		t.Fatalf("post-migration schema check: %v", err)
	}
}

// U14: the step's statement text and the reference schema carry identical
// usage object definitions, so a migrated spine hashes like a fresh one.
func TestUsageDDLMatchesReferenceSchemaByteForByte(t *testing.T) {
	fresh := chainSpine(t)
	migratedPath := filepath.Join(t.TempDir(), "spine.db")
	db := provisionProductionSpine(t, migratedPath, testUUID, b07BaselineIdentity)
	stripUsageTables(t, db)
	registry := ProductionMigrationRegistry()
	recordStep(t, db, registry[0])
	recordStep(t, db, registry[1])
	recordStep(t, db, registry[2])
	if _, err := Migrate(db, registry, CurrentSchemaID); err != nil {
		t.Fatalf("migrate = %v", err)
	}
	freshRows := usageObjectRows(t, fresh)
	migratedRows := usageObjectRows(t, db)
	if len(freshRows) == 0 || len(freshRows) != len(migratedRows) {
		t.Fatalf("usage object counts = %d vs %d; want equal and nonzero", len(freshRows), len(migratedRows))
	}
	for i := range freshRows {
		if freshRows[i] != migratedRows[i] {
			t.Fatalf("usage object %d diverges:\n fresh: %s\nmigrated: %s", i, freshRows[i], migratedRows[i])
		}
	}
}

// usageObjectRows reads the usage object definitions out of a spine's
// schema catalog, ordered, as compare-able strings.
func usageObjectRows(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`SELECT name, sql FROM sqlite_master WHERE name LIKE 'usage%' AND type IN ('table','index','trigger') ORDER BY type, name`)
	if err != nil {
		t.Fatalf("query usage objects: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name, definition string
		if err := rows.Scan(&name, &definition); err != nil {
			t.Fatalf("scan usage object: %v", err)
		}
		out = append(out, fmt.Sprintf("%s|%s", name, definition))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate usage objects: %v", err)
	}
	return out
}

// stripUsageTables removes the usage objects, reproducing a spine
// provisioned by a release before the fourth step existed.
func stripUsageTables(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS usage_observations_no_delete`); err != nil {
		t.Fatalf("strip usage_observations_no_delete: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS usage_observations_no_update`); err != nil {
		t.Fatalf("strip usage_observations_no_update: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS usage_observations_local`); err != nil {
		t.Fatalf("strip usage_observations_local: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS usage_observations`); err != nil {
		t.Fatalf("strip usage_observations: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS usage_events_no_delete`); err != nil {
		t.Fatalf("strip usage_events_no_delete: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS usage_events_no_update`); err != nil {
		t.Fatalf("strip usage_events_no_update: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS usage_events_local`); err != nil {
		t.Fatalf("strip usage_events_local: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS usage_events_native_event`); err != nil {
		t.Fatalf("strip usage_events_native_event: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS usage_events`); err != nil {
		t.Fatalf("strip usage_events: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS usage_local`); err != nil {
		t.Fatalf("strip usage_local: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS usage_request`); err != nil {
		t.Fatalf("strip usage_request: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS usage`); err != nil {
		t.Fatalf("strip usage: %v", err)
	}
	stripActivityChainMap(t, db)
	stripIdentityFoundation(t, db)
	stripLegacyActorSealing(t, db)
	stripDegradedGrants(t, db)
}
