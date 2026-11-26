package verbs

import (
	"context"
	"strings"
	"testing"

	"mc/identity"
)

// B1: the single-person emergency drill end to end (验收句 单人紧急例演练):
// one named subject revokes every active session of every user under the
// purpose statement, the derived grants die with the invalidated granters
// in the same transaction, and the dedicated high-audit-density event lands
// with the purpose verbatim.
func TestBreakGlassSinglePersonEmergencyDrill(t *testing.T) {
	db := igT2Spine(t)
	tokenAlice, _ := igT2Session(t, db, "alice")
	igT2Session(t, db, "bob")
	// A derived grant from alice to bob: it must die with the granter when
	// the global break-glass invalidates every subject.
	igT2Grant(t, db, tokenAlice, "bob", "read", "", 24)

	res, err := IdentitySessionRevoke(db, nil, IdentitySessionRevokeRequest{
		Token: tokenAlice, Scope: "global", Purpose: "emergency session exposure",
	})
	if err != nil {
		t.Fatalf("break-glass revocation: %v", err)
	}
	m := res.(map[string]any)
	if m["revoked"].(int) != 2 || m["grants_revoked"].(int) != 1 || m["break_glass"] != true {
		t.Fatalf("break-glass result = %v; want both sessions and the derived grant flipped, flagged break-glass", m)
	}
	// The cross-user row was revoked by the named subject.
	var revokedBy, state string
	if err := db.QueryRow(`SELECT revoked_by, state FROM user_sessions WHERE user_id = ?`, igUserB).Scan(&revokedBy, &state); err != nil {
		t.Fatal(err)
	}
	if state != "revoked" || revokedBy != igUserA {
		t.Fatalf("bob's session = (%s, %s); want revoked by the drill subject", state, revokedBy)
	}
	// The dedicated audit event carries the purpose verbatim.
	var kind, detail string
	if err := db.QueryRow(`SELECT kind, detail FROM activity WHERE kind = 'session.breakglass_revoked'`).Scan(&kind, &detail); err != nil {
		t.Fatalf("the break-glass audit event is missing: %v", err)
	}
	if !strings.Contains(detail, "emergency session exposure") {
		t.Fatalf("break-glass audit detail %q lacks the purpose statement", detail)
	}
	// The drill subject's own session died too: its token no longer
	// authenticates.
	if _, err := IdentityGrantIssue(db, nil, IdentityGrantIssueRequest{Token: tokenAlice, Grantee: "alice", Scope: "read", TTLHours: 24}); err == nil {
		t.Fatal("a request under the break-glassed subject's own session succeeded")
	}
}

// B2: the purpose statement is mandatory — a global revocation without it is
// refused before any state change (D23 目的申明).
func TestBreakGlassRequiresPurposeStatement(t *testing.T) {
	db := igT2Spine(t)
	token, _ := igT2Session(t, db, "alice")
	if _, err := IdentitySessionRevoke(db, nil, IdentitySessionRevokeRequest{Token: token, Scope: "global"}); err == nil {
		t.Fatal("a purposeless break-glass revocation succeeded")
	}
	var active int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_sessions WHERE state = 'active'`).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 1 {
		t.Fatalf("active sessions after the refused break-glass = %d; want 1 (zero state change)", active)
	}
}

// B3: the frozen list cannot be extended from the implementation side — the
// guard refuses every operation off the list (fail closed), and the
// trigger-side admin exemption is not self-authorizable: a raw global update
// with no context, or a non-admin bound context claiming another user, is
// refused at the execution point.
func TestBreakGlassListCannotBeExtended(t *testing.T) {
	db := igT2Spine(t)
	igT2Session(t, db, "alice")
	igT2Session(t, db, "bob")
	guard := identity.NewGuard()
	// Guard face: off-list operations are refused, the frozen one is allowed.
	if d := guard.DecideBreakGlass("project:delete"); d.Allow {
		t.Fatal("an off-list operation was allowed by the break-glass guard")
	}
	if d := guard.DecideBreakGlass("session-revoke:global"); !d.Allow {
		t.Fatal("the frozen break-glass operation was refused by its own guard")
	}
	// Trigger face, no context at all: the global state flip is refused.
	if _, err := db.Exec(`UPDATE user_sessions SET state = 'revoked' WHERE state = 'active'`); err == nil {
		t.Fatal("an unbound global session revocation passed the execution point")
	}
	// Trigger face, non-admin context for alice: bob's row cannot be flipped
	// under it (the admin exemption is not self-authorizable).
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	if err := identity.BindTrustedContext(context.Background(), conn, igUserA, false); err != nil {
		t.Fatalf("bind context: %v", err)
	}
	_, err = conn.ExecContext(context.Background(), `UPDATE user_sessions SET state = 'revoked', revoked_by = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
		WHERE user_id = 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb' AND state = 'active'`)
	if err == nil || !strings.Contains(err.Error(), "disagrees") {
		t.Fatalf("cross-user revocation under a non-admin context = %v; want the attribution refusal", err)
	}
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	var active int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_sessions WHERE state = 'active'`).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 2 {
		t.Fatalf("active sessions after the refused writes = %d; want 2 (nothing was revoked)", active)
	}
}

// B4: the ordinary self-scoped revocations are unchanged by the break-glass
// path — no purpose is required for them, and they reach only the caller's
// own sessions.
func TestOrdinaryRevocationUnaffectedByBreakGlass(t *testing.T) {
	db := igT2Spine(t)
	tokenAlice, _ := igT2Session(t, db, "alice")
	igT2Session(t, db, "bob")
	if _, err := IdentitySessionRevoke(db, nil, IdentitySessionRevokeRequest{Token: tokenAlice, Scope: "user"}); err != nil {
		t.Fatalf("user-scope revocation without a purpose: %v", err)
	}
	var activeForAlice, activeForBob int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_sessions WHERE user_id = ? AND state = 'active'`, igUserA).Scan(&activeForAlice); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_sessions WHERE user_id = ? AND state = 'active'`, igUserB).Scan(&activeForBob); err != nil {
		t.Fatal(err)
	}
	if activeForAlice != 0 || activeForBob != 1 {
		t.Fatalf("sessions after the ordinary revocation = (%d alice, %d bob); want (0, 1)", activeForAlice, activeForBob)
	}
	// And no break-glass audit event was produced by the ordinary path.
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM activity WHERE kind = 'session.breakglass_revoked'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("break-glass audit events after an ordinary revocation = %d; want 0", n)
	}
}
