package verbs

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"mc/identity"
)

// igT2Spine is the D01 two-user spine plus a shared team, so same-team
// grants pass the guard without the sign-off plane.
func igT2Spine(t *testing.T) *sql.DB {
	t.Helper()
	db := igT1Spine(t)
	team := strings.Repeat("e", 32)
	if _, err := db.Exec(`INSERT INTO teams (id, name) VALUES (?, 'team-one')`, team); err != nil {
		t.Fatalf("seed team: %v", err)
	}
	for _, user := range []string{igUserA, igUserB} {
		if _, err := db.Exec(`INSERT INTO team_members (team_id, user_id) VALUES (?, ?)`, team, user); err != nil {
			t.Fatalf("seed team membership: %v", err)
		}
	}
	return db
}

// igT2Session seeds a live session for the named test user through the
// guard-bound path (the D01 object-scope insert trigger included): the T2/T8
// family exercises revocation and decision semantics, not the password
// authentication surface that the D01 family already covers.
func igT2Session(t *testing.T, db *sql.DB, username string) (token, sessionID string) {
	t.Helper()
	var actor string
	switch username {
	case "alice":
		actor = igUserA
	case "bob":
		actor = igUserB
	default:
		t.Fatalf("unknown fixture user %q", username)
	}
	token, tokenHash, err := identity.NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	sessionID, err = identity.NewOpaqueID()
	if err != nil {
		t.Fatal(err)
	}
	err = inTx(db, func(ctx context.Context, q Q) error {
		if err := identity.BindTrustedContext(ctx, q, actor, false); err != nil {
			return err
		}
		_, err := q.ExecContext(ctx, `INSERT INTO user_sessions
			(session_id, token_hash, user_id, state, absolute_expires_at, idle_expires_at, last_seen_at)
			VALUES (?, ?, ?, 'active', datetime('now', '+24 hours'), datetime('now', '+30 minutes'), datetime('now'))`,
			sessionID, tokenHash, actor)
		return err
	})
	if err != nil {
		t.Fatalf("seed session for %s: %v", username, err)
	}
	return token, sessionID
}

func igT2Grant(t *testing.T, db *sql.DB, token, grantee, scope, resource string, ttlHours int) string {
	t.Helper()
	issued, err := IdentityGrantIssue(db, nil, IdentityGrantIssueRequest{
		Token: token, Grantee: grantee, Scope: scope, Resource: resource, TTLHours: ttlHours,
	})
	if err != nil {
		t.Fatalf("grant issue %s/%s: %v", grantee, scope, err)
	}
	return issued.(map[string]any)["grant_id"].(string)
}

// R1: matrix row "user session" — the next protected request after the
// revocation is refused (IG §6: 下一次受保护请求即拒).
func TestRevocationMatrixSessionRow(t *testing.T) {
	db := igT2Spine(t)
	token, _ := igT2Session(t, db, "alice")
	// A protected request under the live session passes.
	igT2Grant(t, db, token, "alice", "read", "", 24)
	// Revoke that session.
	if _, err := IdentitySessionRevoke(db, nil, IdentitySessionRevokeRequest{Token: token, Scope: "user"}); err != nil {
		t.Fatalf("session revocation: %v", err)
	}
	// The next protected request with the same token is refused.
	if _, err := IdentityGrantIssue(db, nil, IdentityGrantIssueRequest{Token: token, Grantee: "alice", Scope: "read", TTLHours: 24}); err == nil {
		t.Fatal("a protected request under a revoked session succeeded; the matrix row demands refusal at the next request")
	}
}

// R2: matrix row "capability grant" — the next authorization decision after
// the revocation is refused, and the time bounds hold at the decision (IG §6:
// 下一次授权判定即拒).
func TestRevocationMatrixGrantRow(t *testing.T) {
	db := igT2Spine(t)
	token, _ := igT2Session(t, db, "alice")
	grantID := igT2Grant(t, db, token, "bob", "read", "", 24)
	ctx := context.Background()
	now := time.Now().UTC()
	if d, err := identity.GrantAllows(ctx, db, igUserB, "read", "", now); err != nil || !d.Allow {
		t.Fatalf("live grant decision = (%v, %v); want allow", d, err)
	}
	// Time bounds, decided at the injected clock: a grant is refused before
	// it is effective and after it expires, and allowed inside the window.
	if d, _ := identity.GrantAllows(ctx, db, igUserB, "read", "", now.Add(-time.Hour)); d.Allow {
		t.Fatal("a not-yet-effective grant was allowed")
	}
	if d, _ := identity.GrantAllows(ctx, db, igUserB, "read", "", now.Add(25*time.Hour)); d.Allow {
		t.Fatal("an expired grant was allowed")
	}
	// The resource form: a resource-bound grant covers its resource and no
	// other; an unbound grant (resource NULL) covers the scope.
	igT2Grant(t, db, token, "bob", "deploy", "/repo", 24)
	if d, _ := identity.GrantAllows(ctx, db, igUserB, "deploy", "/other", now); d.Allow {
		t.Fatal("a resource-bound grant covered a different resource")
	}
	if d, _ := identity.GrantAllows(ctx, db, igUserB, "deploy", "/repo", now); !d.Allow {
		t.Fatal("a resource-bound grant refused its own resource")
	}
	// The revocation: the next authorization decision is refused.
	if _, err := IdentityGrantRevoke(db, nil, IdentityGrantRevokeRequest{Token: token, GrantID: grantID}); err != nil {
		t.Fatalf("grant revocation: %v", err)
	}
	if d, _ := identity.GrantAllows(ctx, db, igUserB, "read", "", now); d.Allow {
		t.Fatal("a revoked grant was allowed at the next decision")
	}
}

// R3: a derived grant dies with its granter in the same transaction, not at
// its original TTL (IG §6 派生即失效).
func TestDerivedGrantDiesWithGranter(t *testing.T) {
	db := igT2Spine(t)
	token, _ := igT2Session(t, db, "alice")
	igT2Grant(t, db, token, "bob", "read", "", 24)
	res, err := IdentitySessionRevoke(db, nil, IdentitySessionRevokeRequest{Token: token, Scope: "user"})
	if err != nil {
		t.Fatalf("user-scope revocation: %v", err)
	}
	m := res.(map[string]any)
	if m["revoked"].(int) != 1 || m["grants_revoked"].(int) != 1 {
		t.Fatalf("revocation result = %v; want the session and the derived grant flipped together", m)
	}
	// Both state flips committed: the grant is stamped, and the original TTL
	// had not run out when it died.
	var revokedAt, expiresAt string
	if err := db.QueryRow(`SELECT revoked_at, expires_at FROM grants`).Scan(&revokedAt, &expiresAt); err != nil {
		t.Fatal(err)
	}
	if revokedAt == "" {
		t.Fatal("the derived grant carries no revocation stamp")
	}
	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	if expiresAt <= nowStr {
		t.Fatalf("the grant died at its own TTL (expires %s <= now %s); want the granter event's time", expiresAt, nowStr)
	}
	if d, _ := identity.GrantAllows(context.Background(), db, igUserB, "read", "", time.Now().UTC()); d.Allow {
		t.Fatal("the derived grant survived its granter's invalidation")
	}
	// The granter's token no longer authenticates anything.
	if _, err := IdentityGrantIssue(db, nil, IdentityGrantIssueRequest{Token: token, Grantee: "alice", Scope: "read", TTLHours: 24}); err == nil {
		t.Fatal("a request under the invalidated granter's token succeeded")
	}
}

// R4: revoking a single session does not invalidate the granter — other
// active sessions remain, and the grants do not die.
func TestSingleScopeRevocationDoesNotCascade(t *testing.T) {
	db := igT2Spine(t)
	tokenA, sessionA := igT2Session(t, db, "alice")
	tokenB, _ := igT2Session(t, db, "alice")
	igT2Grant(t, db, tokenA, "bob", "read", "", 24)
	if _, err := IdentitySessionRevoke(db, nil, IdentitySessionRevokeRequest{Token: tokenA, Scope: "single", SessionID: sessionA}); err != nil {
		t.Fatalf("single-scope revocation: %v", err)
	}
	var revokedAt sql.NullString
	if err := db.QueryRow(`SELECT revoked_at FROM grants`).Scan(&revokedAt); err != nil {
		t.Fatal(err)
	}
	if revokedAt.Valid {
		t.Fatal("a single-session revocation invalidated the granter's grants")
	}
	if d, _ := identity.GrantAllows(context.Background(), db, igUserB, "read", "", time.Now().UTC()); !d.Allow {
		t.Fatal("the surviving grant stopped covering its scope")
	}
	// The granter still holds access through the other session.
	if _, err := IdentityGrantIssue(db, nil, IdentityGrantIssueRequest{Token: tokenB, Grantee: "alice", Scope: "read", TTLHours: 24}); err != nil {
		t.Fatalf("a request under the surviving session: %v", err)
	}
}

// Z1: the decision plane keeps no ruling across invocations — two live
// decisions bracketing a revocation see the revocation (IG-T8: guard 无跨调用
// 复用判定).
func TestZeroCacheAcrossInvocations(t *testing.T) {
	db := igT2Spine(t)
	token, _ := igT2Session(t, db, "alice")
	grantID := igT2Grant(t, db, token, "bob", "read", "", 24)
	ctx := context.Background()
	now := time.Now().UTC()
	// Stable live reads, then the revocation in its own invocation, then the
	// next decision: a memoized allow would survive, a live read does not.
	if d, _ := identity.GrantAllows(ctx, db, igUserB, "read", "", now); !d.Allow {
		t.Fatal("the first decision refused a live grant")
	}
	if d, _ := identity.GrantAllows(ctx, db, igUserB, "read", "", now); !d.Allow {
		t.Fatal("the repeated decision drifted; the plane must read live every time")
	}
	if _, err := IdentityGrantRevoke(db, nil, IdentityGrantRevokeRequest{Token: token, GrantID: grantID}); err != nil {
		t.Fatalf("grant revocation: %v", err)
	}
	if d, _ := identity.GrantAllows(ctx, db, igUserB, "read", "", now); d.Allow {
		t.Fatal("the decision after the revocation reused the earlier allow")
	}
}

// Z2: inside one write transaction a decision after the revocation sees the
// revocation — the decision input is the live read, never a snapshot taken
// earlier in the call (IG-T8: 同调用内不以旧结果替代撤销后状态).
func TestZeroCacheWithinInvocation(t *testing.T) {
	db := igT2Spine(t)
	token, _ := igT2Session(t, db, "alice")
	igT2Grant(t, db, token, "bob", "read", "", 24)
	now := time.Now().UTC()
	err := inTx(db, func(ctx context.Context, q Q) error {
		if d, err := identity.GrantAllows(ctx, q, igUserB, "read", "", now); err != nil || !d.Allow {
			return sql.ErrNoRows // signal: the pre-revocation read must allow
		}
		if _, err := q.ExecContext(ctx, `UPDATE grants SET revoked_at = datetime('now')
			WHERE grantee_user_id = ? AND scope = 'read'`, igUserB); err != nil {
			return err
		}
		d, err := identity.GrantAllows(ctx, q, igUserB, "read", "", now)
		if err != nil {
			return err
		}
		if d.Allow {
			return sql.ErrNoRows // signal: the post-revocation read must refuse
		}
		return nil
	})
	if err != nil {
		t.Fatalf("the within-transaction decision sequence broke: %v", err)
	}
}

// Z3: a pooled connection reused by another actor carries no context — the
// entry clear runs on the reused connection, and a write claiming the
// previous actor is refused (IG-T8: 连接池无跨 Actor 污染).
func TestPoolReuseCarriesNoCrossActorContext(t *testing.T) {
	db := igT2Spine(t)
	db.SetMaxOpenConns(1)
	token, _ := igT2Session(t, db, "alice")
	igT2Grant(t, db, token, "bob", "read", "", 24) // alice's guarded write; the connection goes back to the pool
	// The next write transaction reuses the same physical connection. An
	// unbound write claiming alice's attribution must be refused: if alice's
	// context had leaked through the pool, the claim would have passed.
	err := inTx(db, func(ctx context.Context, q Q) error {
		_, err := q.ExecContext(ctx, `INSERT INTO user_sessions
			(session_id, token_hash, user_id, state, absolute_expires_at, idle_expires_at, last_seen_at)
			VALUES ('cccccccccccccccccccccccccccccccc', ?, 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'active',
			        datetime('now', '+24 hours'), datetime('now', '+30 minutes'), datetime('now'))`,
			strings.Repeat("d", 64))
		return err
	})
	if err == nil {
		t.Fatal("a reused pooled connection carried the previous actor's trusted context")
	}
}

// Z4: the execution point re-verifies at the moment of the write — a raw
// write that skipped the guard and any decision state is refused by the
// object-scope trigger (IG-T8: 执行点全部复验; the trigger consults the bound
// context, never a decision cache).
func TestExecutionPointReverifiesWithoutDecisionCache(t *testing.T) {
	db := igT2Spine(t)
	if _, err := db.Exec(`INSERT INTO grants
		(grant_id, grantee_user_id, scope, granted_by_user_id, effective_at, expires_at)
		VALUES ('eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', 'read',
		        'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', datetime('now'), datetime('now', '+24 hours'))`); err == nil {
		t.Fatal("an unguarded grant write passed the execution point; the trigger must refuse it")
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM grants`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("grants after the refused write = %d; want 0", n)
	}
}
