package substrate

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

const testUUID = "0123456789abcdef0123456789abcdef"

// provisionProductionSpine builds a spine file carrying the current full
// schema and stamps meta with the given deployment UUID and schema identity.
func provisionProductionSpine(t *testing.T, path, uuid, schemaID string) *sql.DB {
	t.Helper()
	db, err := Open(path)
	if err != nil {
		t.Fatalf("open test spine: %v", err)
	}
	if err := Init(db); err != nil {
		db.Close()
		t.Fatalf("init test spine: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO meta (id, deployment_uuid, schema_id) VALUES (1, ?, ?)`, uuid, schemaID); err != nil {
		db.Close()
		t.Fatalf("stamp test spine meta: %v", err)
	}
	return db
}

// stripLedger removes the migration ledger table, reproducing a spine
// provisioned by a pre-ledger release: every production object present, no
// schema_history.
func stripLedger(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`DROP TABLE schema_history`); err != nil {
		t.Fatalf("drop ledger table: %v", err)
	}
}

func countRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}

// T1 (state 1): an unrecorded step on a pre-ledger spine executes; the step
// statement, its ledger row, its activity event, and the meta stamp all
// appear together (one transaction).
func TestMigrateAppliesUnrecordedStepFromPreLedgerBaseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spine.db")
	db := provisionProductionSpine(t, path, testUUID, preLedgerBaselineIdentity)
	defer db.Close()
	stripLedger(t, db)

	report, err := Migrate(db, ProductionMigrationRegistry(), CurrentSchemaID)
	if err != nil {
		t.Fatalf("migrate = %v", err)
	}
	if !report.Migrated || len(report.Applied) != 1 || report.Applied[0] != "add-schema-history-ledger" {
		t.Fatalf("report = %+v", report)
	}
	step := ProductionMigrationRegistry()[0]
	if n := countRows(t, db, `SELECT COUNT(*) FROM schema_history WHERE version = 1`); n != 1 {
		t.Fatalf("ledger rows = %d", n)
	}
	var checksum, fromID, toID string
	if err := db.QueryRow(`SELECT step_checksum, from_schema_id, to_schema_id FROM schema_history WHERE version = 1`).Scan(&checksum, &fromID, &toID); err != nil {
		t.Fatal(err)
	}
	if checksum != step.Checksum || fromID != preLedgerBaselineIdentity || toID != CurrentSchemaID {
		t.Fatalf("ledger row = (%s, %s, %s); want (%s, %s, %s)", checksum, fromID, toID, step.Checksum, preLedgerBaselineIdentity, CurrentSchemaID)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM activity WHERE kind = 'schema.migration.step' AND subject = ?`, step.Name); n != 1 {
		t.Fatalf("migration events = %d", n)
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

// T2a: a spine already at the target identity is a no-op.
func TestMigrateNoOpWhenSpineAlreadyCurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spine.db")
	db := provisionProductionSpine(t, path, testUUID, CurrentSchemaID)
	defer db.Close()

	report, err := Migrate(db, ProductionMigrationRegistry(), CurrentSchemaID)
	if err != nil {
		t.Fatalf("migrate = %v", err)
	}
	if report.Migrated || len(report.Applied) != 0 {
		t.Fatalf("report = %+v; want no-op", report)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM schema_history`); n != 0 {
		t.Fatalf("ledger rows on current spine = %d", n)
	}
}

// synthetic two-step registry for the skip/conflict/anomaly states.
func syntheticRegistry() []MigrationStep {
	id1 := strings.Repeat("1", 64)
	id2 := strings.Repeat("2", 64)
	id3 := strings.Repeat("3", 64)
	return []MigrationStep{
		newMigrationStep(1, "synthetic-one", id1, id2, "CREATE TABLE mig_one (x INTEGER)"),
		newMigrationStep(2, "synthetic-two", id2, id3, "CREATE TABLE mig_two (x INTEGER)"),
	}
}

// recordStep inserts one ledger row directly (test setup for recorded states).
func recordStep(t *testing.T, db *sql.DB, step MigrationStep) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO schema_history (version, step_name, step_checksum, from_schema_id, to_schema_id, applied_at) VALUES (?, ?, ?, ?, ?, 'now')`,
		step.Version, step.Name, step.Checksum, step.FromSchemaID, step.ToSchemaID); err != nil {
		t.Fatalf("record step row: %v", err)
	}
}

// T2b (state 2): a recorded step whose checksum matches is skipped; only the
// remainder is applied. Re-running is a no-op.
func TestMigrateSkipsRecordedStepAndAppliesRemainder(t *testing.T) {
	registry := syntheticRegistry()
	path := filepath.Join(t.TempDir(), "spine.db")
	db := provisionProductionSpine(t, path, testUUID, registry[0].ToSchemaID)
	defer db.Close()
	if _, err := db.Exec(registry[0].Statement); err != nil {
		t.Fatal(err)
	}
	recordStep(t, db, registry[0])

	report, err := Migrate(db, registry, registry[1].ToSchemaID)
	if err != nil {
		t.Fatalf("migrate = %v", err)
	}
	if !report.Migrated || len(report.Applied) != 1 || report.Applied[0] != "synthetic-two" {
		t.Fatalf("report = %+v; want only synthetic-two applied", report)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM schema_history`); n != 2 {
		t.Fatalf("ledger rows = %d; want 2", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'mig_two'`); n != 1 {
		t.Fatalf("mig_two present = %d", n)
	}

	again, err := Migrate(db, registry, registry[1].ToSchemaID)
	if err != nil {
		t.Fatalf("re-migrate = %v", err)
	}
	if again.Migrated || len(again.Applied) != 0 {
		t.Fatalf("re-run report = %+v; want no-op", again)
	}
}

// T3 (state 3): a recorded step whose checksum no longer matches this
// release's definition stops the migration fail-closed with zero changes.
func TestMigrateFailsClosedOnChecksumConflict(t *testing.T) {
	registry := syntheticRegistry()
	path := filepath.Join(t.TempDir(), "spine.db")
	db := provisionProductionSpine(t, path, testUUID, registry[0].ToSchemaID)
	defer db.Close()
	if _, err := db.Exec(registry[0].Statement); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO schema_history (version, step_name, step_checksum, from_schema_id, to_schema_id, applied_at) VALUES (?, ?, ?, ?, ?, 'now')`,
		registry[0].Version, registry[0].Name, strings.Repeat("e", 64), registry[0].FromSchemaID, registry[0].ToSchemaID); err != nil {
		t.Fatal(err)
	}

	_, err := Migrate(db, registry, registry[1].ToSchemaID)
	if !errors.Is(err, ErrMigrationChecksumConflict) {
		t.Fatalf("migrate error = %v; want ErrMigrationChecksumConflict", err)
	}
	// Zero changes: step two never ran, meta never stamped.
	if n := countRows(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'mig_two'`); n != 0 {
		t.Fatalf("mig_two created despite fail-closed stop: %d", n)
	}
	var stamped string
	if err := db.QueryRow(`SELECT schema_id FROM meta WHERE id = 1`).Scan(&stamped); err != nil {
		t.Fatal(err)
	}
	if stamped != registry[0].ToSchemaID {
		t.Fatalf("meta.schema_id changed to %s despite fail-closed stop", stamped)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM activity WHERE kind = 'schema.migration.step'`); n != 0 {
		t.Fatalf("migration events written despite fail-closed stop: %d", n)
	}
}

// T4 (state 4a): a version gap in the ledger stops the migration for manual
// diagnosis.
func TestMigrateStopsOnVersionGap(t *testing.T) {
	registry := syntheticRegistry()
	path := filepath.Join(t.TempDir(), "spine.db")
	db := provisionProductionSpine(t, path, testUUID, registry[0].ToSchemaID)
	defer db.Close()
	// Step two recorded without step one's row: a gap.
	recordStep(t, db, registry[1])
	_, err := Migrate(db, registry, registry[1].ToSchemaID)
	if !errors.Is(err, ErrMigrationSequenceAnomaly) {
		t.Fatalf("migrate error = %v; want ErrMigrationSequenceAnomaly", err)
	}
}

// T5 (state 4b): a ledger row naming a step this release does not define
// stops the migration for manual diagnosis.
func TestMigrateStopsOnUnknownRecordedStep(t *testing.T) {
	registry := syntheticRegistry()
	path := filepath.Join(t.TempDir(), "spine.db")
	db := provisionProductionSpine(t, path, testUUID, registry[0].FromSchemaID)
	defer db.Close()
	if _, err := db.Exec(
		`INSERT INTO schema_history (version, step_name, step_checksum, from_schema_id, to_schema_id, applied_at) VALUES (?, ?, ?, ?, ?, 'now')`,
		1, "rogue-step", strings.Repeat("d", 64), registry[0].FromSchemaID, registry[0].ToSchemaID); err != nil {
		t.Fatal(err)
	}
	_, err := Migrate(db, registry, registry[1].ToSchemaID)
	if !errors.Is(err, ErrMigrationSequenceAnomaly) {
		t.Fatalf("migrate error = %v; want ErrMigrationSequenceAnomaly", err)
	}
}

// T6: a spine whose identity no registered step starts from has no migration
// path and is refused without touching the database.
func TestMigrateRefusesUnknownIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spine.db")
	unknownID := strings.Repeat("c", 64)
	db := provisionProductionSpine(t, path, testUUID, unknownID)
	defer db.Close()
	stripLedger(t, db)

	_, err := Migrate(db, ProductionMigrationRegistry(), CurrentSchemaID)
	if !errors.Is(err, ErrSchemaUnknown) {
		t.Fatalf("migrate error = %v; want ErrSchemaUnknown", err)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_history'`); n != 0 {
		t.Fatalf("ledger created despite refused migration: %d", n)
	}
	var stamped string
	if err := db.QueryRow(`SELECT schema_id FROM meta WHERE id = 1`).Scan(&stamped); err != nil {
		t.Fatal(err)
	}
	if stamped != unknownID {
		t.Fatalf("meta.schema_id changed to %s despite refused migration", stamped)
	}
}

// The registry discipline that keeps every future identity change honest: a
// release that changed the schema text must carry the step that terminates at
// its own identity, or this test fails the build's test run.
func TestProductionMigrationRegistryTerminatesAtCurrentIdentity(t *testing.T) {
	registry := ProductionMigrationRegistry()
	if err := validateRegistry(registry); err != nil {
		t.Fatalf("registry self-check: %v", err)
	}
	last := registry[len(registry)-1]
	if last.ToSchemaID != CurrentSchemaID {
		t.Fatalf("registry terminal identity %s differs from CurrentSchemaID %s; register the step that crosses this release's boundary", last.ToSchemaID, CurrentSchemaID)
	}
}
