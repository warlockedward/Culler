package verbs

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"mc/identity"
)

func dgIssue(t *testing.T, db *sql.DB, token, risk, scope, resource, compensation, revConds string, ttl int) (any, error) {
	t.Helper()
	return IdentityDegradedGrantIssue(db, nil, IdentityDegradedGrantIssueRequest{
		Token: token, Risk: risk, Scope: scope, Resource: resource,
		Compensation: compensation, RevocationConditions: revConds, TTLHours: ttl,
	})
}

const (
	dgRisk     = "RR-3"
	dgScope    = "network"
	dgComp     = "zero credentials on the egress path"
	dgRevConds = "any observed unauthorized egress event"
)

// G1: every one of the five elements is mandatory at issuance — a degraded
// authorization missing any element is refused before the write, and nothing
// lands (TM §8.1 五要件; 验收句 五要件缺项=拒).
func TestDegradedGrantRequiresAllFiveElements(t *testing.T) {
	db := igT1Spine(t)
	token, _ := igT2Session(t, db, "alice")
	for _, tc := range []struct {
		name   string
		mutate func(*IdentityDegradedGrantIssueRequest)
	}{
		{"risk missing", func(a *IdentityDegradedGrantIssueRequest) { a.Risk = "" }},
		{"scope missing", func(a *IdentityDegradedGrantIssueRequest) { a.Scope = "" }},
		{"compensation missing", func(a *IdentityDegradedGrantIssueRequest) { a.Compensation = "" }},
		{"revocation conditions missing", func(a *IdentityDegradedGrantIssueRequest) { a.RevocationConditions = "" }},
		{"validity missing", func(a *IdentityDegradedGrantIssueRequest) { a.TTLHours = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := IdentityDegradedGrantIssueRequest{
				Token: token, Risk: dgRisk, Scope: dgScope,
				Compensation: dgComp, RevocationConditions: dgRevConds, TTLHours: 24,
			}
			tc.mutate(&req)
			if _, err := IdentityDegradedGrantIssue(db, nil, req); err == nil {
				t.Fatalf("issuance with the %s succeeded; the five-element binding must refuse it", tc.name)
			}
			var n int
			if err := db.QueryRow(`SELECT COUNT(*) FROM degraded_grants`).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 0 {
				t.Fatalf("degraded_grants rows after the refused issuance = %d; want 0", n)
			}
		})
	}
}

// G2: the per-operation decision — a degraded operation is admitted only
// against a live row binding every element, and refused on any mismatch,
// expiry, or revocation (IG §3 判定作用于该项降级操作).
func TestDegradedAllowsDecisionMatrix(t *testing.T) {
	db := igT1Spine(t)
	token, _ := igT2Session(t, db, "alice")
	// An unbound (resource NULL) authorization covers the whole scope.
	res, err := dgIssue(t, db, token, dgRisk, dgScope, "", dgComp, dgRevConds, 24)
	if err != nil {
		t.Fatalf("issue unbound authorization: %v", err)
	}
	grantID := res.(map[string]any)["grant_id"].(string)
	// A resource-bound authorization covers only its object.
	if _, err := dgIssue(t, db, token, dgRisk, "egress", "ws-test", dgComp, dgRevConds, 24); err != nil {
		t.Fatalf("issue resource-bound authorization: %v", err)
	}
	ctx := context.Background()
	now := time.Now().UTC()
	decision := func(risk, scope, resource string, at time.Time) bool {
		d, err := identity.DegradedAllows(ctx, db, risk, scope, resource, at)
		if err != nil {
			t.Fatalf("degraded decision (%s/%s/%s): %v", risk, scope, resource, err)
		}
		return d.Allow
	}
	if !decision(dgRisk, dgScope, "any-object", now) {
		t.Fatal("a live unbound authorization refused its own scope")
	}
	if !decision(dgRisk, "egress", "ws-test", now) {
		t.Fatal("a resource-bound authorization refused its own resource")
	}
	if decision(dgRisk, "egress", "other-object", now) {
		t.Fatal("a resource-bound authorization extended to an unlisted object")
	}
	if decision("RR-4", dgScope, "", now) {
		t.Fatal("an authorization extended to an unlisted risk")
	}
	if decision(dgRisk, "other-scope", "", now) {
		t.Fatal("an authorization extended to an unlisted scope")
	}
	if decision(dgRisk, dgScope, "", now.Add(25*time.Hour)) {
		t.Fatal("an expired authorization was still admitted")
	}
	// Revocation: the per-operation decision refuses at the very next read.
	if _, err := IdentityDegradedGrantRevoke(db, nil, IdentityDegradedGrantRevokeRequest{Token: token, GrantID: grantID}); err != nil {
		t.Fatalf("revocation: %v", err)
	}
	if decision(dgRisk, dgScope, "", now) {
		t.Fatal("a revoked authorization was still admitted")
	}
}

// G3: an empty degraded registry authorizes nothing and stops nothing — the
// ordinary identity paths run unchanged (IG §3: 空表=无降级授权存在，非全局停机).
func TestEmptyDegradedTableStopsNothingOrdinary(t *testing.T) {
	db := igT1Spine(t)
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM degraded_grants`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("degraded_grants rows = %d (%v); want an empty table", n, err)
	}
	token, _ := igT2Session(t, db, "alice")
	if d, _ := identity.DegradedAllows(context.Background(), db, dgRisk, dgScope, "", time.Now().UTC()); d.Allow {
		t.Fatal("the empty table admitted a degraded operation")
	}
	// The ordinary paths: a session-authenticated grant still issues.
	if _, err := IdentityGrantIssue(db, nil, IdentityGrantIssueRequest{Token: token, Grantee: "alice", Scope: "read", TTLHours: 24}); err != nil {
		t.Fatalf("ordinary grant issuance over the empty degraded table: %v", err)
	}
}

// G4: attribution integrity — the row carries the guard-bound issuer, an
// unguarded insert is refused, and a writer that claims another issuer is
// refused (the trigger re-verifies the binding, it does not re-trust the
// writer's own claim).
func TestDegradedGrantAttributionIntegrity(t *testing.T) {
	db := igT1Spine(t)
	token, _ := igT2Session(t, db, "alice")
	_, err := dgIssue(t, db, token, dgRisk, dgScope, "", dgComp, dgRevConds, 24)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	var issuer string
	if err := db.QueryRow(`SELECT issued_by_user_id FROM degraded_grants`).Scan(&issuer); err != nil {
		t.Fatal(err)
	}
	if issuer != igUserA {
		t.Fatalf("issuer = %s; want the guard-bound actor %s", issuer, igUserA)
	}
	// Unguarded: no trusted context at all.
	if _, err := db.Exec(`INSERT INTO degraded_grants
		(grant_id, risk, scope, compensation, revocation_conditions, issued_by_user_id, expires_at)
		VALUES ('dddddddddddddddddddddddddddddddd', 'RR-3', 'network', 'zero credentials',
		        'any egress event', 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', datetime('now', '+24 hours'))`); err == nil ||
		!strings.Contains(err.Error(), "trusted context") {
		t.Fatalf("unguarded degraded grant insert = %v; want the fail-closed refusal", err)
	}
	// Mismatched: context bound for bob, the row claims alice as issuer.
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	if err := identity.BindTrustedContext(context.Background(), conn, igUserB, false); err != nil {
		t.Fatalf("bind context: %v", err)
	}
	_, err = conn.ExecContext(context.Background(), `INSERT INTO degraded_grants
		(grant_id, risk, scope, compensation, revocation_conditions, issued_by_user_id, expires_at)
		VALUES ('eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', 'RR-3', 'network', 'zero credentials',
		        'any egress event', 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', datetime('now', '+24 hours'))`)
	if err == nil || !strings.Contains(err.Error(), "disagrees") {
		t.Fatalf("mismatched degraded grant insert = %v; want the attribution refusal", err)
	}
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
}

// G5: the revocation channel — the issuer revokes; a second revocation and a
// cross-party revocation are both refused (the cross-party path is sign-off,
// not D05).
func TestDegradedGrantRevocationChannel(t *testing.T) {
	db := igT1Spine(t)
	tokenAlice, _ := igT2Session(t, db, "alice")
	tokenBob, _ := igT2Session(t, db, "bob")
	res, err := dgIssue(t, db, tokenAlice, dgRisk, dgScope, "", dgComp, dgRevConds, 24)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	grantID := res.(map[string]any)["grant_id"].(string)
	// Cross-party: bob cannot revoke alice's authorization on the D05 plane.
	if _, err := IdentityDegradedGrantRevoke(db, nil, IdentityDegradedGrantRevokeRequest{Token: tokenBob, GrantID: grantID}); err == nil ||
		!strings.Contains(err.Error(), "sign-off") {
		t.Fatalf("cross-party revocation = %v; want the sign-off refusal", err)
	}
	var revokedAt sql.NullString
	if err := db.QueryRow(`SELECT revoked_at FROM degraded_grants WHERE grant_id = ?`, grantID).Scan(&revokedAt); err != nil {
		t.Fatal(err)
	}
	if revokedAt.Valid {
		t.Fatal("the cross-party refusal stamped the revocation")
	}
	// The issuer revokes.
	if _, err := IdentityDegradedGrantRevoke(db, nil, IdentityDegradedGrantRevokeRequest{Token: tokenAlice, GrantID: grantID}); err != nil {
		t.Fatalf("issuer revocation: %v", err)
	}
	if err := db.QueryRow(`SELECT revoked_at FROM degraded_grants WHERE grant_id = ?`, grantID).Scan(&revokedAt); err != nil {
		t.Fatal(err)
	}
	if !revokedAt.Valid {
		t.Fatal("the issuer revocation did not stamp the row")
	}
	// A second revocation finds nothing to revoke.
	if _, err := IdentityDegradedGrantRevoke(db, nil, IdentityDegradedGrantRevokeRequest{Token: tokenAlice, GrantID: grantID}); err == nil {
		t.Fatal("a second revocation of the same authorization succeeded")
	}
}
