package verbs

import (
	"context"
	"database/sql"
	"fmt"

	"mc/identity"
)

// Session TTL defaults (IG §2: absolute + idle; the spine clock computes
// the instants, the host passes no times).
const (
	sessionAbsoluteTTL = "+24 hours"
	sessionIdleTTL     = "+30 minutes"
)

// IdentityUserCreateRequest bootstraps a P4 account. It is the deployment
// operator's front door, not a P4-authenticated path: no session exists yet
// to authenticate with, and the operator plane (host scope) is what stands
// behind it. The bootstrap lock after first use is a follow-up surface.
type IdentityUserCreateRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// IdentityUserCreate creates a user account and its Class A event.
func IdentityUserCreate(db *sql.DB, id *RunIdentity, a IdentityUserCreateRequest) (any, error) {
	if err := RequireHostScope(id, "identity user creation"); err != nil {
		return nil, err
	}
	if len(a.Username) < 3 || len(a.Username) > 64 {
		return nil, Usagef("The identity username is invalid.")
	}
	if len(a.Password) < 8 {
		return nil, Usagef("The identity password is too short.")
	}
	passwordHash, err := identity.HashPassword(a.Password)
	if err != nil {
		return nil, Usagef("The identity password could not be stored.")
	}
	userID, err := identity.NewOpaqueID()
	if err != nil {
		return nil, Usagef("The identity could not be allocated.")
	}
	var created bool
	if err := inTx(db, func(ctx context.Context, q Q) error {
		res, err := q.ExecContext(ctx, `INSERT INTO users (id, username, password_hash) VALUES (?, ?, ?)`,
			userID, a.Username, passwordHash)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		created = n == 1
		if !created {
			return Domainf("The identity username is already taken.")
		}
		return recordActivity(ctx, q, userID, "user.created", userID, "An identity account was created.")
	}); err != nil {
		return nil, classify(err)
	}
	return map[string]any{"user_id": userID, "created": created}, nil
}

// IdentitySessionCreateRequest authenticates with the local password and
// returns a fresh session: the token is returned once, only its hash is
// stored (IG §2).
type IdentitySessionCreateRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// IdentitySessionCreate verifies the password, then creates the session
// under the guard-bound trusted context: the session's ownership must agree
// with the bound actor, which the object-scope trigger enforces.
func IdentitySessionCreate(db *sql.DB, id *RunIdentity, a IdentitySessionCreateRequest) (any, error) {
	if err := RequireHostScope(id, "identity session creation"); err != nil {
		return nil, err
	}
	token, tokenHash, err := identity.NewSessionToken()
	if err != nil {
		return nil, Usagef("The session token could not be generated.")
	}
	sessionID, err := identity.NewOpaqueID()
	if err != nil {
		return nil, Usagef("The session could not be allocated.")
	}
	out := map[string]any{"created": false}
	if err := inTx(db, func(ctx context.Context, q Q) error {
		var userID, storedHash string
		if err := q.QueryRowContext(ctx, `SELECT id, password_hash FROM users WHERE username = ?`, a.Username).
			Scan(&userID, &storedHash); err != nil {
			if err == sql.ErrNoRows {
				return Domainf("The identity account is unknown.")
			}
			return err
		}
		if !identity.CheckPassword(storedHash, a.Password) {
			return Domainf("The identity password is incorrect.")
		}
		if err := identity.BindTrustedContext(ctx, q, userID, false); err != nil {
			return err
		}
		if _, err := q.ExecContext(ctx, `INSERT INTO user_sessions
			(session_id, token_hash, user_id, state, absolute_expires_at, idle_expires_at, last_seen_at)
			VALUES (?, ?, ?, 'active', datetime('now', ?), datetime('now', ?), datetime('now'))`,
			sessionID, tokenHash, userID, sessionAbsoluteTTL, sessionIdleTTL); err != nil {
			return err
		}
		if err := recordActivity(ctx, q, userID, "session.created", sessionID, "A session was created for the identity."); err != nil {
			return err
		}
		out["session_id"] = sessionID
		out["token"] = token
		out["created"] = true
		return nil
	}); err != nil {
		return nil, classify(err)
	}
	return out, nil
}

// IdentitySessionRevokeRequest revokes sessions by scope: single (the
// caller's own session, named by its non-secret id), user (every session of
// the caller), global (refused on the D01 plane — it is a later admin path).
type IdentitySessionRevokeRequest struct {
	Token     string `json:"token"`
	Scope     string `json:"scope"`
	SessionID string `json:"session_id,omitempty"`
}

// sessionActor authenticates a caller from its session token: the actor is
// DERIVED from the stored hash, never read from the request (the trusted
// context must not re-trust an untrusted input). A session past either TTL
// is invalid; the lazy state flip is a follow-up surface.
func sessionActor(ctx context.Context, q Q, token string) (string, error) {
	tokenHash, err := identity.TokenHash(token)
	if err != nil {
		return "", Usagef("The session token is malformed.")
	}
	var actor, state string
	var absolute, idle string
	err = q.QueryRowContext(ctx, `SELECT user_id, state, absolute_expires_at, idle_expires_at
		FROM user_sessions WHERE token_hash = ?`, tokenHash).
		Scan(&actor, &state, &absolute, &idle)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", Domainf("The session token is unknown.")
		}
		return "", err
	}
	if state != "active" {
		return "", Domainf("The session is not active.")
	}
	var now string
	if err := q.QueryRowContext(ctx, `SELECT datetime('now')`).Scan(&now); err != nil {
		return "", err
	}
	if now >= absolute || now >= idle {
		return "", Domainf("The session has expired.")
	}
	return actor, nil
}

// IdentitySessionRevoke revokes sessions under the guard ruling and the
// guard-bound trusted context.
func IdentitySessionRevoke(db *sql.DB, id *RunIdentity, a IdentitySessionRevokeRequest) (any, error) {
	if err := RequireHostScope(id, "identity session revocation"); err != nil {
		return nil, err
	}
	guard := identity.NewGuard()
	revoked := 0
	derivedRevoked := 0
	if err := inTx(db, func(ctx context.Context, q Q) error {
		actor, err := sessionActor(ctx, q, a.Token)
		if err != nil {
			return err
		}
		decision := guard.DecideSessionLifecycle(actor, a.Scope)
		if decision.RequireSignoff || !decision.Allow {
			return Domainf("%s", decision.Reason)
		}
		if err := identity.BindTrustedContext(ctx, q, actor, false); err != nil {
			return err
		}
		var res sql.Result
		switch a.Scope {
		case "single":
			if a.SessionID == "" {
				return Usagef("The single-scope revocation names no session.")
			}
			res, err = q.ExecContext(ctx, `UPDATE user_sessions SET state = 'revoked', revoked_by = ?
				WHERE session_id = ? AND user_id = ? AND state = 'active'`, actor, a.SessionID, actor)
		case "user":
			res, err = q.ExecContext(ctx, `UPDATE user_sessions SET state = 'revoked', revoked_by = ?
				WHERE user_id = ? AND state = 'active'`, actor, actor)
			// A granter whose every active session is gone is invalid; the
			// grants they issued die in this same transaction, not at their
			// original TTL (IG §6 derived-grant ruling).
			if err == nil {
				var casc sql.Result
				casc, err = q.ExecContext(ctx, `UPDATE grants SET revoked_at = datetime('now')
					WHERE granted_by_user_id = ? AND revoked_at IS NULL`, actor)
				if err == nil {
					if n, derr := casc.RowsAffected(); derr == nil {
						derivedRevoked = int(n)
					}
				}
			}
		default:
			return Usagef("The revocation scope is invalid.")
		}
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err == nil {
			revoked = int(n)
		}
		detail := fmt.Sprintf("Revoked %d session(s) by scope %s.", revoked, a.Scope)
		if derivedRevoked > 0 {
			detail += fmt.Sprintf(" %d derived grant(s) invalidated with the granter.", derivedRevoked)
		}
		return recordActivity(ctx, q, actor, "session.revoked", a.SessionID, detail)
	}); err != nil {
		return nil, classify(err)
	}
	return map[string]any{"revoked": revoked, "grants_revoked": derivedRevoked}, nil
}

// IdentityGrantIssueRequest issues a capability grant under the guard ruling.
type IdentityGrantIssueRequest struct {
	Token    string `json:"token"`
	Grantee  string `json:"grantee"`
	Scope    string `json:"scope"`
	Resource string `json:"resource,omitempty"`
	TTLHours int    `json:"ttl_hours,omitempty"`
}

// IdentityGrantIssue decides through the guard (self/same-team allow,
// cross-team require-signoff, default deny) and writes the grant under the
// guard-bound trusted context: the attribution column must agree with the
// bound actor, which the object-scope trigger enforces.
func IdentityGrantIssue(db *sql.DB, id *RunIdentity, a IdentityGrantIssueRequest) (any, error) {
	if err := RequireHostScope(id, "identity grant issuance"); err != nil {
		return nil, err
	}
	if a.Scope == "" || len(a.Scope) > 128 {
		return nil, Usagef("The grant scope is invalid.")
	}
	if a.TTLHours <= 0 || a.TTLHours > 24*90 {
		return nil, Usagef("The grant duration is invalid.")
	}
	guard := identity.NewGuard()
	out := map[string]any{"created": false}
	if err := inTx(db, func(ctx context.Context, q Q) error {
		actor, err := sessionActor(ctx, q, a.Token)
		if err != nil {
			return err
		}
		var granteeID string
		if err := q.QueryRowContext(ctx, `SELECT id FROM users WHERE username = ?`, a.Grantee).Scan(&granteeID); err != nil {
			if err == sql.ErrNoRows {
				return Domainf("The grantee is unknown.")
			}
			return err
		}
		decision, err := guard.DecideGrantIssue(ctx, q, actor, granteeID)
		if err != nil {
			return err
		}
		if decision.RequireSignoff || !decision.Allow {
			return Domainf("%s", decision.Reason)
		}
		if err := identity.BindTrustedContext(ctx, q, actor, false); err != nil {
			return err
		}
		grantID, err := identity.NewOpaqueID()
		if err != nil {
			return Usagef("The grant could not be allocated.")
		}
		ttl := fmt.Sprintf("+%d hours", a.TTLHours)
		var resource any
		if a.Resource != "" {
			resource = a.Resource
		}
		if _, err := q.ExecContext(ctx, `INSERT INTO grants
			(grant_id, grantee_user_id, scope, resource, granted_by_user_id, effective_at, expires_at)
			VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now', ?))`,
			grantID, granteeID, a.Scope, resource, actor, ttl); err != nil {
			return err
		}
		if err := recordActivity(ctx, q, actor, "grant.issued", grantID, "A capability grant was issued."); err != nil {
			return err
		}
		out["grant_id"] = grantID
		out["created"] = true
		return nil
	}); err != nil {
		return nil, classify(err)
	}
	return out, nil
}

// IdentityGrantRevokeRequest revokes a grant the caller issued (or holds for
// itself); revoking another party's grant is a sign-off path, not a D01 one.
type IdentityGrantRevokeRequest struct {
	Token   string `json:"token"`
	GrantID string `json:"grant_id"`
}

// IdentityGrantRevoke stamps the grant revoked under the guard-bound context.
func IdentityGrantRevoke(db *sql.DB, id *RunIdentity, a IdentityGrantRevokeRequest) (any, error) {
	if err := RequireHostScope(id, "identity grant revocation"); err != nil {
		return nil, err
	}
	revoked := false
	if err := inTx(db, func(ctx context.Context, q Q) error {
		actor, err := sessionActor(ctx, q, a.Token)
		if err != nil {
			return err
		}
		var grantee, granter string
		err = q.QueryRowContext(ctx, `SELECT grantee_user_id, granted_by_user_id FROM grants
			WHERE grant_id = ? AND revoked_at IS NULL`, a.GrantID).Scan(&grantee, &granter)
		if err != nil {
			if err == sql.ErrNoRows {
				return Domainf("The grant is unknown or already revoked.")
			}
			return err
		}
		if grantee != actor && granter != actor {
			return Domainf("The grant belongs to another party; its revocation requires sign-off.")
		}
		if err := identity.BindTrustedContext(ctx, q, actor, false); err != nil {
			return err
		}
		if _, err := q.ExecContext(ctx, `UPDATE grants SET revoked_at = datetime('now') WHERE grant_id = ?`, a.GrantID); err != nil {
			return err
		}
		revoked = true
		return recordActivity(ctx, q, actor, "grant.revoked", a.GrantID, "A capability grant was revoked.")
	}); err != nil {
		return nil, classify(err)
	}
	return map[string]any{"revoked": revoked}, nil
}

// IdentityLegacyMapRequest registers a retired legacy actor's successor
// attribution (IG §7): which named user the legacy actor's standing
// transfers to, on what basis, with what strength, and the independent
// evidence where the owner holds stronger attribution.
type IdentityLegacyMapRequest struct {
	LegacyActor string `json:"legacy_actor"`
	User        string `json:"user"`
	Basis       string `json:"basis"`
	Nature      string `json:"nature"`
	Strength    string `json:"strength"`
	Evidence    string `json:"evidence,omitempty"`
}

// IdentityLegacyMap registers the legacy actor mapping under the guard-bound
// context: the mapping row and its Class A event commit together, and the
// row is append only thereafter.
func IdentityLegacyMap(db *sql.DB, id *RunIdentity, a IdentityLegacyMapRequest) (any, error) {
	if err := RequireHostScope(id, "legacy actor mapping"); err != nil {
		return nil, err
	}
	if a.LegacyActor == "" || len(a.LegacyActor) > 128 {
		return nil, Usagef("The legacy actor value is invalid.")
	}
	for _, field := range []struct {
		name, value string
	}{
		{"basis", a.Basis}, {"nature", a.Nature}, {"strength", a.Strength},
	} {
		if field.value == "" || len(field.value) > 256 {
			return nil, Usagef("The mapping %s is invalid.", field.name)
		}
	}
	if len(a.Evidence) > 4096 {
		return nil, Usagef("The mapping evidence is too long.")
	}
	mapped := false
	if err := inTx(db, func(ctx context.Context, q Q) error {
		var userID string
		err := q.QueryRowContext(ctx, `SELECT id FROM users WHERE username = ?`, a.User).Scan(&userID)
		if err != nil {
			if err == sql.ErrNoRows {
				return Domainf("The successor user is unknown.")
			}
			return err
		}
		var evidence any
		if a.Evidence != "" {
			evidence = a.Evidence
		}
		if _, err := q.ExecContext(ctx, `INSERT INTO legacy_actor_mapping
			(legacy_actor, mapped_user_id, basis, nature, strength, evidence)
			VALUES (?, ?, ?, ?, ?, ?)`,
			a.LegacyActor, userID, a.Basis, a.Nature, a.Strength, evidence); err != nil {
			return Domainf("The legacy actor is already mapped.")
		}
		mapped = true
		return recordActivity(ctx, q, userID, "legacy.mapped", a.LegacyActor,
			fmt.Sprintf("Legacy actor %s mapped to user %s.", a.LegacyActor, a.User))
	}); err != nil {
		return nil, classify(err)
	}
	return map[string]any{"mapped": mapped}, nil
}

// recordActivity lands the Class A event for the identity lifecycle on the
// caller's transaction. The legacy operator actor is refused at this guard
// point (IG §7 封存): the trigger is the execution-plane backstop, the guard
// is the decision-plane front stop, and neither re-trusts a writer that names
// the retired actor.
func recordActivity(ctx context.Context, q Q, actor, kind, subject, detail string) error {
	if actor == "operator" {
		return Domainf("The legacy operator actor is sealed; the event must be attributed to a current actor.")
	}
	if _, err := q.ExecContext(ctx, `INSERT INTO activity (actor, kind, subject, detail) VALUES (?, ?, ?, ?)`,
		actor, kind, subject, detail); err != nil {
		return fmt.Errorf("record identity %s event: %w", kind, err)
	}
	return nil
}
