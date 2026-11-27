package substrate

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// stripSignoffs removes the twelfth step's objects, reproducing a spine
// provisioned by a release before the signoffs plane existed.
func stripSignoffs(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS signoffs_no_delete`); err != nil {
		t.Fatalf("strip signoffs_no_delete: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS signoffs_no_update`); err != nil {
		t.Fatalf("strip signoffs_no_update: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS signoffs_guarded_insert`); err != nil {
		t.Fatalf("strip signoffs_guarded_insert: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS signoffs_subject`); err != nil {
		t.Fatalf("strip signoffs_subject: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS signoffs`); err != nil {
		t.Fatalf("strip signoffs: %v", err)
	}
}

// d07Spine provisions a spine at the d07 identity: everything through the
// eleventh step is in place, the signoffs plane is not, and the ledger
// records S1-S11.
func d07Spine(t *testing.T) *sql.DB {
	t.Helper()
	db := provisionProductionSpine(t, filepath.Join(t.TempDir(), "spine.db"), testUUID, d07AnchorIdentity)
	t.Cleanup(func() { db.Close() })
	stripSignoffs(t, db)
	registry := ProductionMigrationRegistry()
	for _, step := range registry[:11] {
		recordStep(t, db, step)
	}
	return db
}

// SO1: the twelfth step lands the signoffs plane on a d07-baseline spine:
// only the new step applies, the objects are present, the ledger and the
// stamp follow, the spine passes the schema check, and the append-only
// lattice is live at once — a signoff row can never be rewritten or
// deleted (anti-forgery structural carrier).
func TestSignoffsStepFromD07Baseline(t *testing.T) {
	db := d07Spine(t)
	registry := ProductionMigrationRegistry()
	report, err := Migrate(db, registry, CurrentSchemaID)
	if err != nil {
		t.Fatalf("migrate = %v", err)
	}
	if !report.Migrated || len(report.Applied) != 1 || report.Applied[0] != "add-signoffs" {
		t.Fatalf("report = %+v; want only the twelfth step applied", report)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'signoffs'`); n != 1 {
		t.Fatalf("signoffs table = %d; want 1", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name LIKE 'signoffs_%'`); n != 3 {
		t.Fatalf("signoffs triggers = %d; want 3", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM schema_history`); n != 12 {
		t.Fatalf("ledger rows = %d; want 12", n)
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
	// Unguarded birth is refused (the attribution gate).
	if _, err := db.Exec(`INSERT INTO signoffs
		(signoff_id, subject_class, subject_ref, approved_by, decision, reason)
		VALUES ('a1111111111111111111111111111111', 'generic', 'subject', ?, 'approved', 'ok')`,
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err == nil {
		t.Fatal("an unguarded signoff insert succeeded; want the trigger refusal")
	}
	// The append-only lattice is live against a real row: seed the user and
	// sign request faces, bind the actor, land a row, then refuse update and
	// delete.
	ctx := context.Background()
	if err := bindFor(db, ctx, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users (id, username, password_hash) VALUES
		('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'signoff-recorder', ?)`,
		"$2a$10$"+strings.Repeat("a", 53)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO signoffs
		(signoff_id, subject_class, subject_ref, approved_by, decision, reason)
		VALUES ('a2222222222222222222222222222222', 'generic', 'subject', ?, 'approved', 'ok')`,
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err != nil {
		t.Fatalf("the bound signoff insert was refused: %v", err)
	}
	if _, err := db.Exec(`UPDATE signoffs SET decision = 'rejected'`); err == nil {
		t.Fatal("a signoff row accepted an update; the registry is append only")
	}
	if _, err := db.Exec(`DELETE FROM signoffs`); err == nil {
		t.Fatal("a signoff row accepted a delete; the registry is append only")
	}
}

// SO2: the twelfth step's object definitions match the reference schema
// byte for byte, so a migrated spine hashes like a fresh one.
func TestSignoffsDDLMatchesReferenceSchemaByteForByte(t *testing.T) {
	fresh := chainSpine(t)
	db := d07Spine(t)
	registry := ProductionMigrationRegistry()
	if _, err := Migrate(db, registry, CurrentSchemaID); err != nil {
		t.Fatalf("migrate = %v", err)
	}
	readObjects := func(d *sql.DB) []string {
		rows, err := d.Query(`SELECT name, sql FROM sqlite_master
			WHERE name IN ('signoffs','signoffs_subject','signoffs_guarded_insert',
			               'signoffs_no_update','signoffs_no_delete')
			ORDER BY type, name`)
		if err != nil {
			t.Fatalf("query signoff objects: %v", err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var name, definition string
			if err := rows.Scan(&name, &definition); err != nil {
				t.Fatalf("scan signoff object: %v", err)
			}
			out = append(out, name+"|"+definition)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate signoff objects: %v", err)
		}
		return out
	}
	freshRows := readObjects(fresh)
	migratedRows := readObjects(db)
	if len(freshRows) == 0 || len(freshRows) != len(migratedRows) {
		t.Fatalf("signoff object counts = %d vs %d; want equal and nonzero", len(freshRows), len(migratedRows))
	}
	for i := range freshRows {
		if freshRows[i] != migratedRows[i] {
			t.Fatalf("signoff object %d diverges:\n fresh: %s\nmigrated: %s", i, freshRows[i], migratedRows[i])
		}
	}
}
