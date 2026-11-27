package substrate

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func stripActivityChainMap(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`DROP TABLE IF EXISTS activity_chain_map`); err != nil {
		t.Fatalf("strip activity_chain_map: %v", err)
	}
}

// b08Spine provisions a spine at the b09 identity: everything through the
// fifth step is in place, the activity chain mapping is not, and the ledger
// records S1-S5.
func b08Spine(t *testing.T) *sql.DB {
	t.Helper()
	db := provisionProductionSpine(t, filepath.Join(t.TempDir(), "spine.db"), testUUID, b09ChainIdentity)
	t.Cleanup(func() { db.Close() })
	stripActivityChainMap(t, db)
	stripIdentityFoundation(t, db)
	stripLegacyActorSealing(t, db)
	stripDegradedGrants(t, db)
	stripSignRequests(t, db)
	stripAnchorCheckpoints(t, db)
	stripSignoffs(t, db)
	registry := ProductionMigrationRegistry()
	for _, step := range registry[:5] {
		recordStep(t, db, step)
	}
	return db
}

func seedActivityRow(t *testing.T, db *sql.DB, actor, kind string, subject, detail *string, createdAt string) int64 {
	t.Helper()
	var subjectVal, detailVal any
	if subject != nil {
		subjectVal = *subject
	}
	if detail != nil {
		detailVal = *detail
	}
	res, err := db.Exec(`INSERT INTO activity (actor, kind, subject, detail, created_at) VALUES (?, ?, ?, ?, ?)`,
		actor, kind, subjectVal, detailVal, createdAt)
	if err != nil {
		t.Fatalf("seed activity row %s/%s: %v", actor, kind, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("read seeded activity id: %v", err)
	}
	return id
}

func strPtr(s string) *string { return &s }

// activityDigestUpTo folds the same row identity over the pre-existing rows
// (id <= maxID): a successful migration adds its own bookkeeping activity
// row with a larger id, and the zero-rewrite claim is about the history.
func activityDigestUpTo(t *testing.T, db *sql.DB, maxID int64) string {
	t.Helper()
	rows, err := db.Query(`SELECT id, actor, kind, subject, detail, dispatch_key,
		dispatch_request_id, dispatch_result, project, created_at
		FROM activity WHERE id <= ? ORDER BY id ASC`, maxID)
	if err != nil {
		t.Fatalf("read activity for the digest: %v", err)
	}
	defer rows.Close()
	var parts []string
	for rows.Next() {
		var id int64
		var actor, kind, createdAt string
		var subject, detail, dispatchKey, dispatchRequestID, dispatchResult, project sql.NullString
		if err := rows.Scan(&id, &actor, &kind, &subject, &detail, &dispatchKey, &dispatchRequestID, &dispatchResult, &project, &createdAt); err != nil {
			t.Fatalf("scan activity row: %v", err)
		}
		parts = append(parts, strings.Join([]string{
			strconv.FormatInt(id, 10), actor, kind, nullValue(subject), nullValue(detail), nullValue(dispatchKey),
			nullValue(dispatchRequestID), nullValue(dispatchResult), nullValue(project), createdAt,
		}, "|"))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate activity rows: %v", err)
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

// activityDigest is the order-independent identity of the whole activity
// table: every column of every row, folded. The zero-rewrite proof
// compares it across the migration.
func activityDigest(t *testing.T, db *sql.DB) string {
	t.Helper()
	rows, err := db.Query(`SELECT id, actor, kind, subject, detail, dispatch_key,
		dispatch_request_id, dispatch_result, project, created_at
		FROM activity ORDER BY id ASC`)
	if err != nil {
		t.Fatalf("read activity for the digest: %v", err)
	}
	defer rows.Close()
	var parts []string
	for rows.Next() {
		var id int64
		var actor, kind, createdAt string
		var subject, detail, dispatchKey, dispatchRequestID, dispatchResult, project sql.NullString
		if err := rows.Scan(&id, &actor, &kind, &subject, &detail, &dispatchKey, &dispatchRequestID, &dispatchResult, &project, &createdAt); err != nil {
			t.Fatalf("scan activity row: %v", err)
		}
		parts = append(parts, strings.Join([]string{
			strconv.FormatInt(id, 10), actor, kind, nullValue(subject), nullValue(detail), nullValue(dispatchKey),
			nullValue(dispatchRequestID), nullValue(dispatchResult), nullValue(project), createdAt,
		}, "|"))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate activity rows: %v", err)
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

func nullValue(v sql.NullString) string {
	if !v.Valid {
		return "\x00NULL"
	}
	return v.String
}

// N1: the sixth step projects every pre-existing activity row into the
// activity aggregate, one chain row per activity row, in id order, with the
// row's own values verbatim in the payload.
func TestActivityBackfillProjectsStockRowsInOrder(t *testing.T) {
	db := b08Spine(t)
	seedActivityRow(t, db, "operator", "probe.kind.a", strPtr("subject-a"), strPtr("detail-a"), "2026-11-20 03:04:05")
	seedActivityRow(t, db, "mc", "probe.kind.b", nil, nil, "2026-11-20 03:04:06")
	registry := ProductionMigrationRegistry()
	report, err := Migrate(db, registry, CurrentSchemaID)
	if err != nil {
		t.Fatalf("migrate = %v", err)
	}
	if !report.Migrated || len(report.Applied) != 7 || report.Applied[0] != "migrate-activity-to-chain" || report.Applied[1] != "add-identity-foundation" || report.Applied[2] != "add-legacy-actor-sealing" || report.Applied[3] != "add-degraded-grants" || report.Applied[4] != "add-sign-requests" || report.Applied[5] != "add-anchor-checkpoints" || report.Applied[6] != "add-signoffs" {
		t.Fatalf("report = %+v; want the sixth through twelfth steps applied", report)
	}
	// Post-B10 the whole table is chained: the stock rows through the
	// backfill, the migration steps' own rows through the central append.
	// Full coverage (table == chain == mapping) is the spine's invariant, and
	// the stock pair still owns the first two chain positions (below).
	stockAndSteps := countRows(t, db, `SELECT COUNT(*) FROM activity`)
	if n := countRows(t, db, `SELECT COUNT(*) FROM audit_events WHERE aggregate_id = 'activity'`); n != stockAndSteps {
		t.Fatalf("activity chain rows = %d; want %d (every row chained: stock through the backfill, steps through the append)", n, stockAndSteps)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM activity_chain_map`); n != stockAndSteps {
		t.Fatalf("mapping rows = %d; want %d (one per chained row)", n, stockAndSteps)
	}
	var firstPrev string
	if err := db.QueryRow(`SELECT prev_hash FROM audit_events WHERE aggregate_id = 'activity' AND seq = 1`).Scan(&firstPrev); err != nil {
		t.Fatal(err)
	}
	if firstPrev != GenesisHash("activity") {
		t.Fatalf("first activity chain prev_hash = %s; want the activity genesis", firstPrev)
	}
	var firstType string
	if err := db.QueryRow(`SELECT event_type FROM audit_events WHERE aggregate_id = 'activity' AND seq = 1`).Scan(&firstType); err != nil {
		t.Fatal(err)
	}
	if firstType != "probe.kind.a" {
		t.Fatalf("first chain event_type = %s; want the activity kind in id order", firstType)
	}
	var payload string
	if err := db.QueryRow(`SELECT payload_json FROM audit_events WHERE aggregate_id = 'activity' AND seq = 1`).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"actor":"operator"`,
		`"kind":"probe.kind.a"`,
		`"subject":"subject-a"`,
		`"detail":"detail-a"`,
		`"created_at":"2026-11-20T03:04:05.000000Z"`,
	} {
		if !strings.Contains(payload, want) {
			t.Fatalf("first payload %s lacks %s", payload, want)
		}
	}
	verifierReport, err := VerifyAuditChain(db, "activity")
	if err != nil {
		t.Fatalf("verify activity chain: %v", err)
	}
	if len(verifierReport.Errors) != 0 {
		t.Fatalf("activity chain findings = %+v; want none", verifierReport.Errors)
	}
	// The mapping object matches the reference schema byte for byte.
	fresh := chainSpine(t)
	var migratedSQL, freshSQL string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE name = 'activity_chain_map'`).Scan(&migratedSQL); err != nil {
		t.Fatal(err)
	}
	if err := fresh.QueryRow(`SELECT sql FROM sqlite_master WHERE name = 'activity_chain_map'`).Scan(&freshSQL); err != nil {
		t.Fatal(err)
	}
	if migratedSQL != freshSQL {
		t.Fatalf("activity_chain_map definition diverges:\n migrated: %s\n fresh: %s", migratedSQL, freshSQL)
	}
}

// N2: the zero-rewrite comparison (IG §7 ticket face): the activity table's
// full content is identical before and after the migration, and the legacy
// actor survives into the chain payload verbatim.
func TestActivityBackfillRewritesNothing(t *testing.T) {
	db := b08Spine(t)
	seededID := seedActivityRow(t, db, "operator", "probe.legacy", strPtr("subject"), strPtr("detail"), "2026-11-19 22:11:00")
	before := activityDigestUpTo(t, db, seededID)
	registry := ProductionMigrationRegistry()
	if _, err := Migrate(db, registry, CurrentSchemaID); err != nil {
		t.Fatalf("migrate = %v", err)
	}
	after := activityDigestUpTo(t, db, seededID)
	if before != after {
		t.Fatalf("pre-existing activity rows changed across the migration (%s -> %s); the zero-rewrite proof failed", before, after)
	}
	var payload string
	if err := db.QueryRow(`SELECT payload_json FROM audit_events WHERE aggregate_id = 'activity' AND seq = 1`).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(payload, `"actor":"operator"`) {
		t.Fatalf("chain payload %s does not carry the legacy actor verbatim", payload)
	}
	if strings.Contains(payload, `"user"`) {
		t.Fatalf("chain payload %s re-authored the legacy actor; IG §7 forbids it", payload)
	}
}

// N3: a backfill the proof cannot stand behind refuses the whole step: the
// mapping table and its chain rows roll back, the ledger carries no sixth
// row, the stamp stays at the b09 identity, and the activity rows are
// untouched.
func TestActivityBackfillProofFailureRollsBack(t *testing.T) {
	db := b08Spine(t)
	seedActivityRow(t, db, "operator", "probe.kind.a", strPtr("subject"), strPtr("detail"), "2026-11-20 03:04:05")
	before := activityDigest(t, db)

	registry := ProductionMigrationRegistry()
	// The tampered sixth step creates the mapping table but chains nothing:
	// the coverage proof must fail and roll the step back.
	registry[5] = MigrationStep{
		Name:         registry[5].Name,
		Version:      registry[5].Version,
		FromSchemaID: registry[5].FromSchemaID,
		ToSchemaID:   registry[5].ToSchemaID,
		Checksum:     registry[5].Checksum,
		Statement:    registry[5].Statement,
		Verify:       VerifyActivityBackfill,
	}
	_, err := Migrate(db, registry, CurrentSchemaID)
	if err == nil || !errors.Is(err, ErrActivityBackfillProof) {
		t.Fatalf("migrate = %v; want the activity backfill proof refusal", err)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'activity_chain_map'`); n != 0 {
		t.Fatalf("activity_chain_map present after rollback = %d; want 0 (the DDL rolled back too)", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM audit_events WHERE aggregate_id = 'activity'`); n != 0 {
		t.Fatalf("activity chain rows after rollback = %d; want 0", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM schema_history WHERE version = 6`); n != 0 {
		t.Fatalf("ledger carries a sixth row after rollback; want none")
	}
	var stamped string
	if err := db.QueryRow(`SELECT schema_id FROM meta WHERE id = 1`).Scan(&stamped); err != nil {
		t.Fatal(err)
	}
	if stamped != b09ChainIdentity {
		t.Fatalf("meta.schema_id = %s after rollback; want the b09 identity untouched", stamped)
	}
	if activityDigest(t, db) != before {
		t.Fatal("the activity table changed despite the refused step")
	}
}

// N4: a spine whose ledger is complete at this release re-runs as a no-op:
// the recorded sixth step is skipped and the projection is not rebuilt.
func TestActivityMigrationIsIdempotentOnReRun(t *testing.T) {
	db := b08Spine(t)
	seedActivityRow(t, db, "operator", "probe.kind.a", strPtr("subject"), strPtr("detail"), "2026-11-20 03:04:05")
	registry := ProductionMigrationRegistry()
	first, err := Migrate(db, registry, CurrentSchemaID)
	if err != nil || !first.Migrated {
		t.Fatalf("first migrate = (%+v, %v)", first, err)
	}
	again, err := Migrate(db, registry, CurrentSchemaID)
	if err != nil {
		t.Fatalf("re-run migrate = %v", err)
	}
	if again.Migrated || len(again.Applied) != 0 {
		t.Fatalf("re-run report = %+v; want a no-op (the sixth step is recorded)", again)
	}
	// Post-B10 coverage with the no-op re-run: the stock row is projected
	// exactly once through the backfill, and the full coverage invariant —
	// table == chain == mapping — is untouched by the second call.
	var stockID int64
	if err := db.QueryRow(`SELECT id FROM activity WHERE kind = 'probe.kind.a'`).Scan(&stockID); err != nil {
		t.Fatal(err)
	}
	stockEvent := activityChainEventID(stockID)
	if n := countRows(t, db, `SELECT COUNT(*) FROM audit_events WHERE aggregate_id = 'activity' AND event_id = ?`, stockEvent); n != 1 {
		t.Fatalf("stock row chain events after re-run = %d; want exactly 1 (the projection is never rebuilt)", n)
	}
	total := countRows(t, db, `SELECT COUNT(*) FROM activity`)
	if n := countRows(t, db, `SELECT COUNT(*) FROM audit_events WHERE aggregate_id = 'activity'`); n != total {
		t.Fatalf("activity chain rows after re-run = %d; want %d (every row, once)", n, total)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM activity_chain_map`); n != total {
		t.Fatalf("mapping rows after re-run = %d; want %d", n, total)
	}
}
