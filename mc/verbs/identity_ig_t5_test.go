package verbs

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"mc/substrate"
)

// T5-2: the seal holds at both points after the migration — the execution
// point (the trigger refuses the write to the spine) and the guard point
// (the identity event recorder refuses the retired actor before any SQL).
func TestSealRefusesAtGuardAndTrigger(t *testing.T) {
	db := igT1Spine(t)
	// Trigger point: a raw write attributed to the legacy actor is refused
	// at the moment of the write.
	if _, err := db.Exec(`INSERT INTO activity (actor, kind, detail) VALUES ('operator', 'task.added', '{}')`); err == nil ||
		!strings.Contains(err.Error(), "sealed") {
		t.Fatalf("raw legacy operator write = %v; want the seal refusal", err)
	}
	// Guard point: the identity event recorder refuses before the SQL runs,
	// and the refusal leaves no row behind.
	err := inTx(db, func(ctx context.Context, q Q) error {
		return recordActivity(ctx, q, "operator", "probe.kind", "subject", "detail")
	})
	if err == nil || !strings.Contains(err.Error(), "sealed") {
		t.Fatalf("guarded legacy operator event = %v; want the guard refusal", err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM activity WHERE kind = 'probe.kind'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("probe rows after the refused event = %d; want 0", n)
	}
	// The current owner actor writes through both points.
	if _, err := db.Exec(`INSERT INTO activity (actor, kind, detail) VALUES ('owner', 'task.added', '{}')`); err != nil {
		t.Fatalf("owner write refused: %v", err)
	}
}

// T5-3: the mapping registry round-trips its fields (IG-T5 evidence-field
// verification), resists re-registration, and is append only.
func TestLegacyMappingRegistrationAndEvidenceVerification(t *testing.T) {
	db := igT1Spine(t)
	req := IdentityLegacyMapRequest{
		LegacyActor: "operator",
		User:        "alice",
		Basis:       "deployment record",
		Nature:      "permission inheritance",
		Strength:    "no presumed behavioral identity",
		Evidence:    "independent evidence note",
	}
	res, err := IdentityLegacyMap(db, nil, req)
	if err != nil {
		t.Fatalf("mapping registration: %v", err)
	}
	if res.(map[string]any)["mapped"] != true {
		t.Fatalf("registration result = %v; want mapped", res)
	}
	var actor, mapped, basis, nature, strength, evidence string
	if err := db.QueryRow(`SELECT legacy_actor, mapped_user_id, basis, nature, strength, evidence
		FROM legacy_actor_mapping`).Scan(&actor, &mapped, &basis, &nature, &strength, &evidence); err != nil {
		t.Fatal(err)
	}
	var aliceID string
	if err := db.QueryRow(`SELECT id FROM users WHERE username = 'alice'`).Scan(&aliceID); err != nil {
		t.Fatal(err)
	}
	if actor != "operator" || mapped != aliceID || basis != req.Basis || nature != req.Nature ||
		strength != req.Strength || evidence != req.Evidence {
		t.Fatalf("mapping row = (%s, %s, %s, %s, %s, %s); want the registered values verbatim",
			actor, mapped, basis, nature, strength, evidence)
	}
	// Re-registration is refused: the association is governance record, not
	// mutable state.
	if _, err := IdentityLegacyMap(db, nil, req); err == nil {
		t.Fatal("a second registration of the same legacy actor succeeded")
	}
	// Class A: update and delete are refused by the lattice.
	if _, err := db.Exec(`UPDATE legacy_actor_mapping SET evidence = 'tampered'`); err == nil {
		t.Fatal("the mapping row accepted an update")
	}
	if _, err := db.Exec(`DELETE FROM legacy_actor_mapping`); err == nil {
		t.Fatal("the mapping row accepted a delete")
	}
	// Registration is attribution only: the seal does not lift because a
	// successor is registered.
	if _, err := db.Exec(`INSERT INTO activity (actor, kind, detail) VALUES ('operator', 'task.added', '{}')`); err == nil {
		t.Fatal("registering the successor unsealed the legacy actor")
	}
}

// T5-4: the identity bootstrap attributes its own event to the account it
// creates, never to the retired legacy actor.
func TestIdentityBootstrapSelfAttribution(t *testing.T) {
	db := igT1Spine(t)
	password := "fixture" + "pass-123"
	res, err := IdentityUserCreate(db, nil, IdentityUserCreateRequest{Username: "carol", Password: password})
	if err != nil {
		t.Fatalf("user create: %v", err)
	}
	userID := res.(map[string]any)["user_id"].(string)
	var actor, kind string
	if err := db.QueryRow(`SELECT actor, kind FROM activity WHERE kind = 'user.created'`).Scan(&actor, &kind); err != nil {
		t.Fatal(err)
	}
	if actor != userID {
		t.Fatalf("bootstrap event actor = %s; want the created account %s", actor, userID)
	}
}

// T5-5: onboarding an old spine carries its legacy operator rows through the
// whole migration verbatim, and the seal is live on the result — the legacy
// history is protected, and the legacy identity stops writing.
func TestOnboardCarriesLegacyRowsThroughTheSeal(t *testing.T) {
	spine := filepath.Join(t.TempDir(), "spine.db")
	buildPreLedgerSpineFile(t, spine, migrationTestUUID)

	db, err := substrate.Open(spine)
	if err != nil {
		t.Fatal(err)
	}
	// Legacy history as an old deployment would have left it.
	if _, err := db.Exec(`INSERT INTO activity (actor, kind, detail) VALUES ('operator', 'task.added', 'legacy sentinel')`); err != nil {
		db.Close()
		t.Fatalf("seed legacy row: %v", err)
	}
	db.Close()

	req := OnboardSpineRequest{ProtocolVersion: 1, SchemaID: substrate.CurrentSchemaID, MirrorState: "present", MirrorUUID: migrationTestUUID}
	result, err := OnboardSpine(spine, req)
	if err != nil {
		t.Fatalf("OnboardSpine = %v", err)
	}
	if result.Status != "migrated" {
		t.Fatalf("result = %+v; want status migrated", result)
	}

	db, err = substrate.OpenReadOnly(spine)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var actor, detail string
	if err := db.QueryRow(`SELECT actor, detail FROM activity WHERE detail = 'legacy sentinel'`).Scan(&actor, &detail); err != nil {
		t.Fatalf("legacy row lost across the migration: %v", err)
	}
	if actor != "operator" || detail != "legacy sentinel" {
		t.Fatalf("legacy row = (%s, %s); want it verbatim (zero rewrite)", actor, detail)
	}
	var ledgerRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_history`).Scan(&ledgerRows); err != nil || ledgerRows != 9 {
		t.Fatalf("ledger rows = %d (%v); want 9", ledgerRows, err)
	}
	// The seal is live on the onboarded spine: a new legacy write is refused.
	dbRw, err := substrate.Open(spine)
	if err != nil {
		t.Fatal(err)
	}
	defer dbRw.Close()
	if _, err := dbRw.Exec(`INSERT INTO activity (actor, kind, detail) VALUES ('operator', 'task.added', '{}')`); err == nil {
		t.Fatal("the onboarded spine still accepts new legacy operator writes")
	}
}
