package verbs

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"mc/substrate"
)

// vcSpine provisions a fresh current-schema spine with its meta row, for
// the chain-verification verb's tests.
func vcSpine(t *testing.T) *sql.DB {
	t.Helper()
	db, err := substrate.Open(filepath.Join(t.TempDir(), "spine.db"))
	if err != nil {
		t.Fatalf("open spine: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := substrate.Init(db); err != nil {
		t.Fatalf("init spine: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO meta (id, deployment_uuid, schema_id) VALUES (1, ?, ?)`,
		"0123456789abcdef0123456789abcdef", substrate.CurrentSchemaID); err != nil {
		t.Fatalf("stamp meta: %v", err)
	}
	return db
}

func vcReport(t *testing.T, db *sql.DB, aggregateID string) map[string]any {
	t.Helper()
	result, err := VerifyChain(db, aggregateID)
	if err != nil {
		t.Fatalf("VerifyChain = %v", err)
	}
	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result shape = %T; want the report map", result)
	}
	return out
}

// W9: the verify-chain verb over an empty table reports the EMPTY_ROOT
// constant with leaf_count 0; over a clean chain it reports the head and
// the root over it.
func TestVerifyChainReportsCleanSpine(t *testing.T) {
	db := vcSpine(t)
	out := vcReport(t, db, "")
	if out["status"] != "ok" {
		t.Fatalf("status = %v; want ok", out["status"])
	}
	if out["algorithm_id"] != "shz-merkle-v1" {
		t.Fatalf("algorithm_id = %v; want shz-merkle-v1", out["algorithm_id"])
	}
	if out["leaf_count"] != 0 {
		t.Fatalf("leaf_count = %v; want 0", out["leaf_count"])
	}
	// R1 of the root vector suite pins the EMPTY_ROOT constant.
	if out["root_hex"] != "ffa51affaa9e56fc860d273f2bd83aeddef7aae90e9aaa2120d7b69f7786a2ca" {
		t.Fatalf("root_hex = %v; want the EMPTY_ROOT constant", out["root_hex"])
	}
	if len(out["aggregates"].([]map[string]any)) != 0 {
		t.Fatalf("aggregates = %v; want none on an empty table", out["aggregates"])
	}

	if err := substrate.OpenAggregate(db, "run-1", "run", "mc", ""); err != nil {
		t.Fatalf("open aggregate: %v", err)
	}
	out = vcReport(t, db, "")
	if out["status"] != "ok" || out["leaf_count"] != 1 {
		t.Fatalf("report = status %v leaf_count %v; want ok over one head", out["status"], out["leaf_count"])
	}
	aggregates := out["aggregates"].([]map[string]any)
	if len(aggregates) != 1 || aggregates[0]["aggregate_id"] != "run-1" || aggregates[0]["seq_max"] != 1 {
		t.Fatalf("aggregates = %+v; want the run-1 head at seq 1", aggregates)
	}
	// The aggregate-filtered report covers only the named aggregate.
	filtered := vcReport(t, db, "other")
	if filtered["leaf_count"] != 0 || filtered["status"] != "ok" {
		t.Fatalf("filtered report = status %v leaf_count %v; want the empty-set result", filtered["status"], filtered["leaf_count"])
	}
}

// W9b: a corrupted chain is the report, not a refusal: the verb succeeds,
// status carries the finding, and the per-aggregate status marks the
// broken aggregate.
func TestVerifyChainReportsCorruptionAsFindings(t *testing.T) {
	db := vcSpine(t)
	if err := substrate.OpenAggregate(db, "run-1", "run", "mc", ""); err != nil {
		t.Fatalf("open aggregate: %v", err)
	}
	// A forged seq-2 row whose prev_hash is shape-legal but not the stored
	// head's chain_digest.
	_, err := db.Exec(`INSERT INTO audit_events (
		aggregate_id, seq, event_id, aggregate_type, event_type, serializer_id,
		occurred_at, recorded_at, prev_hash, event_digest, chain_digest,
		payload_family, payload_json
	) VALUES ('run-1', 2, ?, 'run', 'aggregate_open', 'shz-canon-v1',
		'2026-11-19T12:00:06.000000Z', '2026-11-19T12:00:06.000002Z', ?, ?, ?,
		'aggregate_open', '{"actor":"forged"}')`,
		strings.Repeat("7", 64), strings.Repeat("8", 64),
		strings.Repeat("9", 64), strings.Repeat("a", 63)+"b")
	if err != nil {
		t.Fatalf("insert forged row: %v", err)
	}

	out := vcReport(t, db, "")
	if out["status"] != "corrupted" {
		t.Fatalf("status = %v; want corrupted", out["status"])
	}
	findings := out["errors"].([]map[string]any)
	if len(findings) == 0 {
		t.Fatal("the corrupted chain carries no findings")
	}
	kinds := map[string]bool{}
	for _, f := range findings {
		kinds[f["kind"].(string)] = true
	}
	if !kinds["broken-chain"] {
		t.Fatalf("findings = %+v; want the broken-chain detection", findings)
	}
	aggregates := out["aggregates"].([]map[string]any)
	if len(aggregates) != 1 || aggregates[0]["status"] != "corrupted" {
		t.Fatalf("aggregates = %+v; want run-1 marked corrupted", aggregates)
	}
}

// W10 (declaration face): the report shape carries exactly the reserved
// fields — no whole-root disappearance finding, because that capability
// is not claimed without an independently anchored basis in place.
func TestVerifyChainReportCarriesNoDisappearanceField(t *testing.T) {
	db := vcSpine(t)
	if err := substrate.OpenAggregate(db, "run-1", "run", "mc", ""); err != nil {
		t.Fatalf("open aggregate: %v", err)
	}
	out := vcReport(t, db, "")
	reserved := map[string]bool{
		"status":       true,
		"algorithm_id": true,
		"leaf_count":   true,
		"root_hex":     true,
		"aggregates":   true,
		"errors":       true,
	}
	for key := range out {
		if !reserved[key] {
			t.Fatalf("report carries an unreserved field %q; the layer-2 disappearance finding is not claimed", key)
		}
	}
}
