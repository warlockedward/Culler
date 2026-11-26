package substrate

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// stripLegacyActorSealing removes the eighth step's objects, reproducing a
// spine provisioned by a release before the legacy actor sealing existed.
func stripLegacyActorSealing(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS activity_legacy_actor_sealed`); err != nil {
		t.Fatalf("strip activity_legacy_actor_sealed: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS legacy_actor_mapping_no_update`); err != nil {
		t.Fatalf("strip legacy_actor_mapping_no_update: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS legacy_actor_mapping_no_delete`); err != nil {
		t.Fatalf("strip legacy_actor_mapping_no_delete: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS legacy_actor_mapping_user`); err != nil {
		t.Fatalf("strip legacy_actor_mapping_user: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS legacy_actor_mapping`); err != nil {
		t.Fatalf("strip legacy_actor_mapping: %v", err)
	}
}

// d01IdentitySpine provisions a spine at the d01 identity: everything
// through the seventh step is in place, the legacy actor sealing is not, and
// the ledger records S1-S7.
func d01IdentitySpine(t *testing.T) *sql.DB {
	t.Helper()
	db := provisionProductionSpine(t, filepath.Join(t.TempDir(), "spine.db"), testUUID, d01FoundationIdentity)
	t.Cleanup(func() { db.Close() })
	stripLegacyActorSealing(t, db)
	stripDegradedGrants(t, db)
	registry := ProductionMigrationRegistry()
	for _, step := range registry[:7] {
		recordStep(t, db, step)
	}
	return db
}

// L1: crossing from the d01 baseline lands the legacy actor sealing and the
// degraded grants registry: both steps apply, the objects are present, the
// ledger and the stamp follow, the spine passes the schema check, and the
// seal is live at once: the legacy actor is refused at the execution point
// while the current owner actor writes through.
func TestLegacyActorSealStepFromD01Baseline(t *testing.T) {
	db := d01IdentitySpine(t)
	registry := ProductionMigrationRegistry()
	report, err := Migrate(db, registry, CurrentSchemaID)
	if err != nil {
		t.Fatalf("migrate = %v", err)
	}
	if !report.Migrated || len(report.Applied) != 2 || report.Applied[0] != "add-legacy-actor-sealing" || report.Applied[1] != "add-degraded-grants" {
		t.Fatalf("report = %+v; want the eighth and ninth steps applied", report)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'legacy_actor_mapping'`); n != 1 {
		t.Fatalf("legacy actor mapping table = %d; want 1", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name IN ('legacy_actor_mapping_no_update','legacy_actor_mapping_no_delete','activity_legacy_actor_sealed')`); n != 3 {
		t.Fatalf("sealing triggers = %d; want 3", n)
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
	// The seal is live at the moment the step lands.
	if _, err := db.Exec(`INSERT INTO activity (actor, kind, detail) VALUES ('operator', 'task.added', '{}')`); err == nil {
		t.Fatal("a new event attributed to the legacy operator actor passed the seal")
	}
	if _, err := db.Exec(`INSERT INTO activity (actor, kind, detail) VALUES ('owner', 'task.added', '{}')`); err != nil {
		t.Fatalf("the current owner actor was refused: %v", err)
	}
}

// L3: zero rewrite — every pre-existing activity row, legacy operator rows
// among them, survives the eighth step verbatim; the step's own bookkeeping
// row is the only addition, and the seal is live on the migrated spine.
func TestLegacyActorSealStepRewritesNothing(t *testing.T) {
	db := d01IdentitySpine(t)
	seededID := seedActivityRow(t, db, "operator", "probe.legacy", strPtr("subject"), strPtr("detail"), "2026-11-19 22:11:00")
	before := activityDigestUpTo(t, db, seededID)
	registry := ProductionMigrationRegistry()
	if _, err := Migrate(db, registry, CurrentSchemaID); err != nil {
		t.Fatalf("migrate = %v", err)
	}
	after := activityDigestUpTo(t, db, seededID)
	if before != after {
		t.Fatalf("pre-existing activity rows changed across the seal migration (%s -> %s); the zero-rewrite proof failed", before, after)
	}
	var actor, kind string
	if err := db.QueryRow(`SELECT actor, kind FROM activity WHERE id = ?`, seededID).Scan(&actor, &kind); err != nil {
		t.Fatal(err)
	}
	if actor != "operator" || kind != "probe.legacy" {
		t.Fatalf("legacy row = (%s, %s); want it verbatim", actor, kind)
	}
	if _, err := db.Exec(`INSERT INTO activity (actor, kind, detail) VALUES ('operator', 'task.added', '{}')`); err == nil {
		t.Fatal("the migrated spine still accepts new legacy operator writes")
	}
}

// L2: the eighth step's object definitions match the reference schema byte
// for byte, so a migrated spine hashes like a fresh one.
func TestLegacyActorSealDDLMatchesReferenceSchemaByteForByte(t *testing.T) {
	fresh := chainSpine(t)
	db := d01IdentitySpine(t)
	registry := ProductionMigrationRegistry()
	if _, err := Migrate(db, registry, CurrentSchemaID); err != nil {
		t.Fatalf("migrate = %v", err)
	}
	readObjects := func(d *sql.DB) []string {
		rows, err := d.Query(`SELECT name, sql FROM sqlite_master
			WHERE name IN ('legacy_actor_mapping','legacy_actor_mapping_user',
			               'legacy_actor_mapping_no_update','legacy_actor_mapping_no_delete',
			               'activity_legacy_actor_sealed')
			ORDER BY type, name`)
		if err != nil {
			t.Fatalf("query sealing objects: %v", err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var name, definition string
			if err := rows.Scan(&name, &definition); err != nil {
				t.Fatalf("scan sealing object: %v", err)
			}
			out = append(out, name+"|"+definition)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate sealing objects: %v", err)
		}
		return out
	}
	freshRows := readObjects(fresh)
	migratedRows := readObjects(db)
	if len(freshRows) == 0 || len(freshRows) != len(migratedRows) {
		t.Fatalf("sealing object counts = %d vs %d; want equal and nonzero", len(freshRows), len(migratedRows))
	}
	for i := range freshRows {
		if freshRows[i] != migratedRows[i] {
			t.Fatalf("sealing object %d diverges:\n fresh: %s\nmigrated: %s", i, freshRows[i], migratedRows[i])
		}
	}
}
