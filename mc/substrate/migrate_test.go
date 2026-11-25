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

// revertHealthAlertTrigger restores the trigger object to the text the
// baseline carried before the rename, reproducing a spine a previous release
// left behind.
func revertHealthAlertTrigger(t *testing.T, db *sql.DB) {
	t.Helper()
	old := strings.Replace(renameHealthAlertBodySQL, "S-Horizon health:", "Culler health:", 1)
	if _, err := db.Exec(old); err != nil {
		t.Fatalf("revert health alert trigger: %v", err)
	}
}

func healthAlertTriggerSQL(t *testing.T, db *sql.DB) string {
	t.Helper()
	var definition string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'trigger' AND name = 'activity_health_alert_outbox'`).Scan(&definition); err != nil {
		t.Fatalf("read health alert trigger definition: %v", err)
	}
	return definition
}

// T1 (state 1, full chain): on a pre-ledger spine neither step is recorded;
// both execute in version order and the spine lands at this release's
// identity with its ledger complete.
func TestMigrateAppliesUnrecordedStepsFromPreLedgerBaseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spine.db")
	db := provisionProductionSpine(t, path, testUUID, preLedgerBaselineIdentity)
	defer db.Close()
	stripLedger(t, db)
	revertHealthAlertTrigger(t, db)

	registry := ProductionMigrationRegistry()
	report, err := Migrate(db, registry, CurrentSchemaID)
	if err != nil {
		t.Fatalf("migrate = %v", err)
	}
	if !report.Migrated || len(report.Applied) != 2 || report.Applied[0] != "add-schema-history-ledger" || report.Applied[1] != "rename-health-alert-body" {
		t.Fatalf("report = %+v", report)
	}
	step1, step2 := registry[0], registry[1]
	var checksum1, from1, to1 string
	if err := db.QueryRow(`SELECT step_checksum, from_schema_id, to_schema_id FROM schema_history WHERE version = 1`).Scan(&checksum1, &from1, &to1); err != nil {
		t.Fatal(err)
	}
	if checksum1 != step1.Checksum || from1 != preLedgerBaselineIdentity || to1 != b01BaselineIdentity {
		t.Fatalf("ledger row 1 = (%s, %s, %s); want (%s, %s, %s)", checksum1, from1, to1, step1.Checksum, preLedgerBaselineIdentity, b01BaselineIdentity)
	}
	var checksum2, from2, to2 string
	if err := db.QueryRow(`SELECT step_checksum, from_schema_id, to_schema_id FROM schema_history WHERE version = 2`).Scan(&checksum2, &from2, &to2); err != nil {
		t.Fatal(err)
	}
	if checksum2 != step2.Checksum || from2 != b01BaselineIdentity || to2 != CurrentSchemaID {
		t.Fatalf("ledger row 2 = (%s, %s, %s); want (%s, %s, %s)", checksum2, from2, to2, step2.Checksum, b01BaselineIdentity, CurrentSchemaID)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM activity WHERE kind = 'schema.migration.step'`); n != 2 {
		t.Fatalf("migration events = %d; want 2", n)
	}
	var stamped string
	if err := db.QueryRow(`SELECT schema_id FROM meta WHERE id = 1`).Scan(&stamped); err != nil {
		t.Fatal(err)
	}
	if stamped != CurrentSchemaID {
		t.Fatalf("meta.schema_id = %s; want %s", stamped, CurrentSchemaID)
	}
	if !strings.Contains(healthAlertTriggerSQL(t, db), "S-Horizon health:") {
		t.Fatal("health alert trigger still carries the pre-rename body")
	}
	if err := CheckSchema(db); err != nil {
		t.Fatalf("post-migration schema check: %v", err)
	}
}

// U1 (state 1, second step): a spine a previous release left at the first
// step's target — S1 recorded, trigger body still pre-rename — takes only
// S2, and the ledger row, the stamp, and the trigger land in one pass.
func TestMigrateAppliesSecondStepFromFirstStepBaseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spine.db")
	db := provisionProductionSpine(t, path, testUUID, b01BaselineIdentity)
	defer db.Close()
	recordStep(t, db, ProductionMigrationRegistry()[0])
	revertHealthAlertTrigger(t, db)

	report, err := Migrate(db, ProductionMigrationRegistry(), CurrentSchemaID)
	if err != nil {
		t.Fatalf("migrate = %v", err)
	}
	if !report.Migrated || len(report.Applied) != 1 || report.Applied[0] != "rename-health-alert-body" {
		t.Fatalf("report = %+v; want only the second step applied", report)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM schema_history`); n != 2 {
		t.Fatalf("ledger rows = %d; want 2", n)
	}
	var stamped string
	if err := db.QueryRow(`SELECT schema_id FROM meta WHERE id = 1`).Scan(&stamped); err != nil {
		t.Fatal(err)
	}
	if stamped != CurrentSchemaID {
		t.Fatalf("meta.schema_id = %s; want %s", stamped, CurrentSchemaID)
	}
	if !strings.Contains(healthAlertTriggerSQL(t, db), "S-Horizon health:") {
		t.Fatal("health alert trigger still carries the pre-rename body")
	}
	if err := CheckSchema(db); err != nil {
		t.Fatalf("post-migration schema check: %v", err)
	}
}

// U3 (state 2): a spine whose ledger is complete at this release's identity
// is a no-op with every row and object untouched.
func TestMigrateNoOpWhenLedgerCompleteAtCurrentIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spine.db")
	db := provisionProductionSpine(t, path, testUUID, CurrentSchemaID)
	defer db.Close()
	registry := ProductionMigrationRegistry()
	recordStep(t, db, registry[0])
	recordStep(t, db, registry[1])

	report, err := Migrate(db, registry, CurrentSchemaID)
	if err != nil {
		t.Fatalf("migrate = %v", err)
	}
	if report.Migrated || len(report.Applied) != 0 {
		t.Fatalf("report = %+v; want no-op", report)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM schema_history`); n != 2 {
		t.Fatalf("ledger rows = %d; want the two recorded rows untouched", n)
	}
}

// U5 (downgrade direction): a spine a later release migrated to its own
// identity, viewed through the earlier release's one-step registry, records
// a step that registry does not define. The migration stops for manual
// diagnosis instead of rewriting or resetting the spine.
func TestPreviousReleaseRegistryRefusesLaterSpine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spine.db")
	db := provisionProductionSpine(t, path, testUUID, CurrentSchemaID)
	defer db.Close()
	registry := ProductionMigrationRegistry()
	recordStep(t, db, registry[0])
	recordStep(t, db, registry[1])

	previousRelease := registry[:1]
	_, err := Migrate(db, previousRelease, previousRelease[0].ToSchemaID)
	if !errors.Is(err, ErrMigrationSequenceAnomaly) {
		t.Fatalf("migrate error = %v; want ErrMigrationSequenceAnomaly", err)
	}
	var stamped string
	if err := db.QueryRow(`SELECT schema_id FROM meta WHERE id = 1`).Scan(&stamped); err != nil {
		t.Fatal(err)
	}
	if stamped != CurrentSchemaID {
		t.Fatalf("meta.schema_id changed to %s despite the refused view", stamped)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM schema_history`); n != 2 {
		t.Fatalf("ledger rows = %d; want 2 untouched", n)
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
