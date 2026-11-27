package substrate

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// stripAnchorCheckpoints removes the eleventh step's objects, reproducing a
// spine provisioned by a release before the anchoring registry existed.
func stripAnchorCheckpoints(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS anchor_checkpoints_no_delete`); err != nil {
		t.Fatalf("strip anchor_checkpoints_no_delete: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS anchor_checkpoints_no_update`); err != nil {
		t.Fatalf("strip anchor_checkpoints_no_update: %v", err)
	}
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS anchor_checkpoints_guarded_insert`); err != nil {
		t.Fatalf("strip anchor_checkpoints_guarded_insert: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS anchor_checkpoints`); err != nil {
		t.Fatalf("strip anchor_checkpoints: %v", err)
	}
}

// d06Spine provisions a spine at the d06 identity: everything through the
// tenth step is in place, the anchoring registry is not, and the ledger
// records S1-S10.
func d06Spine(t *testing.T) *sql.DB {
	t.Helper()
	db := provisionProductionSpine(t, filepath.Join(t.TempDir(), "spine.db"), testUUID, d06SigningIdentity)
	t.Cleanup(func() { db.Close() })
	stripAnchorCheckpoints(t, db)
	stripSignoffs(t, db)
	registry := ProductionMigrationRegistry()
	for _, step := range registry[:10] {
		recordStep(t, db, step)
	}
	return db
}

// A1: the eleventh step lands the anchoring registry on a d06-baseline
// spine: only the new step applies, the objects are present, the ledger and
// the stamp follow, the spine passes the schema check, and the append-only
// lattice is live at once — a checkpoint row can never be rewritten or
// deleted (D22 anti-rollback).
func TestAnchorCheckpointsStepFromD06Baseline(t *testing.T) {
	db := d06Spine(t)
	registry := ProductionMigrationRegistry()
	report, err := Migrate(db, registry, CurrentSchemaID)
	if err != nil {
		t.Fatalf("migrate = %v", err)
	}
	if !report.Migrated || len(report.Applied) != 2 || report.Applied[0] != "add-anchor-checkpoints" || report.Applied[1] != "add-signoffs" {
		t.Fatalf("report = %+v; want the eleventh and twelfth steps applied", report)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'anchor_checkpoints'`); n != 1 {
		t.Fatalf("anchor_checkpoints table = %d; want 1", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name LIKE 'anchor_checkpoints_%'`); n != 3 {
		t.Fatalf("anchor registry triggers = %d; want 3", n)
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
	if _, err := db.Exec(`INSERT INTO anchor_checkpoints
		(checkpoint_seq, prev_digest, root_digest, algorithm_id, heads_digest, leaf_count, heads_json,
		 published_by, sign_request_id, artifact_digest)
		VALUES (1, ?, ?, 'shz-merkle-v1', ?, 0, '[]', 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
		        'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', ?)`,
		strings.Repeat("c", 64), strings.Repeat("4", 64), strings.Repeat("d", 64), strings.Repeat("e", 64)); err == nil {
		t.Fatal("an unguarded checkpoint insert succeeded; want the trigger refusal")
	}
	// The append-only lattice is live: with a real checkpoint row in place
	// (a zero-row UPDATE would never fire the row triggers), no update and
	// no delete pass.
	ctx := context.Background()
	if err := bindFor(db, ctx, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err != nil {
		t.Fatal(err)
	}
	// The users row only needs to satisfy its CHECK fence (this test never
	// logs in through it).
	if _, err := db.Exec(`INSERT INTO users (id, username, password_hash) VALUES
		('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'anchor-publisher', ?)`,
		"$2a$10$"+strings.Repeat("a", 53)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sign_requests
		(request_id, root_digest, algorithm_id, serializer_id, source_actor, scope, expires_at)
		VALUES ('bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', ?, 'shz-merkle-v1', 'shz-canon-v1',
		        'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'anchor', datetime('now', '+24 hours'))`,
		strings.Repeat("d", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO anchor_checkpoints
		(checkpoint_seq, prev_digest, root_digest, algorithm_id, heads_digest, leaf_count, heads_json,
		 published_by, sign_request_id, artifact_digest)
		VALUES (1, ?, ?, 'shz-merkle-v1', ?, 0, '[]', 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
		        'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', ?)`,
		strings.Repeat("c", 64), strings.Repeat("4", 64), strings.Repeat("d", 64), strings.Repeat("e", 64)); err != nil {
		t.Fatalf("the bound checkpoint insert was refused: %v", err)
	}
	if _, err := db.Exec(`UPDATE anchor_checkpoints SET root_digest = ?`, strings.Repeat("0", 64)); err == nil {
		t.Fatal("an anchor checkpoint accepted an update; the registry is append only")
	}
	if _, err := db.Exec(`DELETE FROM anchor_checkpoints`); err == nil {
		t.Fatal("an anchor checkpoint accepted a delete; the registry is append only")
	}
}

// bindFor binds one actor through a short transaction so the attribution
// trigger accepts rows attributed to it.
func bindFor(db *sql.DB, ctx context.Context, actor string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO trusted_ctx (actor, admin) VALUES (?, 0)
		ON CONFLICT (actor) DO UPDATE SET admin = excluded.admin`, actor); err != nil {
		return err
	}
	return tx.Commit()
}

// A2: the eleventh step's object definitions match the reference schema
// byte for byte, so a migrated spine hashes like a fresh one.
func TestAnchorCheckpointsDDLMatchesReferenceSchemaByteForByte(t *testing.T) {
	fresh := chainSpine(t)
	db := d06Spine(t)
	registry := ProductionMigrationRegistry()
	if _, err := Migrate(db, registry, CurrentSchemaID); err != nil {
		t.Fatalf("migrate = %v", err)
	}
	readObjects := func(d *sql.DB) []string {
		rows, err := d.Query(`SELECT name, sql FROM sqlite_master
			WHERE name IN ('anchor_checkpoints','anchor_checkpoints_guarded_insert',
			               'anchor_checkpoints_no_update','anchor_checkpoints_no_delete')
			ORDER BY type, name`)
		if err != nil {
			t.Fatalf("query anchor checkpoint objects: %v", err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var name, definition string
			if err := rows.Scan(&name, &definition); err != nil {
				t.Fatalf("scan anchor checkpoint object: %v", err)
			}
			out = append(out, name+"|"+definition)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate anchor checkpoint objects: %v", err)
		}
		return out
	}
	freshRows := readObjects(fresh)
	migratedRows := readObjects(db)
	if len(freshRows) == 0 || len(freshRows) != len(migratedRows) {
		t.Fatalf("anchor checkpoint object counts = %d vs %d; want equal and nonzero", len(freshRows), len(migratedRows))
	}
	for i := range freshRows {
		if freshRows[i] != migratedRows[i] {
			t.Fatalf("anchor checkpoint object %d diverges:\n fresh: %s\nmigrated: %s", i, freshRows[i], migratedRows[i])
		}
	}
}
