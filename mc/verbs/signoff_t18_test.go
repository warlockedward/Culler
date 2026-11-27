package verbs

import (
	"database/sql"
	"testing"
)

// The signoff test environment: the D01 two-user spine, an isolated node
// home (the sign flow needs it for bootstrap), alice's session, and a seeded
// break-glass audit row whose executor is alice.
type signoffTestEnv struct {
	token string
}

func newSignoffTestEnv(t *testing.T) signTestEnv {
	t.Helper()
	env := newSignTestEnv(t)
	// Seed the break-glass audit row that D04 leaves behind: executor =
	// alice (the actor on the row).
	if _, err := env.spine.Exec(`INSERT INTO activity (actor, kind, detail) VALUES
		(?, 'session.breakglass_revoked', 'Break-glass global revocation (purpose: drill). Revoked 0 session(s).')`,
		igUserA); err != nil {
		t.Fatalf("seed the break-glass audit row: %v", err)
	}
	return env
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

// G1: the D18 structural redline — recording a signoff creates no demand.
// Every demand table's count is unchanged by the signoff, and the only new
// rows are the signoff itself and its Class A event.
func TestSignoffNeverCreatesDemand(t *testing.T) {
	env := newSignTestEnv(t)
	aliceToken, _ := igT2Session(t, env.spine, "alice")
	before := map[string]int{}
	for _, table := range []string{"tasks", "projects", "degraded_grants", "sign_requests", "runs", "requirements_revisions"} {
		var n int
		if err := env.spine.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
			t.Fatalf("count %s before: %v", table, err)
		}
		before[table] = n
	}
	if _, err := SignoffRecord(env.spine, nil, SignoffRecordRequest{
		Token: aliceToken, SubjectClass: "generic", SubjectRef: "subject-locator-1",
		Decision: "approved", Reason: "reviewed and accepted",
	}); err != nil {
		t.Fatalf("signoff record: %v", err)
	}
	for table, want := range before {
		var n int
		if err := env.spine.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
			t.Fatalf("count %s after: %v", table, err)
		}
		if n != want {
			t.Fatalf("%s count moved %d -> %d by a signoff; an approval must never create demand (D18)", table, want, n)
		}
	}
	var signoffs, events int
	if err := env.spine.QueryRow(`SELECT COUNT(*) FROM signoffs`).Scan(&signoffs); err != nil {
		t.Fatal(err)
	}
	if err := env.spine.QueryRow(`SELECT COUNT(*) FROM activity WHERE kind = 'signoff.recorded'`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if signoffs != 1 || events != 1 {
		t.Fatalf("signoff rows = %d, events = %d; want exactly one of each (the signoff's whole write face)", signoffs, events)
	}
}

// G2: anti-forgery — the approver is derived from the session token (never
// self-claimed), unguarded birth and rewrites are refused.
func TestSignoffAntiForgery(t *testing.T) {
	env := newSignTestEnv(t)
	aliceToken, _ := igT2Session(t, env.spine, "alice")
	if _, err := SignoffRecord(env.spine, nil, SignoffRecordRequest{
		Token: aliceToken, SubjectClass: "generic", SubjectRef: "s-1",
		Decision: "approved", Reason: "alice's own approval",
	}); err != nil {
		t.Fatalf("alice signoff: %v", err)
	}
	// The approver is the session-derived actor: alice's id, not anything
	// the request claimed (the request carries no approver field at all).
	var approvedBy string
	if err := env.spine.QueryRow(`SELECT approved_by FROM signoffs WHERE subject_ref = 's-1'`).Scan(&approvedBy); err != nil {
		t.Fatal(err)
	}
	if approvedBy != igUserA {
		t.Fatalf("approved_by = %s; want the session-derived actor %s", approvedBy, igUserA)
	}
	// Unguarded birth refused.
	if _, err := env.spine.Exec(`INSERT INTO signoffs
		(signoff_id, subject_class, subject_ref, approved_by, decision, reason)
		VALUES ('a3333333333333333333333333333333', 'generic', 'forged', 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'approved', 'forged')`); err == nil {
		t.Fatal("an unguarded signoff insert succeeded; want the trigger refusal")
	}
	// Rewrite refused.
	if _, err := env.spine.Exec(`UPDATE signoffs SET decision = 'rejected'`); err == nil {
		t.Fatal("a signoff accepted an update; the registry is append only")
	}
	// Delete refused.
	if _, err := env.spine.Exec(`DELETE FROM signoffs`); err == nil {
		t.Fatal("a signoff accepted a delete; the registry is append only")
	}
}

// G3: SoD for break-glass ratification — the executor may not approve their
// own operation (IG §3 hard rule), the requester is bound from the audit
// row (never from the request), and a different approver records cleanly.
func TestSignoffBreakGlassSoD(t *testing.T) {
	env := newSignoffTestEnv(t)
	aliceToken, _ := igT2Session(t, env.spine, "alice")
	bobToken, _ := igT2Session(t, env.spine, "bob")
	var rowID int64
	if err := env.spine.QueryRow(`SELECT id FROM activity WHERE kind = 'session.breakglass_revoked'`).Scan(&rowID); err != nil {
		t.Fatal(err)
	}
	subject := itoa(int(rowID))
	// Alice executed the break-glass operation: she cannot ratify it.
	if _, err := SignoffRecord(env.spine, nil, SignoffRecordRequest{
		Token: aliceToken, SubjectClass: "breakglass.ratification", SubjectRef: subject,
		Decision: "approved", Reason: "self ratification attempt",
	}); err == nil {
		t.Fatal("the executor ratified their own break-glass operation; SoD must refuse")
	}
	var n int
	if err := env.spine.QueryRow(`SELECT COUNT(*) FROM signoffs`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("signoff rows after the refused self-approval = %d; want 0", n)
	}
	// Bob (a different actor) ratifies: accepted, and requested_by is bound
	// to the executor from the audit row.
	res, err := SignoffRecord(env.spine, nil, SignoffRecordRequest{
		Token: bobToken, SubjectClass: "breakglass.ratification", SubjectRef: subject,
		Decision: "approved", Reason: "ratified after Authority review",
	})
	if err != nil {
		t.Fatalf("bob ratification: %v", err)
	}
	var requestedBy sql.NullString
	if err := env.spine.QueryRow(`SELECT requested_by FROM signoffs WHERE signoff_id = ?`,
		res.(map[string]any)["signoff_id"]).Scan(&requestedBy); err != nil {
		t.Fatal(err)
	}
	if !requestedBy.Valid || requestedBy.String != igUserA {
		t.Fatalf("requested_by = %v; want the executor %s bound from the audit row", requestedBy, igUserA)
	}
}

// G4: SoD is configurable-not-default — the generic class does not demand
// it, unknown classes are fail-closed, and a break-glass subject pointing
// at a nonexistent audit row is refused.
func TestSignoffClassPolicy(t *testing.T) {
	env := newSignTestEnv(t)
	aliceToken, _ := igT2Session(t, env.spine, "alice")
	// Generic: self-approval is allowed (SoD not default for this class).
	if _, err := SignoffRecord(env.spine, nil, SignoffRecordRequest{
		Token: aliceToken, SubjectClass: "generic", SubjectRef: "g-1",
		Decision: "approved", Reason: "generic self-recording, SoD not demanded",
	}); err != nil {
		t.Fatalf("generic signoff: %v", err)
	}
	// Unknown class: fail closed.
	if _, err := SignoffRecord(env.spine, nil, SignoffRecordRequest{
		Token: aliceToken, SubjectClass: "mystery.class", SubjectRef: "s",
		Decision: "approved", Reason: "whitelist probe",
	}); err == nil {
		t.Fatal("an unknown subject class was accepted; the whitelist must fail closed")
	}
	// Break-glass class with a nonexistent audit row: refused.
	if _, err := SignoffRecord(env.spine, nil, SignoffRecordRequest{
		Token: aliceToken, SubjectClass: "breakglass.ratification", SubjectRef: "999999",
		Decision: "approved", Reason: "phantom subject",
	}); err == nil {
		t.Fatal("a break-glass ratification with a nonexistent subject was accepted")
	}
}

// G5: the decision enum carries rejections too, and both forms are
// append-only records.
func TestSignoffRejectionRecorded(t *testing.T) {
	env := newSignTestEnv(t)
	aliceToken, _ := igT2Session(t, env.spine, "alice")
	if _, err := SignoffRecord(env.spine, nil, SignoffRecordRequest{
		Token: aliceToken, SubjectClass: "generic", SubjectRef: "r-1",
		Decision: "rejected", Reason: "the subject does not hold",
	}); err != nil {
		t.Fatalf("rejected signoff: %v", err)
	}
	var decision string
	if err := env.spine.QueryRow(`SELECT decision FROM signoffs WHERE subject_ref = 'r-1'`).Scan(&decision); err != nil {
		t.Fatal(err)
	}
	if decision != "rejected" {
		t.Fatalf("decision = %s; want rejected", decision)
	}
}
