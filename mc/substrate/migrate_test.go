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

// stripAuditChain removes the audit chain objects, reproducing a spine
// provisioned by a release before the chain table existed.
func stripAuditChain(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS audit_events_seq_prefix`); err != nil {
		t.Fatalf("drop seq prefix trigger: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS audit_events_no_delete`); err != nil {
		t.Fatalf("drop no delete trigger: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS audit_events_no_update`); err != nil {
		t.Fatalf("drop no update trigger: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS audit_events_event_type`); err != nil {
		t.Fatalf("drop event type index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS audit_events`); err != nil {
		t.Fatalf("drop audit events table: %v", err)
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

// T1 (state 1, full chain): on a pre-ledger spine no step is recorded;
// all three execute in version order and the spine lands at this release's
// identity with its ledger complete.
func TestMigrateAppliesUnrecordedStepsFromPreLedgerBaseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spine.db")
	db := provisionProductionSpine(t, path, testUUID, preLedgerBaselineIdentity)
	defer db.Close()
	stripLedger(t, db)
	stripAuditChain(t, db)
	stripUsageTables(t, db)
	stripActivityChainMap(t, db)
	stripIdentityFoundation(t, db)
	stripLegacyActorSealing(t, db)
	stripDegradedGrants(t, db)
	stripSignRequests(t, db)
	stripAnchorCheckpoints(t, db)
	revertHealthAlertTrigger(t, db)

	registry := ProductionMigrationRegistry()
	report, err := Migrate(db, registry, CurrentSchemaID)
	if err != nil {
		t.Fatalf("migrate = %v", err)
	}
	if !report.Migrated || len(report.Applied) != 11 || report.Applied[0] != "add-schema-history-ledger" || report.Applied[1] != "rename-health-alert-body" || report.Applied[2] != "add-audit-events-chain" || report.Applied[3] != "add-usage-observations" || report.Applied[4] != "migrate-usage-to-v2" || report.Applied[5] != "migrate-activity-to-chain" || report.Applied[6] != "add-identity-foundation" || report.Applied[7] != "add-legacy-actor-sealing" || report.Applied[8] != "add-degraded-grants" || report.Applied[9] != "add-sign-requests" || report.Applied[10] != "add-anchor-checkpoints" {
		t.Fatalf("report = %+v", report)
	}
	step1, step2, step3, step4, step5, step6, step7, step8, step9, step10, step11 := registry[0], registry[1], registry[2], registry[3], registry[4], registry[5], registry[6], registry[7], registry[8], registry[9], registry[10]
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
	if checksum2 != step2.Checksum || from2 != b01BaselineIdentity || to2 != b02BaselineIdentity {
		t.Fatalf("ledger row 2 = (%s, %s, %s); want (%s, %s, %s)", checksum2, from2, to2, step2.Checksum, b01BaselineIdentity, b02BaselineIdentity)
	}
	var checksum3, from3, to3 string
	if err := db.QueryRow(`SELECT step_checksum, from_schema_id, to_schema_id FROM schema_history WHERE version = 3`).Scan(&checksum3, &from3, &to3); err != nil {
		t.Fatal(err)
	}
	if checksum3 != step3.Checksum || from3 != b02BaselineIdentity || to3 != b07BaselineIdentity {
		t.Fatalf("ledger row 3 = (%s, %s, %s); want (%s, %s, %s)", checksum3, from3, to3, step3.Checksum, b02BaselineIdentity, b07BaselineIdentity)
	}
	var checksum4, from4, to4 string
	if err := db.QueryRow(`SELECT step_checksum, from_schema_id, to_schema_id FROM schema_history WHERE version = 4`).Scan(&checksum4, &from4, &to4); err != nil {
		t.Fatal(err)
	}
	if checksum4 != step4.Checksum || from4 != b07BaselineIdentity || to4 != c03UsageIdentity {
		t.Fatalf("ledger row 4 = (%s, %s, %s); want (%s, %s, %s)", checksum4, from4, to4, step4.Checksum, b07BaselineIdentity, c03UsageIdentity)
	}
	var checksum5, from5, to5 string
	if err := db.QueryRow(`SELECT step_checksum, from_schema_id, to_schema_id FROM schema_history WHERE version = 5`).Scan(&checksum5, &from5, &to5); err != nil {
		t.Fatal(err)
	}
	if checksum5 != step5.Checksum || from5 != c03UsageIdentity || to5 != b09ChainIdentity {
		t.Fatalf("ledger row 5 = (%s, %s, %s); want (%s, %s, %s)", checksum5, from5, to5, step5.Checksum, c03UsageIdentity, b09ChainIdentity)
	}
	var checksum6, from6, to6 string
	if err := db.QueryRow(`SELECT step_checksum, from_schema_id, to_schema_id FROM schema_history WHERE version = 6`).Scan(&checksum6, &from6, &to6); err != nil {
		t.Fatal(err)
	}
	if checksum6 != step6.Checksum || from6 != b09ChainIdentity || to6 != b08MapIdentity {
		t.Fatalf("ledger row 6 = (%s, %s, %s); want (%s, %s, %s)", checksum6, from6, to6, step6.Checksum, b09ChainIdentity, b08MapIdentity)
	}
	var checksum7, from7, to7 string
	if err := db.QueryRow(`SELECT step_checksum, from_schema_id, to_schema_id FROM schema_history WHERE version = 7`).Scan(&checksum7, &from7, &to7); err != nil {
		t.Fatal(err)
	}
	if checksum7 != step7.Checksum || from7 != b08MapIdentity || to7 != d01FoundationIdentity {
		t.Fatalf("ledger row 7 = (%s, %s, %s); want (%s, %s, %s)", checksum7, from7, to7, step7.Checksum, b08MapIdentity, d01FoundationIdentity)
	}
	var checksum8, from8, to8 string
	if err := db.QueryRow(`SELECT step_checksum, from_schema_id, to_schema_id FROM schema_history WHERE version = 8`).Scan(&checksum8, &from8, &to8); err != nil {
		t.Fatal(err)
	}
	if checksum8 != step8.Checksum || from8 != d01FoundationIdentity || to8 != d03SealIdentity {
		t.Fatalf("ledger row 8 = (%s, %s, %s); want (%s, %s, %s)", checksum8, from8, to8, step8.Checksum, d01FoundationIdentity, d03SealIdentity)
	}
	var checksum9, from9, to9 string
	if err := db.QueryRow(`SELECT step_checksum, from_schema_id, to_schema_id FROM schema_history WHERE version = 9`).Scan(&checksum9, &from9, &to9); err != nil {
		t.Fatal(err)
	}
	if checksum9 != step9.Checksum || from9 != d03SealIdentity || to9 != d05RegistryIdentity {
		t.Fatalf("ledger row 9 = (%s, %s, %s); want (%s, %s, %s)", checksum9, from9, to9, step9.Checksum, d03SealIdentity, d05RegistryIdentity)
	}
	var checksum10, from10, to10 string
	if err := db.QueryRow(`SELECT step_checksum, from_schema_id, to_schema_id FROM schema_history WHERE version = 10`).Scan(&checksum10, &from10, &to10); err != nil {
		t.Fatal(err)
	}
	if checksum10 != step10.Checksum || from10 != d05RegistryIdentity || to10 != d06SigningIdentity {
		t.Fatalf("ledger row 10 = (%s, %s, %s); want (%s, %s, %s)", checksum10, from10, to10, step10.Checksum, d05RegistryIdentity, d06SigningIdentity)
	}
	var checksum11, from11, to11 string
	if err := db.QueryRow(`SELECT step_checksum, from_schema_id, to_schema_id FROM schema_history WHERE version = 11`).Scan(&checksum11, &from11, &to11); err != nil {
		t.Fatal(err)
	}
	if checksum11 != step11.Checksum || from11 != d06SigningIdentity || to11 != CurrentSchemaID {
		t.Fatalf("ledger row 11 = (%s, %s, %s); want (%s, %s, %s)", checksum11, from11, to11, step11.Checksum, d06SigningIdentity, CurrentSchemaID)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM activity WHERE kind = 'schema.migration.step'`); n != 11 {
		t.Fatalf("migration events = %d; want 11", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM audit_events WHERE aggregate_id = 'schema'`); n != 9 {
		t.Fatalf("schema aggregate chain rows = %d; want 9 (the step that creates the table starts the chain)", n)
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

// U1 (state 1, second step onward): a spine a previous release left at the
// first step's target — S1 recorded, trigger body still pre-rename, no
// chain table — takes S2 and S3, and the ledger rows, the stamp, the
// trigger, and the chain land in one pass.
func TestMigrateAppliesSecondStepFromFirstStepBaseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spine.db")
	db := provisionProductionSpine(t, path, testUUID, b01BaselineIdentity)
	defer db.Close()
	recordStep(t, db, ProductionMigrationRegistry()[0])
	stripAuditChain(t, db)
	stripUsageTables(t, db)
	stripActivityChainMap(t, db)
	stripIdentityFoundation(t, db)
	stripLegacyActorSealing(t, db)
	stripDegradedGrants(t, db)
	stripSignRequests(t, db)
	stripAnchorCheckpoints(t, db)
	revertHealthAlertTrigger(t, db)

	report, err := Migrate(db, ProductionMigrationRegistry(), CurrentSchemaID)
	if err != nil {
		t.Fatalf("migrate = %v", err)
	}
	if !report.Migrated || len(report.Applied) != 10 || report.Applied[0] != "rename-health-alert-body" || report.Applied[1] != "add-audit-events-chain" || report.Applied[2] != "add-usage-observations" || report.Applied[3] != "migrate-usage-to-v2" || report.Applied[4] != "migrate-activity-to-chain" || report.Applied[5] != "add-identity-foundation" || report.Applied[6] != "add-legacy-actor-sealing" || report.Applied[7] != "add-degraded-grants" || report.Applied[8] != "add-sign-requests" || report.Applied[9] != "add-anchor-checkpoints" {
		t.Fatalf("report = %+v; want the second through eleventh steps applied", report)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM schema_history`); n != 11 {
		t.Fatalf("ledger rows = %d; want 11", n)
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
	recordStep(t, db, registry[2])
	recordStep(t, db, registry[3])
	recordStep(t, db, registry[4])
	recordStep(t, db, registry[5])
	recordStep(t, db, registry[6])
	recordStep(t, db, registry[7])
	recordStep(t, db, registry[8])
	recordStep(t, db, registry[9])
	recordStep(t, db, registry[10])

	report, err := Migrate(db, registry, CurrentSchemaID)
	if err != nil {
		t.Fatalf("migrate = %v", err)
	}
	if report.Migrated || len(report.Applied) != 0 {
		t.Fatalf("report = %+v; want no-op", report)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM schema_history`); n != 11 {
		t.Fatalf("ledger rows = %d; want the eleven recorded rows untouched", n)
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
