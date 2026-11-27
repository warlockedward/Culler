package verbs

import (
	"context"
	"database/sql"

	"mc/identity"
)

// The signoffs plane (D08, D18): approvals are bound to specific subjects,
// recorded append-only, and never create demand — the recording
// transaction touches only the signoffs registry and its own Class A
// event. The approver is derived from the session token, never read from
// the request (anti-forgery decision face); the execution-constraint face
// is the guarded birth and the append-only lattice.

// SignoffRecordRequest records one approval or rejection for a specific
// subject. The subject class must be on the whitelist; for SoD-mandatory
// classes the guard verifies the approver against the original executor.
type SignoffRecordRequest struct {
	Token        string `json:"token"`
	SubjectClass string `json:"subject_class"`
	SubjectRef   string `json:"subject_ref"`
	Decision     string `json:"decision"`
	Reason       string `json:"reason"`
}

// SignoffRecord records one signoff under the guard-bound context. The
// approver is derived from the session; the subject class is whitelist
// checked; break-glass ratification additionally validates that the
// subject is a real break-glass audit row and binds its executor as the
// requester for the SoD ruling. The write face of this verb is the
// signoffs registry and the activity event — nothing else (D18: an
// approval never creates demand).
func SignoffRecord(db *sql.DB, id *RunIdentity, a SignoffRecordRequest) (any, error) {
	if err := RequireHostScope(id, "signoff recording"); err != nil {
		return nil, err
	}
	if a.SubjectClass == "" || a.SubjectRef == "" || len(a.SubjectRef) > 256 {
		return nil, Usagef("The signoff subject is invalid.")
	}
	if a.Decision != "approved" && a.Decision != "rejected" {
		return nil, Usagef("The signoff decision is invalid.")
	}
	if a.Reason == "" || len(a.Reason) > 1024 {
		return nil, Usagef("The signoff reason is invalid.")
	}
	guard := identity.NewGuard()
	out := map[string]any{"recorded": false}
	if err := inTx(db, func(ctx context.Context, q Q) error {
		approver, err := sessionActor(ctx, q, a.Token)
		if err != nil {
			return err
		}
		// Subject validation per class (fail closed for known classes): a
		// break-glass ratification must point at a real break-glass audit
		// row, and its executor is bound as the requester for the SoD
		// ruling (never taken from the request).
		requestedBy := ""
		if a.SubjectClass == "breakglass.ratification" {
			var executor string
			err := q.QueryRowContext(ctx, `SELECT actor FROM activity
				WHERE id = ? AND kind = 'session.breakglass_revoked'`, a.SubjectRef).Scan(&executor)
			if err != nil {
				if err == sql.ErrNoRows {
					return Domainf("The break-glass audit row named by the subject does not exist.")
				}
				return err
			}
			requestedBy = executor
		}
		decision := guard.DecideSignoff(a.SubjectClass, approver, requestedBy)
		if decision.RequireSignoff || !decision.Allow {
			return Domainf("%s", decision.Reason)
		}
		if err := identity.BindTrustedContext(ctx, q, approver, false); err != nil {
			return err
		}
		signoffID, err := identity.NewOpaqueID()
		if err != nil {
			return Usagef("The signoff could not be allocated.")
		}
		var requestedByVal any
		if requestedBy != "" {
			requestedByVal = requestedBy
		}
		if _, err := q.ExecContext(ctx, `INSERT INTO signoffs
			(signoff_id, subject_class, subject_ref, requested_by, approved_by, decision, reason)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			signoffID, a.SubjectClass, a.SubjectRef, requestedByVal, approver, a.Decision, a.Reason); err != nil {
			return Domainf("The signoff could not be recorded.")
		}
		out["signoff_id"] = signoffID
		out["recorded"] = true
		return recordActivity(ctx, q, approver, "signoff.recorded", signoffID,
			a.Decision+" signoff for "+a.SubjectClass+" subject "+a.SubjectRef+": "+a.Reason)
	}); err != nil {
		return nil, classify(err)
	}
	return out, nil
}
