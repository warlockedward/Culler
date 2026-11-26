package verbs

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"mc/identity"
	"mc/substrate"
)

const (
	igUserA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	igUserB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func igT1Spine(t *testing.T) *sql.DB {
	t.Helper()
	db, err := substrate.Open(filepath.Join(t.TempDir(), "spine.db"))
	if err != nil {
		t.Fatalf("open test spine: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := substrate.Init(db); err != nil {
		t.Fatalf("init test spine: %v", err)
	}
	hash, err := identity.HashPassword("correct-horse-battery")
	if err != nil {
		t.Fatalf("hash test password: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO users (id, username, password_hash) VALUES (?, 'alice', ?), (?, 'bob', ?)`,
		igUserA, hash, igUserB, hash); err != nil {
		t.Fatalf("seed test users: %v", err)
	}
	return db
}

func igT1Count(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	switch table {
	case "user_sessions":
		err := db.QueryRow(`SELECT COUNT(*) FROM user_sessions`).Scan(&n)
		if err != nil {
			t.Fatalf("count user_sessions: %v", err)
		}
	case "user_sessions_active":
		err := db.QueryRow(`SELECT COUNT(*) FROM user_sessions WHERE state = 'active'`).Scan(&n)
		if err != nil {
			t.Fatalf("count active user_sessions: %v", err)
		}
	case "grants":
		err := db.QueryRow(`SELECT COUNT(*) FROM grants`).Scan(&n)
		if err != nil {
			t.Fatalf("count grants: %v", err)
		}
	default:
		t.Fatalf("unknown count table %q", table)
	}
	return n
}

// IG-T1 ①: direct SQL bypass. A write that never went through the guard has
// no trusted context, so the object-scope guards refuse it; and a write
// whose attribution column disagrees with the bound actor is refused too —
// the trigger re-checks the guard's binding, it does not re-trust the
// writer's own claim.
func TestIGT1DirectSQLBypassFailsClosed(t *testing.T) {
	db := igT1Spine(t)

	// No context at all: the guard table is absent on this connection.
	if _, err := db.Exec(`INSERT INTO user_sessions
		(session_id, token_hash, user_id, state, absolute_expires_at, idle_expires_at, last_seen_at)
		VALUES ('cccccccccccccccccccccccccccccccc', ?, 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'active',
		        datetime('now', '+24 hours'), datetime('now', '+30 minutes'), datetime('now'))`,
		strings.Repeat("d", 64)); err == nil {
		t.Fatal("unguarded session insert succeeded; want a fail-closed refusal")
	}
	if _, err := db.Exec(`INSERT INTO grants
		(grant_id, grantee_user_id, scope, granted_by_user_id, effective_at, expires_at)
		VALUES ('eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'read',
		        'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', datetime('now'), datetime('now', '+24 hours'))`); err == nil {
		t.Fatal("unguarded grant insert succeeded; want a fail-closed refusal")
	}
	if n := igT1Count(t, db, "user_sessions"); n != 0 {
		t.Fatalf("user_sessions rows after refused inserts = %d; want 0", n)
	}

	// Context bound for A, but the rows claim B: attribution must agree with
	// the binding, not with the writer.
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
	if _, err := conn.ExecContext(context.Background(), `INSERT INTO user_sessions
		(session_id, token_hash, user_id, state, absolute_expires_at, idle_expires_at, last_seen_at)
		VALUES ('cccccccccccccccccccccccccccccccc', ?, 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', 'active',
		        datetime('now', '+24 hours'), datetime('now', '+30 minutes'), datetime('now'))`,
		strings.Repeat("d", 64)); err == nil || !strings.Contains(err.Error(), "disagrees") {
		t.Fatalf("mismatched session insert = %v; want the attribution refusal", err)
	}
	if _, err := conn.ExecContext(context.Background(), `INSERT INTO grants
		(grant_id, grantee_user_id, scope, granted_by_user_id, effective_at, expires_at)
		VALUES ('eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', 'read',
		        'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', datetime('now'), datetime('now', '+24 hours'))`); err == nil ||
		!strings.Contains(err.Error(), "disagrees") {
		t.Fatalf("mismatched grant insert = %v; want the attribution refusal", err)
	}
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
}

// IG-T1 ②: a context bound inside one write transaction does not authorize
// a later write transaction. The guard's binding lives for exactly the
// transaction that bound it; the next inTx starts from an unbound slate, so
// a write there that skipped the guard is refused.
func TestIGT1ContextDoesNotPersistAcrossTransactions(t *testing.T) {
	db := igT1Spine(t)

	// First transaction: the guarded path binds the actor and writes a
	// session for it. This commits.
	if err := inTx(db, func(ctx context.Context, q Q) error {
		if err := identity.BindTrustedContext(ctx, q, igUserA, false); err != nil {
			return err
		}
		_, err := q.ExecContext(ctx, `INSERT INTO user_sessions
			(session_id, token_hash, user_id, state, absolute_expires_at, idle_expires_at, last_seen_at)
			VALUES ('cccccccccccccccccccccccccccccccc', ?, ?, 'active',
			        datetime('now', '+24 hours'), datetime('now', '+30 minutes'), datetime('now'))`,
			strings.Repeat("d", 64), igUserA)
		return err
	}); err != nil {
		t.Fatalf("guarded session write = %v", err)
	}

	// Second transaction: no binding. A write attributed to the previous
	// actor must now be refused — the earlier context did not carry over.
	err := inTx(db, func(ctx context.Context, q Q) error {
		_, err := q.ExecContext(ctx, `INSERT INTO user_sessions
			(session_id, token_hash, user_id, state, absolute_expires_at, idle_expires_at, last_seen_at)
			VALUES ('dddddddddddddddddddddddddddddddd', ?, ?, 'active',
			        datetime('now', '+24 hours'), datetime('now', '+30 minutes'), datetime('now'))`,
			strings.Repeat("e", 64), igUserA)
		return err
	})
	if err == nil {
		t.Fatal("a later unbound transaction was authorized by the earlier one's context; want a fail-closed refusal")
	}
	if n := igT1Count(t, db, "user_sessions"); n != 1 {
		t.Fatalf("user_sessions rows = %d; want only the first transaction's row", n)
	}
}

// IG-T1 ③: exception and reuse cleanup. A transaction that bound a context
// and then failed must not leave that context behind: the rollback discards
// the binding, and the next transaction's entry clear guarantees a slate.
func TestIGT1ContextClearedOnFailureAndReuse(t *testing.T) {
	db := igT1Spine(t)

	// First write transaction: bind the context, then fail; the rollback must
	// discard the binding.
	err := inTx(db, func(ctx context.Context, q Q) error {
		if err := identity.BindTrustedContext(ctx, q, igUserA, false); err != nil {
			return err
		}
		return errors.New("injected failure")
	})
	if err == nil {
		t.Fatal("injected failure did not surface")
	}

	// Second write transaction: no binding, so the guarded write must be
	// refused — the failed transaction's actor did not survive.
	err = inTx(db, func(ctx context.Context, q Q) error {
		_, err := q.ExecContext(ctx, `INSERT INTO user_sessions
			(session_id, token_hash, user_id, state, absolute_expires_at, idle_expires_at, last_seen_at)
			VALUES ('cccccccccccccccccccccccccccccccc', ?, 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'active',
			        datetime('now', '+24 hours'), datetime('now', '+30 minutes'), datetime('now'))`,
			strings.Repeat("d", 64))
		return err
	})
	if err == nil {
		t.Fatal("the reused write path carried the failed transaction's context; want a fail-closed refusal")
	}
	if n := igT1Count(t, db, "user_sessions"); n != 0 {
		t.Fatalf("user_sessions rows = %d; want 0", n)
	}
}

// IG-T1 ④: guard and trigger scope agreement. Every write the trigger
// accepts is one the guard bound; every guard refusal stops before the
// write; and the trigger refuses what the guard never bound.
func TestIGT1GuardAndTriggerScopesAgree(t *testing.T) {
	db := igT1Spine(t)

	// (a) The guard-allow path lands: the real verb binds the context and the
	// write survives the trigger.
	created, err := IdentitySessionCreate(db, nil, IdentitySessionCreateRequest{Username: "alice", Password: "correct-horse-battery"})
	if err != nil {
		t.Fatalf("guarded session create = %v", err)
	}
	createdMap := created.(map[string]any)
	token := createdMap["token"].(string)
	if token == "" {
		t.Fatal("session create returned no token")
	}
	if n := igT1Count(t, db, "user_sessions_active"); n != 1 {
		t.Fatalf("active sessions after the guarded create = %d; want 1", n)
	}

	// (b) A guard refusal stops before any write: an unknown token revokes
	// nothing.
	if _, err := IdentitySessionRevoke(db, nil, IdentitySessionRevokeRequest{Token: "unknown", Scope: "user"}); err == nil {
		t.Fatal("revocation with an unknown token succeeded; want the guard refusal")
	}
	if n := igT1Count(t, db, "user_sessions_active"); n != 1 {
		t.Fatalf("active sessions after the refused revocation = %d; want 1 (unchanged)", n)
	}

	// (c) A require-signoff ruling is not an allow: a cross-team grant the
	// guard cannot place in a shared team is refused on the D01 plane.
	if _, err := IdentityGrantIssue(db, nil, IdentityGrantIssueRequest{
		Token: token, Grantee: "bob", Scope: "read", TTLHours: 24,
	}); err == nil || !strings.Contains(err.Error(), "sign-off") {
		t.Fatalf("cross-team grant issue = %v; want the sign-off refusal", err)
	}
	if n := igT1Count(t, db, "grants"); n != 0 {
		t.Fatalf("grants after the sign-off refusal = %d; want 0", n)
	}

	// (d) The same grant inside a shared team passes the guard and the
	// trigger, attributed to the bound actor.
	if _, err := db.Exec(`INSERT INTO teams (id, name) VALUES ('eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', 'team-one')`); err != nil {
		t.Fatal(err)
	}
	for _, user := range []string{igUserA, igUserB} {
		if _, err := db.Exec(`INSERT INTO team_members (team_id, user_id) VALUES ('eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', ?)`, user); err != nil {
			t.Fatal(err)
		}
	}
	issued, err := IdentityGrantIssue(db, nil, IdentityGrantIssueRequest{
		Token: token, Grantee: "bob", Scope: "read", TTLHours: 24,
	})
	if err != nil {
		t.Fatalf("same-team grant issue = %v", err)
	}
	if issued.(map[string]any)["created"] != true {
		t.Fatalf("same-team grant issue = %v; want created", issued)
	}
	var granter string
	if err := db.QueryRow(`SELECT granted_by_user_id FROM grants`).Scan(&granter); err != nil {
		t.Fatal(err)
	}
	if granter != igUserA {
		t.Fatalf("grant attributed to %s; want the guard-bound actor %s", granter, igUserA)
	}
}

// The session state machine moves one way only, and the stored token form is
// the hash, never the token.
func TestSessionStateOneWayAndTokenHashOnly(t *testing.T) {
	db := igT1Spine(t)
	created, err := IdentitySessionCreate(db, nil, IdentitySessionCreateRequest{Username: "alice", Password: "correct-horse-battery"})
	if err != nil {
		t.Fatalf("session create = %v", err)
	}
	createdMap := created.(map[string]any)
	token := createdMap["token"].(string)
	sessionID := createdMap["session_id"].(string)
	expectedHash, err := identity.TokenHash(token)
	if err != nil {
		t.Fatalf("derive expected token hash: %v", err)
	}
	var storedHash, state, userID string
	if err := db.QueryRow(`SELECT token_hash, state, user_id FROM user_sessions`).Scan(&storedHash, &state, &userID); err != nil {
		t.Fatal(err)
	}
	if storedHash != expectedHash {
		t.Fatal("the stored token form is not the token's stored-form hash")
	}
	if storedHash == token {
		t.Fatal("the token itself is stored; only its hash may be")
	}
	// Revoke through the guard path, then try to re-enter active by raw SQL.
	if _, err := IdentitySessionRevoke(db, nil, IdentitySessionRevokeRequest{Token: token, Scope: "single",
		SessionID: sessionID}); err != nil {
		t.Fatalf("self revocation = %v", err)
	}
	if _, err := db.Exec(`UPDATE user_sessions SET state = 'active' WHERE session_id = ?`, sessionID); err == nil {
		t.Fatal("a revoked session re-entered active; the state machine must refuse")
	}
	// A wrong password never creates a session.
	if _, err := IdentitySessionCreate(db, nil, IdentitySessionCreateRequest{Username: "alice", Password: "wrong-password"}); err == nil {
		t.Fatal("session create with a wrong password succeeded")
	}
	if n := igT1Count(t, db, "user_sessions"); n != 1 {
		t.Fatalf("sessions after the refused login = %d; want 1", n)
	}
}
