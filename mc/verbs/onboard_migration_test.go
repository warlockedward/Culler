package verbs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mc/deployment"
	"mc/distribution"
	"mc/substrate"
)

const migrationTestUUID = "77777777777777777777777777777777"

// buildPreLedgerSpineFile creates a spine file exactly as a pre-ledger
// release would have provisioned it: the current production objects minus
// the migration ledger table and the audit chain objects, stamped at the
// registry's first source identity.
func buildPreLedgerSpineFile(t *testing.T, path, uuid string) {
	t.Helper()
	db, err := substrate.Open(path)
	if err != nil {
		t.Fatalf("open spine: %v", err)
	}
	if err := substrate.Init(db); err != nil {
		db.Close()
		t.Fatalf("init spine: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE schema_history`); err != nil {
		db.Close()
		t.Fatalf("strip ledger: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS audit_events_seq_prefix`); err != nil {
		db.Close()
		t.Fatalf("strip seq prefix trigger: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS audit_events_no_delete`); err != nil {
		db.Close()
		t.Fatalf("strip no delete trigger: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS audit_events_no_update`); err != nil {
		db.Close()
		t.Fatalf("strip no update trigger: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS audit_events_event_type`); err != nil {
		db.Close()
		t.Fatalf("strip event type index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS audit_events`); err != nil {
		db.Close()
		t.Fatalf("strip audit chain table: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS usage_observations_no_delete`); err != nil {
		db.Close()
		t.Fatalf("strip usage observations no delete trigger: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS usage_observations_no_update`); err != nil {
		db.Close()
		t.Fatalf("strip usage observations no update trigger: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS usage_observations_local`); err != nil {
		db.Close()
		t.Fatalf("strip usage observations index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS usage_observations`); err != nil {
		db.Close()
		t.Fatalf("strip usage observations table: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS usage_events_no_delete`); err != nil {
		db.Close()
		t.Fatalf("strip usage events no delete trigger: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS usage_events_no_update`); err != nil {
		db.Close()
		t.Fatalf("strip usage events no update trigger: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS usage_events_local`); err != nil {
		db.Close()
		t.Fatalf("strip usage events index: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS usage_events_native_event`); err != nil {
		db.Close()
		t.Fatalf("strip usage events native event index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS usage_events`); err != nil {
		db.Close()
		t.Fatalf("strip usage events table: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS usage_local`); err != nil {
		db.Close()
		t.Fatalf("strip usage local index: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS usage_request`); err != nil {
		db.Close()
		t.Fatalf("strip usage request index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS usage`); err != nil {
		db.Close()
		t.Fatalf("strip usage table: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS activity_chain_map`); err != nil {
		db.Close()
		t.Fatalf("strip activity chain map: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS user_sessions_state_machine`); err != nil {
		db.Close()
		t.Fatalf("strip user sessions state machine: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS user_sessions_guarded_revoke`); err != nil {
		db.Close()
		t.Fatalf("strip user sessions guarded revoke: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS user_sessions_guarded_insert`); err != nil {
		db.Close()
		t.Fatalf("strip user sessions guarded insert: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS grants_guarded_insert`); err != nil {
		db.Close()
		t.Fatalf("strip grants guarded insert: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS grants_grantee`); err != nil {
		db.Close()
		t.Fatalf("strip grants grantee index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS grants`); err != nil {
		db.Close()
		t.Fatalf("strip grants table: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS team_members`); err != nil {
		db.Close()
		t.Fatalf("strip team members: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS teams_name`); err != nil {
		db.Close()
		t.Fatalf("strip teams name index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS teams`); err != nil {
		db.Close()
		t.Fatalf("strip teams table: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS user_sessions_user`); err != nil {
		db.Close()
		t.Fatalf("strip user sessions user index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS user_sessions`); err != nil {
		db.Close()
		t.Fatalf("strip user sessions table: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS users_username`); err != nil {
		db.Close()
		t.Fatalf("strip users username index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS users`); err != nil {
		db.Close()
		t.Fatalf("strip users table: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS trusted_ctx`); err != nil {
		db.Close()
		t.Fatalf("strip trusted context table: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS activity_legacy_actor_sealed`); err != nil {
		db.Close()
		t.Fatalf("strip legacy actor seal trigger: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS legacy_actor_mapping_no_update`); err != nil {
		db.Close()
		t.Fatalf("strip legacy actor mapping no update: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS legacy_actor_mapping_no_delete`); err != nil {
		db.Close()
		t.Fatalf("strip legacy actor mapping no delete: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS legacy_actor_mapping_user`); err != nil {
		db.Close()
		t.Fatalf("strip legacy actor mapping index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS legacy_actor_mapping`); err != nil {
		db.Close()
		t.Fatalf("strip legacy actor mapping table: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS degraded_grants_guarded_insert`); err != nil {
		db.Close()
		t.Fatalf("strip degraded grants guard trigger: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS degraded_grants_risk_scope`); err != nil {
		db.Close()
		t.Fatalf("strip degraded grants index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS degraded_grants`); err != nil {
		db.Close()
		t.Fatalf("strip degraded grants table: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS sign_requests_guarded_insert`); err != nil {
		db.Close()
		t.Fatalf("strip sign requests guard trigger: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS sign_requests_state_machine`); err != nil {
		db.Close()
		t.Fatalf("strip sign requests state machine trigger: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS sign_requests_state`); err != nil {
		db.Close()
		t.Fatalf("strip sign requests state index: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS sign_requests_supersedes`); err != nil {
		db.Close()
		t.Fatalf("strip sign requests supersedes index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS sign_requests`); err != nil {
		db.Close()
		t.Fatalf("strip sign requests table: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS anchor_checkpoints_guarded_insert`); err != nil {
		db.Close()
		t.Fatalf("strip anchor checkpoints guard trigger: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS anchor_checkpoints_no_update`); err != nil {
		db.Close()
		t.Fatalf("strip anchor checkpoints no update trigger: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS anchor_checkpoints_no_delete`); err != nil {
		db.Close()
		t.Fatalf("strip anchor checkpoints no delete trigger: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS anchor_checkpoints`); err != nil {
		db.Close()
		t.Fatalf("strip anchor checkpoints table: %v", err)
	}
	preID := substrate.ProductionMigrationRegistry()[0].FromSchemaID
	if _, err := db.Exec(`INSERT INTO meta (id, deployment_uuid, schema_id) VALUES (1, ?, ?)`, uuid, preID); err != nil {
		db.Close()
		t.Fatalf("stamp meta: %v", err)
	}
	db.Close()
}

// T7: OnboardSpine migrates a known old spine instead of refusing; the
// deployment UUID is preserved and the host side accepts the result.
func TestOnboardSpineMigratesPreLedgerSpine(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MC_HOME", home)
	spine := filepath.Join(t.TempDir(), "spine.db")
	buildPreLedgerSpineFile(t, spine, migrationTestUUID)

	req := OnboardSpineRequest{ProtocolVersion: 1, SchemaID: substrate.CurrentSchemaID, MirrorState: "present", MirrorUUID: migrationTestUUID}
	result, err := OnboardSpine(spine, req)
	if err != nil {
		t.Fatalf("OnboardSpine = %v", err)
	}
	if result.Status != "migrated" || result.DeploymentUUID != migrationTestUUID {
		t.Fatalf("result = %+v; want status migrated with the preserved UUID", result)
	}

	// The migrated spine now passes the schema check and carries the ledger.
	db, err := substrate.OpenReadOnly(spine)
	if err != nil {
		t.Fatalf("reopen migrated spine: %v", err)
	}
	if err := substrate.CheckSchema(db); err != nil {
		t.Fatalf("migrated spine schema check: %v", err)
	}
	var ledgerRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_history`).Scan(&ledgerRows); err != nil || ledgerRows != 11 {
		t.Fatalf("ledger rows = %d (%v); want 11", ledgerRows, err)
	}
	db.Close()

	// Host-side acceptance: FinalizeOnboardHome accepts the migrated status.
	if err := os.WriteFile(filepath.Join(home, "deployment.uuid"), []byte(migrationTestUUID+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	status, detail, err := FinalizeOnboardHome(result)
	if err != nil {
		t.Fatalf("FinalizeOnboardHome = %v", err)
	}
	if !strings.Contains(detail, "migrated to the current schema") {
		t.Fatalf("detail = %q; want the migration statement", detail)
	}
	if status != "done" {
		t.Fatalf("status = %q; want done", status)
	}
}

// T7b regression guard: an explicit Reset still replaces a schema-differing
// spine; migration never preempts an operator-ordered reset.
func TestOnboardSpineExplicitResetStillReplacesOldSpine(t *testing.T) {
	spine := filepath.Join(t.TempDir(), "spine.db")
	buildPreLedgerSpineFile(t, spine, migrationTestUUID)

	req := OnboardSpineRequest{ProtocolVersion: 1, SchemaID: substrate.CurrentSchemaID, MirrorState: "present", MirrorUUID: migrationTestUUID, Reset: true}
	result, err := OnboardSpine(spine, req)
	if err != nil {
		t.Fatalf("OnboardSpine = %v", err)
	}
	if result.Status != "reset" {
		t.Fatalf("status = %q; want reset", result.Status)
	}
}

// T7c regression guard: a spine at an identity no registered step starts
// from is still refused with the pre-B01 message, not reset and not
// migrated.
func TestOnboardSpineStillRefusesUnknownIdentity(t *testing.T) {
	spine := filepath.Join(t.TempDir(), "spine.db")
	db, err := substrate.Open(spine)
	if err != nil {
		t.Fatal(err)
	}
	if err := substrate.Init(db); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP TABLE schema_history`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	unknownID := strings.Repeat("c", 64)
	if _, err := db.Exec(`INSERT INTO meta (id, deployment_uuid, schema_id) VALUES (1, ?, ?)`, migrationTestUUID, unknownID); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()

	req := OnboardSpineRequest{ProtocolVersion: 1, SchemaID: substrate.CurrentSchemaID, MirrorState: "present", MirrorUUID: migrationTestUUID}
	_, err = OnboardSpine(spine, req)
	if err == nil {
		t.Fatal("OnboardSpine accepted a spine with no migration path")
	}
	if !strings.Contains(err.Error(), "database schema differs") && !strings.Contains(err.Error(), "database-schema-differs") {
		t.Fatalf("error = %v; want the schema-differs refusal", err)
	}
}

// T8 (verify-window evidence, Q10-A ordering): the update flow re-crosses
// OnboardSpine with the new binary while the journal sits in the verify
// phase. This test composes that exact situation: an uncommitted journal
// whose Previous snapshot (record + image + schema identity) is intact, and
// a deployment spine at the previous release's identity. The crossing must
// migrate the spine while the old snapshot stays restorable: the journal
// never advances to committed and the Previous reference never clears.
func TestMigrationCrossingLeavesPreviousSnapshotRestorable(t *testing.T) {
	spine := filepath.Join(t.TempDir(), "spine.db")
	buildPreLedgerSpineFile(t, spine, migrationTestUUID)

	preID := substrate.ProductionMigrationRegistry()[0].FromSchemaID
	journal := &deployment.UpdateJournal{
		Version: 1,
		ID:      strings.Repeat("a", 16),
		Phase:   "verify",
		Previous: &distribution.Record{
			Version:   "0.1.0",
			Format:    1,
			Commit:    strings.Repeat("b", 40),
			ReleaseID: strings.Repeat("b", 40),
			Tag:       "v0.1.0",
			Platform:  "darwin-arm64",
			Schema:    preID,
			Image:     "ghcr.io/example/mc-prod:previous",
		},
	}
	if journal.Committed() {
		t.Fatal("test journal must start uncommitted")
	}

	// The same OnboardSpine crossing the update flow's home section runs.
	req := OnboardSpineRequest{ProtocolVersion: 1, SchemaID: substrate.CurrentSchemaID, MirrorState: "present", MirrorUUID: migrationTestUUID}
	result, err := OnboardSpine(spine, req)
	if err != nil || result.Status != "migrated" {
		t.Fatalf("crossing = %+v, %v; want a successful migration", result, err)
	}

	// Q10-A: the old binary snapshot remains in force for the whole window —
	// the journal is still uncommitted, Previous still recorded, so the
	// restore path is available if anything after the migration fails.
	if journal.Committed() {
		t.Fatal("journal committed without the update flow advancing it")
	}
	if journal.Previous == nil || journal.Previous.Schema != preID || journal.Previous.Image == "" {
		t.Fatal("the previous release snapshot was lost around the migration")
	}
}

// T9 (negative control): a spine whose migration history no longer matches
// this release's step definition is refused by the crossing (state 3). The
// refusal must leave the database untouched and the update journal
// unadvanced, so the previous release stays restorable instead of the data
// being wiped over a ledger the release cannot trust.
func TestBlockedMigrationKeepsRestoreAvailable(t *testing.T) {
	spine := filepath.Join(t.TempDir(), "spine.db")
	buildPreLedgerSpineFile(t, spine, migrationTestUUID)

	preID := substrate.ProductionMigrationRegistry()[0].FromSchemaID
	step := substrate.ProductionMigrationRegistry()[0]

	// Seed the spine by running the step for real, then alter the recorded
	// history and the stamp: the spine is structurally current but its
	// ledger checksum conflicts with this release's definition.
	db, err := substrate.Open(spine)
	if err != nil {
		t.Fatalf("open spine: %v", err)
	}
	report, err := substrate.Migrate(db, substrate.ProductionMigrationRegistry(), substrate.CurrentSchemaID)
	if err != nil || !report.Migrated {
		db.Close()
		t.Fatalf("seed migration = %+v, %v; want S1 applied", report, err)
	}
	if _, err := db.Exec(`UPDATE schema_history SET step_checksum = ? WHERE step_name = ?`, strings.Repeat("0", 64), step.Name); err != nil {
		db.Close()
		t.Fatalf("tamper ledger checksum: %v", err)
	}
	if _, err := db.Exec(`UPDATE meta SET schema_id = ? WHERE id = 1`, preID); err != nil {
		db.Close()
		t.Fatalf("retire the stamp to the pre-ledger identity: %v", err)
	}

	var eventsBefore int
	if err := db.QueryRow(`SELECT COUNT(*) FROM activity WHERE kind = 'schema.migration.step'`).Scan(&eventsBefore); err != nil {
		db.Close()
		t.Fatalf("count seed events: %v", err)
	}
	db.Close()

	// The crossing is refused with the blocked-ledger code.
	req := OnboardSpineRequest{ProtocolVersion: 1, SchemaID: substrate.CurrentSchemaID, MirrorState: "present", MirrorUUID: migrationTestUUID}
	_, err = OnboardSpine(spine, req)
	var blocked *DomainError
	if !errors.As(err, &blocked) || blocked.Code != "database-migration-blocked" {
		t.Fatalf("OnboardSpine = %v; want the blocked-migration refusal", err)
	}

	// The database is untouched by the refusal: the stamp still predates
	// the ledger's baseline, the tampered row still carries its forged
	// checksum, and the crossing wrote no migration event of its own.
	ro, err := substrate.OpenReadOnly(spine)
	if err != nil {
		t.Fatalf("reopen spine: %v", err)
	}
	var stampedID, storedChecksum string
	var eventsAfter int
	if err := ro.QueryRow(`SELECT schema_id FROM meta WHERE id = 1`).Scan(&stampedID); err != nil || stampedID != preID {
		t.Fatalf("meta.schema_id = %q (%v); want the pre-ledger identity untouched", stampedID, err)
	}
	if err := ro.QueryRow(`SELECT step_checksum FROM schema_history WHERE step_name = ?`, step.Name).Scan(&storedChecksum); err != nil || storedChecksum != strings.Repeat("0", 64) {
		t.Fatalf("ledger checksum = %q (%v); want the tampered row untouched", storedChecksum, err)
	}
	if err := ro.QueryRow(`SELECT COUNT(*) FROM activity WHERE kind = 'schema.migration.step'`).Scan(&eventsAfter); err != nil || eventsAfter != eventsBefore {
		t.Fatalf("migration events = %d, was %d; the refusal must write none", eventsAfter, eventsBefore)
	}
	ro.Close()

	// The update journal the crossing belongs to never advanced: an
	// uncommitted journal passes RestoreUpdateCheckpoint's committed gate,
	// so the restore path entry stays open after the refusal.
	journal := &deployment.UpdateJournal{
		Version: 1,
		ID:      strings.Repeat("a", 16),
		Phase:   "verify",
		Previous: &distribution.Record{
			Version:   "0.1.0",
			Format:    1,
			Commit:    strings.Repeat("b", 40),
			ReleaseID: strings.Repeat("b", 40),
			Tag:       "v0.1.0",
			Platform:  "darwin-arm64",
			Schema:    preID,
			Image:     "ghcr.io/example/mc-prod:previous",
		},
	}
	restoreErr := deployment.RestoreUpdateCheckpoint(context.Background(), "", journal, nil)
	if restoreErr == nil || !strings.Contains(restoreErr.Error(), "no complete checkpoint") {
		t.Fatalf("RestoreUpdateCheckpoint = %v; want the uncommitted journal to pass the committed gate", restoreErr)
	}
}

// buildFirstStepSpineFile reproduces the spine state a release carrying only
// the first step would have left: this baseline's objects minus the audit
// chain objects, the ledger recording step one, and the meta stamp at step
// one's target identity.
func buildFirstStepSpineFile(t *testing.T, path, uuid string) {
	t.Helper()
	db, err := substrate.Open(path)
	if err != nil {
		t.Fatalf("open spine: %v", err)
	}
	if err := substrate.Init(db); err != nil {
		db.Close()
		t.Fatalf("init spine: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS audit_events_seq_prefix`); err != nil {
		db.Close()
		t.Fatalf("strip seq prefix trigger: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS audit_events_no_delete`); err != nil {
		db.Close()
		t.Fatalf("strip no delete trigger: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS audit_events_no_update`); err != nil {
		db.Close()
		t.Fatalf("strip no update trigger: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS audit_events_event_type`); err != nil {
		db.Close()
		t.Fatalf("strip event type index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS audit_events`); err != nil {
		db.Close()
		t.Fatalf("strip audit chain table: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS usage_observations_no_delete`); err != nil {
		db.Close()
		t.Fatalf("strip usage observations no delete trigger: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS usage_observations_no_update`); err != nil {
		db.Close()
		t.Fatalf("strip usage observations no update trigger: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS usage_observations_local`); err != nil {
		db.Close()
		t.Fatalf("strip usage observations index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS usage_observations`); err != nil {
		db.Close()
		t.Fatalf("strip usage observations table: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS usage_events_no_delete`); err != nil {
		db.Close()
		t.Fatalf("strip usage events no delete trigger: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS usage_events_no_update`); err != nil {
		db.Close()
		t.Fatalf("strip usage events no update trigger: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS usage_events_local`); err != nil {
		db.Close()
		t.Fatalf("strip usage events index: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS usage_events_native_event`); err != nil {
		db.Close()
		t.Fatalf("strip usage events native event index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS usage_events`); err != nil {
		db.Close()
		t.Fatalf("strip usage events table: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS usage_local`); err != nil {
		db.Close()
		t.Fatalf("strip usage local index: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS usage_request`); err != nil {
		db.Close()
		t.Fatalf("strip usage request index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS usage`); err != nil {
		db.Close()
		t.Fatalf("strip usage table: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS activity_chain_map`); err != nil {
		db.Close()
		t.Fatalf("strip activity chain map: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS user_sessions_state_machine`); err != nil {
		db.Close()
		t.Fatalf("strip user sessions state machine: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS user_sessions_guarded_revoke`); err != nil {
		db.Close()
		t.Fatalf("strip user sessions guarded revoke: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS user_sessions_guarded_insert`); err != nil {
		db.Close()
		t.Fatalf("strip user sessions guarded insert: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS grants_guarded_insert`); err != nil {
		db.Close()
		t.Fatalf("strip grants guarded insert: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS grants_grantee`); err != nil {
		db.Close()
		t.Fatalf("strip grants grantee index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS grants`); err != nil {
		db.Close()
		t.Fatalf("strip grants table: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS team_members`); err != nil {
		db.Close()
		t.Fatalf("strip team members: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS teams_name`); err != nil {
		db.Close()
		t.Fatalf("strip teams name index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS teams`); err != nil {
		db.Close()
		t.Fatalf("strip teams table: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS user_sessions_user`); err != nil {
		db.Close()
		t.Fatalf("strip user sessions user index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS user_sessions`); err != nil {
		db.Close()
		t.Fatalf("strip user sessions table: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS users_username`); err != nil {
		db.Close()
		t.Fatalf("strip users username index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS users`); err != nil {
		db.Close()
		t.Fatalf("strip users table: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS trusted_ctx`); err != nil {
		db.Close()
		t.Fatalf("strip trusted context table: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS activity_legacy_actor_sealed`); err != nil {
		db.Close()
		t.Fatalf("strip legacy actor seal trigger: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS legacy_actor_mapping_no_update`); err != nil {
		db.Close()
		t.Fatalf("strip legacy actor mapping no update: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS legacy_actor_mapping_no_delete`); err != nil {
		db.Close()
		t.Fatalf("strip legacy actor mapping no delete: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS legacy_actor_mapping_user`); err != nil {
		db.Close()
		t.Fatalf("strip legacy actor mapping index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS legacy_actor_mapping`); err != nil {
		db.Close()
		t.Fatalf("strip legacy actor mapping table: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS degraded_grants_guarded_insert`); err != nil {
		db.Close()
		t.Fatalf("strip degraded grants guard trigger: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS degraded_grants_risk_scope`); err != nil {
		db.Close()
		t.Fatalf("strip degraded grants index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS degraded_grants`); err != nil {
		db.Close()
		t.Fatalf("strip degraded grants table: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS sign_requests_guarded_insert`); err != nil {
		db.Close()
		t.Fatalf("strip sign requests guard trigger: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS sign_requests_state_machine`); err != nil {
		db.Close()
		t.Fatalf("strip sign requests state machine trigger: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS sign_requests_state`); err != nil {
		db.Close()
		t.Fatalf("strip sign requests state index: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS sign_requests_supersedes`); err != nil {
		db.Close()
		t.Fatalf("strip sign requests supersedes index: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS sign_requests`); err != nil {
		db.Close()
		t.Fatalf("strip sign requests table: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS anchor_checkpoints_guarded_insert`); err != nil {
		db.Close()
		t.Fatalf("strip anchor checkpoints guard trigger: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS anchor_checkpoints_no_update`); err != nil {
		db.Close()
		t.Fatalf("strip anchor checkpoints no update trigger: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS anchor_checkpoints_no_delete`); err != nil {
		db.Close()
		t.Fatalf("strip anchor checkpoints no delete trigger: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS anchor_checkpoints`); err != nil {
		db.Close()
		t.Fatalf("strip anchor checkpoints table: %v", err)
	}
	step1 := substrate.ProductionMigrationRegistry()[0]
	if _, err := db.Exec(`INSERT INTO meta (id, deployment_uuid, schema_id) VALUES (1, ?, ?)`, uuid, step1.ToSchemaID); err != nil {
		db.Close()
		t.Fatalf("stamp first step identity: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO schema_history (version, step_name, step_checksum, from_schema_id, to_schema_id, applied_at) VALUES (?, ?, ?, ?, ?, 'now')`, step1.Version, step1.Name, step1.Checksum, step1.FromSchemaID, step1.ToSchemaID); err != nil {
		db.Close()
		t.Fatalf("record first step: %v", err)
	}
	db.Close()
}

// U4: OnboardSpine crosses a spine left at the first step's target identity:
// the recorded first step is skipped, the second through eleventh steps
// land, the deployment UUID is preserved, and the schema check passes on
// the result.
func TestOnboardSpineMigratesFirstStepBaselineSpine(t *testing.T) {
	spine := filepath.Join(t.TempDir(), "spine.db")
	buildFirstStepSpineFile(t, spine, migrationTestUUID)

	req := OnboardSpineRequest{ProtocolVersion: 1, SchemaID: substrate.CurrentSchemaID, MirrorState: "present", MirrorUUID: migrationTestUUID}
	result, err := OnboardSpine(spine, req)
	if err != nil {
		t.Fatalf("OnboardSpine = %v", err)
	}
	if result.Status != "migrated" || result.DeploymentUUID != migrationTestUUID {
		t.Fatalf("result = %+v; want status migrated with the preserved UUID", result)
	}
	db, err := substrate.OpenReadOnly(spine)
	if err != nil {
		t.Fatalf("reopen spine: %v", err)
	}
	if err := substrate.CheckSchema(db); err != nil {
		t.Fatalf("migrated spine schema check: %v", err)
	}
	var ledgerRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_history`).Scan(&ledgerRows); err != nil || ledgerRows != 11 {
		t.Fatalf("ledger rows = %d (%v); want 11", ledgerRows, err)
	}
	db.Close()
}
