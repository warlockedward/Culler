// Package identity is the P4 data-access identity plane (IG §1-§3): the
// single authorization decision plane (the mc guard), the trusted-context
// injection that lets object-scope triggers verify the acting party, and
// the session token mechanics. The plane is orthogonal to the run.json
// execution plane: the guard decides from P4 identity only, never from an
// execution tier (I3: identity is not authority).
package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Decision is the guard's verdict. It is the ONLY allow/deny/
// require-signoff decision in the system for this domain (D-IG-3):
// triggers and state machines enforce scope and state, they never decide.
type Decision struct {
	Allow          bool
	RequireSignoff bool
	Reason         string
}

// contextWriter is the statement surface the trusted context binds to:
// the connection the guard's write transaction runs on.
type contextWriter interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// BindTrustedContext binds the guard-decided actor to the current write
// transaction through the trusted_ctx lifecycle table: a single row, set
// here, read by the object-scope triggers, cleared by the write transaction's
// entry and terminus (inTx). The write lock serializes writers, so at the
// moment a trigger fires the bound row — if any — was set by this same
// transaction's guard; a writer that skipped the guard finds no row and is
// refused (fail closed). Binding a row is not authorization: the guard's
// decision happens in Go, before the bind.
func BindTrustedContext(ctx context.Context, q contextWriter, actorID string, admin bool) error {
	adminBit := 0
	if admin {
		adminBit = 1
	}
	if _, err := q.ExecContext(ctx, `INSERT INTO trusted_ctx (actor, admin) VALUES (?, ?)
		ON CONFLICT (actor) DO UPDATE SET admin = excluded.admin`, actorID, adminBit); err != nil {
		return fmt.Errorf("bind trusted context actor: %w", err)
	}
	return nil
}

// ClearTrustedContext removes the bound acting party. inTx calls it at the
// entry of every write transaction (discarding a row a crashed predecessor
// left behind) and at its terminus (discarding the row the just-finished
// transaction committed). It is the no-op safe form when nothing is bound.
func ClearTrustedContext(ctx context.Context, q contextWriter) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM trusted_ctx`); err != nil {
		return fmt.Errorf("clear trusted context: %w", err)
	}
	return nil
}

// ClearTrustedContextIfPresent removes the bound acting party when the
// trusted_ctx table exists. Spines provisioned before the identity
// foundation carry no table, and their write transactions must not fail on
// the clear — the probe makes the clear a safe no-op there.
func ClearTrustedContextIfPresent(ctx context.Context, q contextWriter) error {
	var present int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'trusted_ctx'`).Scan(&present); err != nil {
		return fmt.Errorf("probe trusted context table: %w", err)
	}
	if present == 0 {
		return nil
	}
	return ClearTrustedContext(ctx, q)
}

// NewSessionToken returns a fresh 256-bit high-entropy session token and its
// storage form: the SHA-256 hash of the token's raw bytes, which is the only
// form ever persisted (IG §2: the server stores only the hash of the token).
func NewSessionToken() (token string, tokenHash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate session token: %w", err)
	}
	token = hex.EncodeToString(buf)
	tokenHash, err = TokenHash(token)
	if err != nil {
		return "", "", err
	}
	return token, tokenHash, nil
}

// TokenHash derives a session token's stored form: the SHA-256 of the
// token's raw bytes. It is the single authority for that derivation —
// creation and verification must both go through it, so a token and its
// stored hash can never be computed against different byte strings.
func TokenHash(token string) (string, error) {
	raw, err := hex.DecodeString(token)
	if err != nil {
		return "", fmt.Errorf("decode session token: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// NewOpaqueID returns a fresh 128-bit hex identifier: session ids are
// non-secret locators, user and grant ids are opaque handles.
func NewOpaqueID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate identifier: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// HashPassword renders a local password in its stored form (IG §2:
// bcrypt; the salt is embedded in the hash and is not secret).
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword verifies a local password against its stored bcrypt form.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// Guard is the single decision plane for the identity domain.
type Guard struct {
	now func() time.Time
}

// NewGuard returns the guard on the production clock.
func NewGuard() *Guard { return &Guard{now: time.Now} }

// SetClock overrides the guard's clock (test surface).
func (g *Guard) SetClock(now func() time.Time) { g.now = now }

// SharedTeamMembers reports whether both users sit in at least one common
// team: membership gives discovery and entry, and same-team grants are the
// default-allowed axis of the minimal D01 policy.
func SharedTeamMembers(ctx context.Context, q interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}, a, b string) (bool, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM team_members m1
		JOIN team_members m2 ON m1.team_id = m2.team_id
		WHERE m1.user_id = ? AND m2.user_id = ?`, a, b).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("query shared team membership: %w", err)
	}
	return n > 0, nil
}

// DecideGrantIssue is the guard's ruling on issuing a grant. The minimal
// D01 policy (IG §1 relational constraint, I2): a session must be valid;
// self-grants and same-team grants pass; a cross-team grant requires
// sign-off rather than failing outright; the default is deny.
func (g *Guard) DecideGrantIssue(ctx context.Context, q interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}, actor, grantee string) (Decision, error) {
	if actor == "" || grantee == "" {
		return Decision{Reason: "the grant decision needs both parties"}, nil
	}
	if actor == grantee {
		return Decision{Allow: true, Reason: "self grant"}, nil
	}
	shared, err := SharedTeamMembers(ctx, q, actor, grantee)
	if err != nil {
		return Decision{}, err
	}
	if shared {
		return Decision{Allow: true, Reason: "shared team membership"}, nil
	}
	// Cross-team grants are permitted but require sign-off (the sign-off
	// plane is D04's; the guard states the requirement, it does not waive it).
	return Decision{RequireSignoff: true, Reason: "cross-team grant requires sign-off"}, nil
}

// DecideSessionLifecycle is the guard's ruling on revoking sessions.
// Single and user-scoped revocations reach only the caller's own sessions;
// the global emergency scope has no D01 path (it is a later admin plane, and
// the guard refuses it here rather than guess at it).
func (g *Guard) DecideSessionLifecycle(actor, scope string) Decision {
	switch scope {
	case "single", "user":
		return Decision{Allow: true, Reason: "self session revocation"}
	case "global":
		return Decision{Reason: "the global revocation scope has no D01 authorization path"}
	default:
		return Decision{Reason: "the revocation scope is unrecognized"}
	}
}

// DecideDegradedIssue is the guard's ruling on issuing a Degraded Operation
// Authorization. The mechanism is the named, session-authenticated user
// (TM §8.1: the deployment owner's designated named authorizing subject); the
// guard requires that subject to be present. Selecting WHO that subject is,
// and the separation-of-duties around self-approval, is a governance ruling
// (break-glass / sign-off plane), not a decision this plane makes.
func (g *Guard) DecideDegradedIssue(actor string) Decision {
	if actor == "" {
		return Decision{Reason: "a degraded authorization requires a named authorizing subject"}
	}
	return Decision{Allow: true, Reason: "named authorizing subject present"}
}

// GrantAllows is the consumption-side ruling on a capability grant: it
// answers, at the moment of the authorization decision, whether the actor
// currently holds a grant covering the scope and resource. The input is a
// live read of the spine only — no cached or derived state may enter the
// decision (IG §6 zero-cache ruling), so a revoked or expired grant is
// refused at the very next decision. The time bound is evaluated at `now`,
// rendered in the spine's own time form so the comparison matches the
// stored values.
func GrantAllows(ctx context.Context, q interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}, actorID, scope, resource string, now time.Time) (Decision, error) {
	nowStr := now.UTC().Format("2006-01-02 15:04:05")
	var n int
	err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM grants
		WHERE grantee_user_id = ? AND scope = ?
		  AND (resource IS NULL OR resource = ?)
		  AND revoked_at IS NULL
		  AND effective_at <= ? AND expires_at >= ?`,
		actorID, scope, resource, nowStr, nowStr).Scan(&n)
	if err != nil {
		return Decision{}, fmt.Errorf("query grant validity: %w", err)
	}
	if n > 0 {
		return Decision{Allow: true, Reason: "a valid grant covers the request"}, nil
	}
	return Decision{Reason: "no valid grant covers the request"}, nil
}

// DegradedAllows is the consumption-side ruling on a Degraded Operation
// Authorization (TM §8.1): at the moment of the degraded operation it
// answers whether a live row binds ALL five elements — the specific risk,
// the operation scope, the compensating control, the validity, and the
// revocation conditions — to this operation. The input is a live read of the
// spine only, so an expired or revoked authorization is refused at the very
// next decision. risk and scope match exactly (an authorization never extends
// beyond what was listed); resource matches the row's value or a NULL row
// (unbound to an object). Review opinions, AI judgment, and historical
// operation facts never enter this decision: only a stored, live, fully-bound
// row does. An empty table refuses the degraded operation without stopping
// anything else — the ordinary path never reads here.
func DegradedAllows(ctx context.Context, q interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}, risk, scope, resource string, now time.Time) (Decision, error) {
	nowStr := now.UTC().Format("2006-01-02 15:04:05")
	var n int
	err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM degraded_grants
		WHERE risk = ? AND scope = ?
		  AND (resource IS NULL OR resource = ?)
		  AND revoked_at IS NULL
		  AND expires_at >= ?`,
		risk, scope, resource, nowStr).Scan(&n)
	if err != nil {
		return Decision{}, fmt.Errorf("query degraded authorization: %w", err)
	}
	if n > 0 {
		return Decision{Allow: true, Reason: "a live degraded authorization binds every element"}, nil
	}
	return Decision{Reason: "no live degraded authorization binds every element"}, nil
}
