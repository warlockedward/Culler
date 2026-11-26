package substrate

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// stripDegradedGrants removes the ninth step's objects, reproducing a spine
// provisioned by a release before the degraded grants registry existed.
func stripDegradedGrants(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS degraded_grants_guarded_insert`); err != nil {
		t.Fatalf("strip degraded_grants_guarded_insert: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS degraded_grants_risk_scope`); err != nil {
		t.Fatalf("strip degraded_grants_risk_scope: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS degraded_grants`); err != nil {
		t.Fatalf("strip degraded_grants: %v", err)
	}
}

// d03Spine provisions a spine at the d03 identity: everything through the
// eighth step is in place, the degraded grants registry is not, and the
// ledger records S1-S8.
func d03Spine(t *testing.T) *sql.DB {
	t.Helper()
	db := provisionProductionSpine(t, filepath.Join(t.TempDir(), "spine.db"), testUUID, d03SealIdentity)
	t.Cleanup(func() { db.Close() })
	stripDegradedGrants(t, db)
	stripSignRequests(t, db)
	registry := ProductionMigrationRegistry()
	for _, step := range registry[:8] {
		recordStep(t, db, step)
	}
	return db
}

// M1: crossing from the d03 baseline lands the degraded grants registry and
// the signing request state machine: both steps apply, the objects are
// present, the ledger and the stamp follow, the spine passes the schema
// check, and the guard trigger is live at once — an unguarded insert is
// refused and a mismatched attribution is refused.
func TestDegradedGrantsStepFromD03Baseline(t *testing.T) {
	db := d03Spine(t)
	registry := ProductionMigrationRegistry()
	report, err := Migrate(db, registry, CurrentSchemaID)
	if err != nil {
		t.Fatalf("migrate = %v", err)
	}
	if !report.Migrated || len(report.Applied) != 2 || report.Applied[0] != "add-degraded-grants" || report.Applied[1] != "add-sign-requests" {
		t.Fatalf("report = %+v; want the ninth and tenth steps applied", report)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'degraded_grants'`); n != 1 {
		t.Fatalf("degraded_grants table = %d; want 1", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name = 'degraded_grants_guarded_insert'`); n != 1 {
		t.Fatalf("guard trigger = %d; want 1", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM schema_history`); n != 10 {
		t.Fatalf("ledger rows = %d; want 10", n)
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
	// The guard trigger is live at the moment the step lands.
	if _, err := db.Exec(`INSERT INTO degraded_grants
		(grant_id, risk, scope, compensation, revocation_conditions, issued_by_user_id, expires_at)
		VALUES ('cccccccccccccccccccccccccccccccc', 'RR-3', 'network', 'zero credentials',
		        'any egress event', 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', datetime('now', '+24 hours'))`); err == nil {
		t.Fatal("an unguarded degraded grant insert succeeded; want a fail-closed refusal")
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM degraded_grants`); n != 0 {
		t.Fatalf("degraded_grants rows after the refused insert = %d; want 0", n)
	}
}

// M2: the ninth step's object definitions match the reference schema byte for
// byte, so a migrated spine hashes like a fresh one.
func TestDegradedGrantsDDLMatchesReferenceSchemaByteForByte(t *testing.T) {
	fresh := chainSpine(t)
	db := d03Spine(t)
	registry := ProductionMigrationRegistry()
	if _, err := Migrate(db, registry, CurrentSchemaID); err != nil {
		t.Fatalf("migrate = %v", err)
	}
	readObjects := func(d *sql.DB) []string {
		rows, err := d.Query(`SELECT name, sql FROM sqlite_master
			WHERE name IN ('degraded_grants','degraded_grants_risk_scope','degraded_grants_guarded_insert')
			ORDER BY type, name`)
		if err != nil {
			t.Fatalf("query degraded grants objects: %v", err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var name, definition string
			if err := rows.Scan(&name, &definition); err != nil {
				t.Fatalf("scan degraded grants object: %v", err)
			}
			out = append(out, name+"|"+definition)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate degraded grants objects: %v", err)
		}
		return out
	}
	freshRows := readObjects(fresh)
	migratedRows := readObjects(db)
	if len(freshRows) == 0 || len(freshRows) != len(migratedRows) {
		t.Fatalf("degraded grants object counts = %d vs %d; want equal and nonzero", len(freshRows), len(migratedRows))
	}
	for i := range freshRows {
		if freshRows[i] != migratedRows[i] {
			t.Fatalf("degraded grants object %d diverges:\n fresh: %s\nmigrated: %s", i, freshRows[i], migratedRows[i])
		}
	}
}
