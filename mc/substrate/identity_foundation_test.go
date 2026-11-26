package substrate

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// stripIdentityFoundation removes the seventh step's objects, reproducing a
// spine provisioned by a release before the identity foundation existed.
func stripIdentityFoundation(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS user_sessions_state_machine`); err != nil {
		t.Fatalf("strip user_sessions_state_machine: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS user_sessions_guarded_revoke`); err != nil {
		t.Fatalf("strip user_sessions_guarded_revoke: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS user_sessions_guarded_insert`); err != nil {
		t.Fatalf("strip user_sessions_guarded_insert: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS grants_guarded_insert`); err != nil {
		t.Fatalf("strip grants_guarded_insert: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS grants_grantee`); err != nil {
		t.Fatalf("strip grants_grantee: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS grants`); err != nil {
		t.Fatalf("strip grants: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS team_members`); err != nil {
		t.Fatalf("strip team_members: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS teams_name`); err != nil {
		t.Fatalf("strip teams_name: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS teams`); err != nil {
		t.Fatalf("strip teams: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS user_sessions_user`); err != nil {
		t.Fatalf("strip user_sessions_user: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS user_sessions`); err != nil {
		t.Fatalf("strip user_sessions: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS users_username`); err != nil {
		t.Fatalf("strip users_username: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS users`); err != nil {
		t.Fatalf("strip users: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS trusted_ctx`); err != nil {
		t.Fatalf("strip trusted_ctx: %v", err)
	}
}

// b08IdentitySpine provisions a spine at the b08 identity: everything
// through the sixth step is in place, the identity foundation is not, and
// the ledger records S1-S6.
func b08IdentitySpine(t *testing.T) *sql.DB {
	t.Helper()
	db := provisionProductionSpine(t, filepath.Join(t.TempDir(), "spine.db"), testUUID, b08MapIdentity)
	t.Cleanup(func() { db.Close() })
	stripIdentityFoundation(t, db)
	stripLegacyActorSealing(t, db)
	registry := ProductionMigrationRegistry()
	for _, step := range registry[:6] {
		recordStep(t, db, step)
	}
	return db
}

// I1: crossing from the b08 baseline lands the identity foundation and the
// legacy actor sealing: both steps apply, the objects are present, the
// ledger and the stamp follow, and the spine passes the schema check.
func TestIdentityFoundationStepFromB08Baseline(t *testing.T) {
	db := b08IdentitySpine(t)
	registry := ProductionMigrationRegistry()
	report, err := Migrate(db, registry, CurrentSchemaID)
	if err != nil {
		t.Fatalf("migrate = %v", err)
	}
	if !report.Migrated || len(report.Applied) != 2 || report.Applied[0] != "add-identity-foundation" || report.Applied[1] != "add-legacy-actor-sealing" {
		t.Fatalf("report = %+v; want the seventh and eighth steps applied", report)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name IN ('users','user_sessions','teams','team_members','grants','trusted_ctx')`); n != 6 {
		t.Fatalf("identity tables present = %d; want 6", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name IN ('grants_guarded_insert','user_sessions_guarded_insert','user_sessions_guarded_revoke','user_sessions_state_machine')`); n != 4 {
		t.Fatalf("object-scope guards present = %d; want 4", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'legacy_actor_mapping'`); n != 1 {
		t.Fatalf("legacy actor mapping table present = %d; want 1", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM schema_history`); n != 8 {
		t.Fatalf("ledger rows = %d; want 8", n)
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

// I2: the seventh step's object definitions match the reference schema byte
// for byte, so a migrated spine hashes like a fresh one.
func TestIdentityFoundationDDLMatchesReferenceSchemaByteForByte(t *testing.T) {
	fresh := chainSpine(t)
	db := b08IdentitySpine(t)
	registry := ProductionMigrationRegistry()
	if _, err := Migrate(db, registry, CurrentSchemaID); err != nil {
		t.Fatalf("migrate = %v", err)
	}
	readObjects := func(d *sql.DB) []string {
		rows, err := d.Query(`SELECT name, sql FROM sqlite_master
			WHERE (name IN ('users','user_sessions','teams','team_members','grants','trusted_ctx')
			       OR name IN ('users_username','teams_name','user_sessions_user','grants_grantee')
			       OR name IN ('grants_guarded_insert','user_sessions_guarded_insert','user_sessions_guarded_revoke','user_sessions_state_machine'))
			ORDER BY type, name`)
		if err != nil {
			t.Fatalf("query identity objects: %v", err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var name, definition string
			if err := rows.Scan(&name, &definition); err != nil {
				t.Fatalf("scan identity object: %v", err)
			}
			out = append(out, name+"|"+definition)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate identity objects: %v", err)
		}
		return out
	}
	freshRows := readObjects(fresh)
	migratedRows := readObjects(db)
	if len(freshRows) == 0 || len(freshRows) != len(migratedRows) {
		t.Fatalf("identity object counts = %d vs %d; want equal and nonzero", len(freshRows), len(migratedRows))
	}
	for i := range freshRows {
		if freshRows[i] != migratedRows[i] {
			t.Fatalf("identity object %d diverges:\n fresh: %s\nmigrated: %s", i, freshRows[i], migratedRows[i])
		}
	}
}
