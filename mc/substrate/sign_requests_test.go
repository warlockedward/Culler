package substrate

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// stripSignRequests removes the tenth step's objects, reproducing a spine
// provisioned by a release before the signing request state machine
// existed.
func stripSignRequests(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS sign_requests_guarded_insert`); err != nil {
		t.Fatalf("strip sign_requests_guarded_insert: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS sign_requests_state_machine`); err != nil {
		t.Fatalf("strip sign_requests_state_machine: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS sign_requests_state`); err != nil {
		t.Fatalf("strip sign_requests_state: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS sign_requests_supersedes`); err != nil {
		t.Fatalf("strip sign_requests_supersedes: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS sign_requests`); err != nil {
		t.Fatalf("strip sign_requests: %v", err)
	}
}

// d05Spine provisions a spine at the d05 identity: everything through the
// ninth step is in place, the signing request state machine is not, and the
// ledger records S1-S9.
func d05Spine(t *testing.T) *sql.DB {
	t.Helper()
	db := provisionProductionSpine(t, filepath.Join(t.TempDir(), "spine.db"), testUUID, d05RegistryIdentity)
	t.Cleanup(func() { db.Close() })
	stripSignRequests(t, db)
	registry := ProductionMigrationRegistry()
	for _, step := range registry[:9] {
		recordStep(t, db, step)
	}
	return db
}

// S1: the tenth step lands the signing request state machine on a d05-
// baseline spine: only the new step applies, the objects are present, the
// ledger and the stamp follow, the spine passes the schema check, and the
// one-way machine is live at once.
func TestSignRequestsStepFromD05Baseline(t *testing.T) {
	db := d05Spine(t)
	registry := ProductionMigrationRegistry()
	report, err := Migrate(db, registry, CurrentSchemaID)
	if err != nil {
		t.Fatalf("migrate = %v", err)
	}
	if !report.Migrated || len(report.Applied) != 1 || report.Applied[0] != "add-sign-requests" {
		t.Fatalf("report = %+v; want only the tenth step applied", report)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'sign_requests'`); n != 1 {
		t.Fatalf("sign_requests table = %d; want 1", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name IN ('sign_requests_guarded_insert','sign_requests_state_machine')`); n != 2 {
		t.Fatalf("sign request triggers = %d; want 2", n)
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
	// The one-way machine is live at the moment the step lands.
	if _, err := db.Exec(`INSERT INTO sign_requests
		(request_id, root_digest, algorithm_id, serializer_id, source_actor, scope, expires_at)
		VALUES ('a1111111111111111111111111111111', ?, 'alg', 'ser', 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'anchor', datetime('now', '+24 hours'))`,
		strings.Repeat("b", 64)); err == nil {
		t.Fatal("an unguarded sign request insert succeeded; want a fail-closed refusal")
	}
}

// S2: the tenth step's object definitions match the reference schema byte
// for byte, so a migrated spine hashes like a fresh one.
func TestSignRequestsDDLMatchesReferenceSchemaByteForByte(t *testing.T) {
	fresh := chainSpine(t)
	db := d05Spine(t)
	registry := ProductionMigrationRegistry()
	if _, err := Migrate(db, registry, CurrentSchemaID); err != nil {
		t.Fatalf("migrate = %v", err)
	}
	readObjects := func(d *sql.DB) []string {
		rows, err := d.Query(`SELECT name, sql FROM sqlite_master
			WHERE name IN ('sign_requests','sign_requests_state','sign_requests_supersedes',
			               'sign_requests_guarded_insert','sign_requests_state_machine')
			ORDER BY type, name`)
		if err != nil {
			t.Fatalf("query sign request objects: %v", err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var name, definition string
			if err := rows.Scan(&name, &definition); err != nil {
				t.Fatalf("scan sign request object: %v", err)
			}
			out = append(out, name+"|"+definition)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate sign request objects: %v", err)
		}
		return out
	}
	freshRows := readObjects(fresh)
	migratedRows := readObjects(db)
	if len(freshRows) == 0 || len(freshRows) != len(migratedRows) {
		t.Fatalf("sign request object counts = %d vs %d; want equal and nonzero", len(freshRows), len(migratedRows))
	}
	for i := range freshRows {
		if freshRows[i] != migratedRows[i] {
			t.Fatalf("sign request object %d diverges:\n fresh: %s\nmigrated: %s", i, freshRows[i], migratedRows[i])
		}
	}
}
