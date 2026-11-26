package substrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// b09hex derives a stable 64-char lowercase hex value from a label, for
// seeding observation ids and digests that satisfy the schema fences.
func b09hex(label string) string {
	sum := sha256.Sum256([]byte(label))
	return hex.EncodeToString(sum[:])
}

const b09Time = "2026-11-26T08:30:12.000000Z"

// seedUsageSlot persists one raw event plus observation rows at the given
// versions, directly in SQL: this is a migration test, so the pre-migration
// v1 state is constructed by hand rather than through the write path (which
// would also touch the v2 table that does not exist yet at the c03 identity).
func seedUsageSlot(t *testing.T, db *sql.DB, identity, local, model, family string, seq int64, versions []string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO usage_events
		(request_identity, identity_origin, local_request_id, provider_request_id,
		 native_event_id, event_seq, usage_family, raw_json, occurred_at, recorded_at)
		VALUES (?, ?, ?, NULL, NULL, ?, ?, '{}', ?, ?)`,
		identity, "local", local, seq, family, b09Time, b09Time); err != nil {
		t.Fatalf("seed raw event for %s: %v", identity, err)
	}
	for _, version := range versions {
		seqKey := strconv.FormatInt(seq, 10)
		obsID := b09hex(identity + ":" + seqKey + ":" + version)
		digest := b09hex("digest:" + identity + ":" + seqKey + ":" + version)
		if _, err := db.Exec(`INSERT INTO usage_observations
			(observation_id, request_identity, local_request_id, provider_request_id,
			 event_seq, source_version, origin, usage_family, model_id,
			 output_tokens, input_tokens, report_kind, finality, identity_provability,
			 state, payload_digest, supersedes_observation_id, occurred_at, recorded_at)
		 VALUES (?, ?, ?, NULL, ?, ?, 'local', ?, ?, '3', '10', 'cumulative', 'final', 'cursor',
			 'reported', ?, NULL, ?, ?)`,
			obsID, identity, local, seq, version, family, model, digest, b09Time, b09Time); err != nil {
			t.Fatalf("seed observation %s v%s: %v", identity, version, err)
		}
	}
}

// b09Spine provisions a spine at the c03 identity: the v1 usage tables are
// present (S4 applied) but the v2 usage table is not (S5 not yet run), with
// the ledger recording S1-S4.
func b09Spine(t *testing.T) *sql.DB {
	t.Helper()
	db := provisionProductionSpine(t, filepath.Join(t.TempDir(), "spine.db"), testUUID, c03UsageIdentity)
	t.Cleanup(func() { db.Close() })
	stripUsageV2(t, db)
	registry := ProductionMigrationRegistry()
	recordStep(t, db, registry[0])
	recordStep(t, db, registry[1])
	recordStep(t, db, registry[2])
	recordStep(t, db, registry[3])
	return db
}

func stripUsageV2(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`DROP INDEX IF EXISTS usage_local`); err != nil {
		t.Fatalf("strip usage local index: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS usage_request`); err != nil {
		t.Fatalf("strip usage request index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS usage`); err != nil {
		t.Fatalf("strip usage table: %v", err)
	}
}

func usageRowCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	return countRows(t, db, `SELECT COUNT(*) FROM usage`)
}

func usageRow(t *testing.T, db *sql.DB, identity string) map[string]string {
	t.Helper()
	row := map[string]string{}
	var model, output, version, state, digest, reportID string
	err := db.QueryRow(`SELECT source_report_id, model_id, output_tokens, source_version, state, payload_digest
		FROM usage WHERE request_identity = ?`, identity).
		Scan(&reportID, &model, &output, &version, &state, &digest)
	if err != nil {
		t.Fatalf("read usage row for %s: %v", identity, err)
	}
	row["source_report_id"] = reportID
	row["model_id"] = model
	row["output_tokens"] = output
	row["source_version"] = version
	row["state"] = state
	row["payload_digest"] = digest
	return row
}

// M1: the backfill promotes each slot's max-version observation and only
// those; a raw event with no observation is never promoted.
func TestUsageBackfillPromotesMaxVersionOnly(t *testing.T) {
	db := b09Spine(t)
	// Slot A carries two versions; the backfill must keep the max (v2).
	seedUsageSlot(t, db, "id-a", "local-a", "model-a", "openai-chat", 1, []string{"1", "2"})
	// Slot B carries a single version.
	seedUsageSlot(t, db, "id-b", "local-b", "model-b", "anthropic", 1, []string{"1"})
	// Slot C is raw-only: an event with no observation.
	if _, err := db.Exec(`INSERT INTO usage_events
		(request_identity, identity_origin, local_request_id, provider_request_id,
		 native_event_id, event_seq, usage_family, raw_json, occurred_at, recorded_at)
		VALUES ('id-c', 'local', 'local-c', NULL, NULL, 1, 'google', '{}', ?, ?)`, b09Time, b09Time); err != nil {
		t.Fatalf("seed raw-only event: %v", err)
	}

	registry := ProductionMigrationRegistry()
	report, err := Migrate(db, registry, CurrentSchemaID)
	if err != nil {
		t.Fatalf("migrate = %v", err)
	}
	if !report.Migrated || len(report.Applied) != 1 || report.Applied[0] != "migrate-usage-to-v2" {
		t.Fatalf("report = %+v; want only the fifth step applied", report)
	}
	if n := usageRowCount(t, db); n != 2 {
		t.Fatalf("usage rows = %d; want 2 (slots A and B; C is raw-only)", n)
	}
	a := usageRow(t, db, "id-a")
	if a["source_version"] != "2" || a["model_id"] != "model-a" {
		t.Fatalf("slot A promoted row = %v; want the max version (2)", a)
	}
	b := usageRow(t, db, "id-b")
	if b["source_version"] != "1" || b["model_id"] != "model-b" {
		t.Fatalf("slot B promoted row = %v; want its only version", b)
	}
	var c int
	if err := db.QueryRow(`SELECT COUNT(*) FROM usage WHERE request_identity = 'id-c'`).Scan(&c); err != nil {
		t.Fatal(err)
	}
	if c != 0 {
		t.Fatalf("raw-only slot C promoted %d rows; want 0", c)
	}
}

// M2: the same-transaction proof passes on a correct backfill — count,
// multiset digest, and both anti-join directions.
func TestUsageBackfillProofPasses(t *testing.T) {
	db := b09Spine(t)
	seedUsageSlot(t, db, "id-a", "local-a", "model-a", "openai-chat", 1, []string{"1", "2", "3"})
	seedUsageSlot(t, db, "id-b", "local-b", "model-b", "google", 2, []string{"1"})
	registry := ProductionMigrationRegistry()
	if _, err := Migrate(db, registry, CurrentSchemaID); err != nil {
		t.Fatalf("migrate = %v", err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := VerifyUsageBackfill(tx); err != nil {
		t.Fatalf("proof over a correct backfill = %v; want nil", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
}

// M3: a backfill the proof cannot stand behind refuses the whole step: the
// v2 table is absent after rollback, the ledger carries no fifth row, the
// stamp stays at the c03 identity, and the v1 originals are untouched.
func TestUsageBackfillProofFailureRollsBack(t *testing.T) {
	db := b09Spine(t)
	seedUsageSlot(t, db, "id-a", "local-a", "model-a", "openai-chat", 1, []string{"1", "2"})
	seedCountBefore := countRows(t, db, `SELECT COUNT(*) FROM usage_observations`)

	// A registry whose fifth step creates the v2 table but backfills nothing:
	// the proof's count check must fail and roll the step back.
	registry := ProductionMigrationRegistry()
	ddlOnly, _, _ := strings.Cut(registry[4].Statement, "INSERT INTO usage")
	bad := newMigrationStep(5, "migrate-usage-to-v2", c03UsageIdentity, CurrentSchemaID, ddlOnly)
	bad.Verify = VerifyUsageBackfill
	registry[4] = bad

	_, err := Migrate(db, registry, CurrentSchemaID)
	if err == nil || !errors.Is(err, ErrBackfillProof) {
		t.Fatalf("migrate = %v; want the backfill proof refusal", err)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'usage'`); n != 0 {
		t.Fatalf("usage table present after rollback = %d; want 0 (the DDL rolled back too)", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM schema_history WHERE version = 5`); n != 0 {
		t.Fatalf("ledger carries a fifth row after rollback; want none")
	}
	var stamped string
	if err := db.QueryRow(`SELECT schema_id FROM meta WHERE id = 1`).Scan(&stamped); err != nil {
		t.Fatal(err)
	}
	if stamped != c03UsageIdentity {
		t.Fatalf("meta.schema_id = %s after rollback; want the c03 identity untouched", stamped)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM usage_observations`); n != seedCountBefore {
		t.Fatalf("v1 observations after rollback = %d; want %d (originals never touched)", n, seedCountBefore)
	}
}

// M4: a spine whose ledger is complete at this release is a no-op: the
// recorded fifth step is skipped and the v2 rows are not re-backfilled.
func TestUsageMigrationIsIdempotentOnReRun(t *testing.T) {
	db := b09Spine(t)
	seedUsageSlot(t, db, "id-a", "local-a", "model-a", "openai-chat", 1, []string{"1", "2"})
	registry := ProductionMigrationRegistry()
	first, err := Migrate(db, registry, CurrentSchemaID)
	if err != nil || !first.Migrated {
		t.Fatalf("first migrate = (%+v, %v)", first, err)
	}
	again, err := Migrate(db, registry, CurrentSchemaID)
	if err != nil {
		t.Fatalf("re-run migrate = %v", err)
	}
	if again.Migrated || len(again.Applied) != 0 {
		t.Fatalf("re-run report = %+v; want a no-op (the fifth step is recorded)", again)
	}
	if n := usageRowCount(t, db); n != 1 {
		t.Fatalf("usage rows after re-run = %d; want 1 (not re-backfilled)", n)
	}
}

// M5: the fifth step's usage object definitions match the reference schema
// byte for byte, so a migrated spine hashes like a fresh one.
func TestUsageV2DDLMatchesReferenceSchemaByteForByte(t *testing.T) {
	fresh := chainSpine(t)
	db := b09Spine(t)
	seedUsageSlot(t, db, "id-a", "local-a", "model-a", "openai-chat", 1, []string{"1"})
	registry := ProductionMigrationRegistry()
	if _, err := Migrate(db, registry, CurrentSchemaID); err != nil {
		t.Fatalf("migrate = %v", err)
	}
	readObjects := func(d *sql.DB) []string {
		rows, err := d.Query(`SELECT name, sql FROM sqlite_master WHERE name = 'usage' OR name IN ('usage_request','usage_local') ORDER BY type, name`)
		if err != nil {
			t.Fatalf("query usage v2 objects: %v", err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var name, definition string
			if err := rows.Scan(&name, &definition); err != nil {
				t.Fatalf("scan usage v2 object: %v", err)
			}
			out = append(out, name+"|"+definition)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate usage v2 objects: %v", err)
		}
		return out
	}
	freshRows, migratedRows := readObjects(fresh), readObjects(db)
	if len(freshRows) == 0 || len(freshRows) != len(migratedRows) {
		t.Fatalf("usage v2 object counts = %d vs %d; want equal and nonzero", len(freshRows), len(migratedRows))
	}
	for i := range freshRows {
		if freshRows[i] != migratedRows[i] {
			t.Fatalf("usage v2 object %d diverges:\n fresh: %s\nmigrated: %s", i, freshRows[i], migratedRows[i])
		}
	}
}

// M6: steady-state writes keep the v2 projection current in the same
// transaction as the v1 evidence: a new version replaces the slot's row, an
// idempotent replay leaves it, and a rolled-back write leaves both planes.
func TestUsageCurrentRowTracksObservations(t *testing.T) {
	db := chainSpine(t)
	ctx := context.Background()
	const identity = "00000000-0000-4000-8000-000000000001"
	tx := usageTx(t, db)
	if _, err := RecordUsageEvent(ctx, tx, UsageEventInput{
		LocalRequestID: identity, UsageFamily: "openai-chat",
		RawJSON: `{"input_tokens":10,"output_tokens":3}`, OccurredAt: usageNow(),
	}); err != nil {
		t.Fatalf("record raw event: %v", err)
	}
	base := UsageObservationInput{
		LocalRequestID: identity, EventSeq: 1, SourceVersion: 1,
		Origin: "local", UsageFamily: "openai-chat", ModelID: "model-live",
		OutputTokens: "3", InputTokens: "10", ReportKind: "cumulative",
		Finality: "interim", IdentityProvability: "cursor", OccurredAt: usageNow(),
	}
	first, err := RecordUsageObservation(ctx, tx, base)
	if err != nil {
		t.Fatalf("first observation: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if got := usageRow(t, db, identity)["source_report_id"]; got != first.ObservationID {
		t.Fatalf("v2 row = %s; want the first observation %s", got, first.ObservationID)
	}
	// A newer version replaces the slot's current row.
	next := base
	next.SourceVersion = 2
	next.OutputTokens = "9"
	next.Finality = "final"
	tx2 := usageTx(t, db)
	second, err := RecordUsageObservation(ctx, tx2, next)
	if err != nil {
		t.Fatalf("second observation: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if n := usageRowCount(t, db); n != 1 {
		t.Fatalf("usage rows after the new version = %d; want 1 (the slot keeps one current row)", n)
	}
	row := usageRow(t, db, identity)
	if row["source_report_id"] != second.ObservationID || row["output_tokens"] != "9" || row["source_version"] != "2" {
		t.Fatalf("v2 row after version 2 = %v; want the newer observation", row)
	}
	// A rolled-back write leaves both planes untouched.
	tx3, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	revert := base
	revert.SourceVersion = 3
	revert.OutputTokens = "1"
	if _, err := RecordUsageObservation(ctx, tx3, revert); err != nil {
		t.Fatalf("third observation: %v", err)
	}
	if err := tx3.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if row := usageRow(t, db, identity); row["source_report_id"] != second.ObservationID || row["output_tokens"] != "9" {
		t.Fatalf("v2 row after rollback = %v; want the pre-rollback current row", row)
	}
}
