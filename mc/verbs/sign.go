package verbs

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"mc/deployment"
	"mc/identity"
	"mc/signnode"
)

// The anchor signing state machine (D06, IG §5.2/§5.3): the spine side of
// REQUESTED -> SIGNED -> RECORDED with the one-way CLOSED exit. The node is
// the external side effect; the spine never signs and never re-signs —
// reconciliation fetches the node's result by request id.

// errSignNodeUnknown is the conservative classification of a node that
// cannot be reached or speaks infrastructure errors: the reconciliation
// stays pending and no substitute request is issued (IG §5.3 UNKNOWN branch).
var errSignNodeUnknown = errors.New("the sign node is unreachable; the request stays pending")

// signNodeServe is the seam to the signing node: production serves the
// operation in-process over the on-disk ledger (stateless per operation,
// flock-serialized — the O11 ruling P2/P5 semantics); the independent
// __sign-node child process carries the same serve for operator and
// automation use, and the production carrier (IG §5.4 B) restores true
// process separation. Tests swap this seam to isolate homes or inject
// unreachable-node failures.
var signNodeServe = signnode.Serve

// SignRequestCreateRequest births a signing request: the request row and
// its Class A event land in one transaction (REQUESTED is atomic, IG §5.3).
type SignRequestCreateRequest struct {
	Token        string `json:"token"`
	RootDigest   string `json:"root_digest"`
	AlgorithmID  string `json:"algorithm_id"`
	SerializerID string `json:"serializer_id"`
	Scope        string `json:"scope"`
	TTLHours     int    `json:"ttl_hours,omitempty"`
}

// SignRequestCreate registers a signing request under the guard-bound
// context. The acting user must hold a live anchor.sign.request grant (IG
// §5.1: the use subject of the anchor key is the request capability, not a
// general signing right).
func SignRequestCreate(db *sql.DB, id *RunIdentity, a SignRequestCreateRequest) (any, error) {
	if err := RequireHostScope(id, "signing request creation"); err != nil {
		return nil, err
	}
	if len(a.RootDigest) != 64 {
		return nil, Usagef("The sign request root digest is invalid.")
	}
	if a.AlgorithmID == "" || len(a.AlgorithmID) > 128 || a.SerializerID == "" || len(a.SerializerID) > 128 {
		return nil, Usagef("The sign request algorithm and serializer are invalid.")
	}
	if a.Scope == "" || len(a.Scope) > 128 {
		return nil, Usagef("The sign request scope is invalid.")
	}
	if a.TTLHours <= 0 || a.TTLHours > 24*90 {
		return nil, Usagef("The sign request validity is invalid.")
	}
	requestID, err := identity.NewOpaqueID()
	if err != nil {
		return nil, Usagef("The sign request could not be allocated.")
	}
	out := map[string]any{"created": false}
	if err := inTx(db, func(ctx context.Context, q Q) error {
		actor, err := sessionActor(ctx, q, a.Token)
		if err != nil {
			return err
		}
		if d, err := identity.GrantAllows(ctx, q, actor, "anchor.sign.request", "", time.Now().UTC()); err != nil || !d.Allow {
			return Domainf("The anchor sign request capability is not granted to this user.")
		}
		if err := identity.BindTrustedContext(ctx, q, actor, false); err != nil {
			return err
		}
		requestDigest, err := signnode.RequestDigest(requestID, a.RootDigest, a.AlgorithmID, a.SerializerID, a.Scope, actor)
		if err != nil {
			return Usagef("The sign request digest could not be rendered.")
		}
		ttl := fmt.Sprintf("+%d hours", a.TTLHours)
		if _, err := q.ExecContext(ctx, `INSERT INTO sign_requests
			(request_id, root_digest, algorithm_id, serializer_id, source_actor, scope, expires_at)
			VALUES (?, ?, ?, ?, ?, ?, datetime('now', ?))`,
			requestID, a.RootDigest, a.AlgorithmID, a.SerializerID, actor, a.Scope, ttl); err != nil {
			return Domainf("The sign request could not be recorded.")
		}
		out["request_id"] = requestID
		out["request_digest"] = requestDigest
		out["created"] = true
		return recordActivity(ctx, q, actor, "sign.requested", requestID,
			fmt.Sprintf("A signing request was registered for root digest %s.", a.RootDigest))
	}); err != nil {
		return nil, classify(err)
	}
	return out, nil
}

// SignRequestExecuteRequest drives the node once for a requested sign
// request: the payload is presented to the node, which verifies it against
// the root digest and signs it. The spine lands the result idempotently by
// request id (SIGNED) after verifying the receipt.
type SignRequestExecuteRequest struct {
	Token      string `json:"token"`
	RequestID  string `json:"request_id"`
	PayloadHex string `json:"payload_hex"`
}

// SignRequestExecute performs the external signing step and, only after the
// receipt verifies against the node's key ring, moves the row to SIGNED with
// the result bound back by digest. Re-executing a signed request returns
// the stored result without contacting the node.
func SignRequestExecute(db *sql.DB, id *RunIdentity, a SignRequestExecuteRequest) (any, error) {
	if err := RequireHostScope(id, "signing request execution"); err != nil {
		return nil, err
	}
	if a.RequestID == "" {
		return nil, Usagef("The sign request id is required.")
	}
	home, err := signNodeHome()
	if err != nil {
		return nil, err
	}
	stored, err := signRowFor(db, a.Token, a.RequestID)
	if err != nil {
		return nil, classify(err)
	}
	if stored.State == "signed" || stored.State == "recorded" {
		if stored.SignatureHex == nil || stored.ResultDigest == nil {
			return nil, Domainf("The sign request result is incomplete.")
		}
		return map[string]any{"request_id": a.RequestID, "state": stored.State,
			"result_digest": *stored.ResultDigest, "signature_hex": *stored.SignatureHex}, nil
	}
	if stored.State != "requested" {
		return nil, Domainf("The sign request is closed and cannot be executed.")
	}
	requestDigest, err := stored.requestDigest(a.RequestID)
	if err != nil {
		return nil, Usagef("The sign request digest could not be rendered.")
	}
	resp, err := signNodeCall(home, signnode.Request{
		Op: "request", RequestID: a.RequestID, RequestDigest: requestDigest,
		RootDigest: stored.RootDigest, AlgorithmID: stored.AlgorithmID,
		SerializerID: stored.SerializerID, PayloadHex: a.PayloadHex,
	})
	if err != nil {
		return nil, classify(err)
	}
	if resp.Status == signnode.StatusFenced {
		return nil, Domainf("The sign request was fenced; it cannot be executed.")
	}
	if resp.Status != signnode.StatusSigned || resp.Receipt == nil {
		return nil, Domainf("The sign node refused the request: %s.", resp.Status)
	}
	receipt := *resp.Receipt
	if err := verifySignReceipt(home, a.RequestID, requestDigest, receipt); err != nil {
		return nil, err
	}
	updated := false
	if err := inTx(db, func(ctx context.Context, q Q) error {
		res, err := q.ExecContext(ctx, `UPDATE sign_requests SET state = 'signed', result_digest = ?,
			signature_hex = ?, receipt_attestation = ?, key_id = ?
			WHERE request_id = ? AND state = 'requested'`,
			receipt.ResultDigest, receipt.SignatureHex, receipt.AttestationHex, receipt.KeyID, a.RequestID)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err == nil && n == 1 {
			updated = true
		}
		return recordActivity(ctx, q, stored.SourceActor, "sign.result", a.RequestID,
			fmt.Sprintf("The sign node produced the result for the request (digest %s).", receipt.ResultDigest))
	}); err != nil {
		return nil, classify(err)
	}
	if !updated {
		return nil, Domainf("The sign request state moved concurrently; reconcile it.")
	}
	return map[string]any{"request_id": a.RequestID, "state": "signed",
		"result_digest": receipt.ResultDigest, "signature_hex": receipt.SignatureHex}, nil
}

// SignRequestReconcileRequest reconciles a sign request against the node
// (IG §5.3 four branches): SIGNED fetches the receipt without re-signing;
// NOT_ACCEPTED closes the request; FENCED closes it fenced; UNKNOWN leaves
// it pending and issues no substitute.
type SignRequestReconcileRequest struct {
	Token     string `json:"token"`
	RequestID string `json:"request_id"`
}

// SignRequestReconcile runs one reconciliation pass over the request.
func SignRequestReconcile(db *sql.DB, id *RunIdentity, a SignRequestReconcileRequest) (any, error) {
	if err := RequireHostScope(id, "signing request reconciliation"); err != nil {
		return nil, err
	}
	if a.RequestID == "" {
		return nil, Usagef("The sign request id is required.")
	}
	home, err := signNodeHome()
	if err != nil {
		return nil, err
	}
	row, err := signRowFor(db, a.Token, a.RequestID)
	if err != nil {
		return nil, classify(err)
	}
	requestDigest, err := row.requestDigest(a.RequestID)
	if err != nil {
		return nil, Usagef("The sign request digest could not be rendered.")
	}
	switch row.State {
	case "recorded":
		return map[string]any{"request_id": a.RequestID, "state": "recorded", "verdict": "already_recorded"}, nil
	case "closed":
		return map[string]any{"request_id": a.RequestID, "state": "closed", "verdict": "already_closed"}, nil
	case "signed":
		// The result landed but the receipt was not yet recorded (the
		// SIGNED-unrecorded crash point). Fetch by request id: the node
		// returns its stored receipt, it does not re-sign.
		resp, err := signNodeCall(home, signnode.Request{Op: "query", RequestID: a.RequestID, RequestDigest: requestDigest})
		if err != nil {
			return map[string]any{"request_id": a.RequestID, "state": "signed", "verdict": "pending"}, errSignNodeUnknown
		}
		if resp.Status != signnode.StatusSigned || resp.Receipt == nil {
			// The node ledger disagrees with the spine: fail closed, no
			// substitute, no fabrication of a recorded state.
			return nil, Domainf("The node ledger disagrees with the spine result; reconcile after node state recovery.")
		}
		if row.SignatureHex == nil || *row.SignatureHex != resp.Receipt.SignatureHex {
			return nil, Domainf("The node receipt does not match the spine result; reconcile after node state recovery.")
		}
		if err := verifySignReceipt(home, a.RequestID, requestDigest, *resp.Receipt); err != nil {
			return nil, err
		}
		return finishSignRecorded(db, a.RequestID, row.SourceActor, resp.Receipt)
	case "requested":
		resp, err := signNodeCall(home, signnode.Request{Op: "query", RequestID: a.RequestID, RequestDigest: requestDigest})
		if err != nil {
			// UNKNOWN: stay pending, never issue a substitute.
			return map[string]any{"request_id": a.RequestID, "state": "requested", "verdict": "pending"}, errSignNodeUnknown
		}
		switch resp.Status {
		case signnode.StatusSigned:
			if resp.Receipt == nil {
				return nil, Domainf("The node receipt is malformed.")
			}
			if err := verifySignReceipt(home, a.RequestID, requestDigest, *resp.Receipt); err != nil {
				return nil, err
			}
			receipt := *resp.Receipt
			landed := false
			if err := inTx(db, func(ctx context.Context, q Q) error {
				if _, err := q.ExecContext(ctx, `UPDATE sign_requests SET state = 'signed', result_digest = ?,
					signature_hex = ?, receipt_attestation = ?, key_id = ?
					WHERE request_id = ? AND state = 'requested'`,
					receipt.ResultDigest, receipt.SignatureHex, receipt.AttestationHex, receipt.KeyID, a.RequestID); err != nil {
					return err
				}
				if err := recordActivity(ctx, q, row.SourceActor, "sign.result", a.RequestID,
					fmt.Sprintf("Reconciliation fetched the node result for the request (digest %s).", receipt.ResultDigest)); err != nil {
					return err
				}
				if _, err := q.ExecContext(ctx, `UPDATE sign_requests SET state = 'recorded' WHERE request_id = ? AND state = 'signed'`, a.RequestID); err != nil {
					return err
				}
				landed = true
				return recordActivity(ctx, q, row.SourceActor, "sign.recorded", a.RequestID,
					"Reconciliation recorded the verified receipt; the state machine terminates.")
			}); err != nil {
				return nil, classify(err)
			}
			if !landed {
				return nil, Domainf("The sign request state moved concurrently; reconcile it again.")
			}
			return map[string]any{"request_id": a.RequestID, "state": "recorded", "verdict": "recorded",
				"result_digest": receipt.ResultDigest}, nil
		case signnode.StatusNotAccepted:
			return closeSignRequest(db, a.RequestID, row.SourceActor, "not_accepted",
				"Reconciliation found the request never accepted at the node; it is closed for supersession.")
		case signnode.StatusFenced:
			return closeSignRequest(db, a.RequestID, row.SourceActor, "fenced",
				"Reconciliation found the request fenced at the node; it is closed.")
		default:
			return nil, Domainf("The node ledger digest disagrees with the spine request; no reconciliation state changed.")
		}
	}
	return nil, Domainf("The sign request state is unrecognized.")
}

// SignRequestCancelRequest fences a requested sign request at the node
// (the verifiable cancellation, IG §5.3 guarantee five) and closes the row.
type SignRequestCancelRequest struct {
	Token     string `json:"token"`
	RequestID string `json:"request_id"`
	Reason    string `json:"reason"`
}

// SignRequestCancel performs the fencing cancel. A request that already
// signed is refused here: its receipt is the reconciliation's business.
func SignRequestCancel(db *sql.DB, id *RunIdentity, a SignRequestCancelRequest) (any, error) {
	if err := RequireHostScope(id, "signing request cancellation"); err != nil {
		return nil, err
	}
	if a.RequestID == "" {
		return nil, Usagef("The sign request id is required.")
	}
	if a.Reason == "" {
		return nil, Usagef("The cancellation requires a reason.")
	}
	home, err := signNodeHome()
	if err != nil {
		return nil, err
	}
	row, err := signRowFor(db, a.Token, a.RequestID)
	if err != nil {
		return nil, classify(err)
	}
	if row.State != "requested" {
		return nil, Domainf("Only a requested sign request can be fenced; reconcile the signed one.")
	}
	requestDigest, err := row.requestDigest(a.RequestID)
	if err != nil {
		return nil, Usagef("The sign request digest could not be rendered.")
	}
	resp, err := signNodeCall(home, signnode.Request{Op: "cancel", RequestID: a.RequestID, RequestDigest: requestDigest, Reason: a.Reason})
	if err != nil {
		return map[string]any{"request_id": a.RequestID, "state": "requested", "verdict": "pending"}, errSignNodeUnknown
	}
	if resp.Status == signnode.StatusSigned {
		return nil, Domainf("The request already signed at the node; reconcile the receipt instead of canceling.")
	}
	if resp.Status != signnode.StatusFenced {
		return nil, Domainf("The node cancel confirmation is malformed: %s.", resp.Status)
	}
	return closeSignRequest(db, a.RequestID, row.SourceActor, "fenced",
		fmt.Sprintf("The request was fenced at the node: %s", a.Reason))
}

// SignRequestSupersedeRequest continues a closed request with a new one
// (DC §3.3 supersedes chain). The substitute is permitted only after a
// verifiable node-side outcome: NOT_ACCEPTED or FENCED — never after
// UNKNOWN.
type SignRequestSupersedeRequest struct {
	Token     string `json:"token"`
	RequestID string `json:"request_id"`
	Reason    string `json:"reason"`
	TTLHours  int    `json:"ttl_hours,omitempty"`
}

// SignRequestSupersede creates the successor request bound to the closed
// predecessor.
func SignRequestSupersede(db *sql.DB, id *RunIdentity, a SignRequestSupersedeRequest) (any, error) {
	if err := RequireHostScope(id, "signing request supersession"); err != nil {
		return nil, err
	}
	if a.RequestID == "" {
		return nil, Usagef("The sign request id is required.")
	}
	if a.Reason == "" {
		return nil, Usagef("The supersession requires a reason.")
	}
	if a.TTLHours <= 0 || a.TTLHours > 24*90 {
		return nil, Usagef("The sign request validity is invalid.")
	}
	base, err := signRowFor(db, a.Token, a.RequestID)
	if err != nil {
		return nil, classify(err)
	}
	var row struct {
		signRow
		CloseReason *string
	}
	row.signRow = base
	ctx := context.Background()
	if err := db.QueryRowContext(ctx, `SELECT close_reason FROM sign_requests WHERE request_id = ?`, a.RequestID).Scan(&row.CloseReason); err != nil {
		return nil, classify(err)
	}
	var successors int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sign_requests WHERE supersedes_request_id = ?`, a.RequestID).Scan(&successors); err != nil {
		return nil, classify(err)
	}
	if successors != 0 {
		return nil, Domainf("The closed request already has a successor; the supersedes chain does not branch.")
	}
	if row.State != "closed" || row.CloseReason == nil ||
		(*row.CloseReason != "not_accepted" && *row.CloseReason != "fenced") {
		return nil, Domainf("Supersession requires a request closed by a verifiable node outcome (not accepted or fenced).")
	}
	requestID, err := identity.NewOpaqueID()
	if err != nil {
		return nil, Usagef("The successor sign request could not be allocated.")
	}
	requestDigest, err := signnode.RequestDigest(requestID, row.RootDigest, row.AlgorithmID, row.SerializerID, row.Scope, row.SourceActor)
	if err != nil {
		return nil, Usagef("The sign request digest could not be rendered.")
	}
	ttl := fmt.Sprintf("+%d hours", a.TTLHours)
	out := map[string]any{"created": false}
	if err := inTx(db, func(ctx context.Context, q Q) error {
		if err := identity.BindTrustedContext(ctx, q, row.SourceActor, false); err != nil {
			return err
		}
		if _, err := q.ExecContext(ctx, `INSERT INTO sign_requests
			(request_id, root_digest, algorithm_id, serializer_id, source_actor, scope, expires_at, supersedes_request_id)
			VALUES (?, ?, ?, ?, ?, ?, datetime('now', ?), ?)`,
			requestID, row.RootDigest, row.AlgorithmID, row.SerializerID, row.SourceActor, row.Scope, ttl, a.RequestID); err != nil {
			return Domainf("The successor sign request could not be recorded.")
		}
		out["request_id"] = requestID
		out["request_digest"] = requestDigest
		out["created"] = true
		return recordActivity(ctx, q, row.SourceActor, "sign.requested", requestID,
			fmt.Sprintf("A successor signing request supersedes %s: %s", a.RequestID, a.Reason))
	}); err != nil {
		return nil, classify(err)
	}
	return out, nil
}

// signRowFor reads the session actor and one sign request row. The read is
// plain, without the write lock: the state change that follows re-checks the
// row inside its own transaction (the CAS pattern the other verbs use).
func signRowFor(db *sql.DB, token, requestID string) (signRow, error) {
	ctx := context.Background()
	row := signRow{}
	var err error
	if row.SourceActor, err = sessionActor(ctx, db, token); err != nil {
		return row, err
	}
	return row, row.scan(db, ctx, requestID)
}

// signRow is one sign_requests row read off the spine, with the helpers the
// verbs share for rendering its canonical digest.
type signRow struct {
	State        string
	SourceActor  string
	RootDigest   string
	AlgorithmID  string
	SerializerID string
	Scope        string
	SignatureHex *string
	ResultDigest *string
	KeyID        *string
}

func (r *signRow) scan(q Q, ctx context.Context, requestID string) error {
	err := q.QueryRowContext(ctx, `SELECT state, source_actor, root_digest, algorithm_id, serializer_id, scope,
		signature_hex, result_digest, key_id FROM sign_requests WHERE request_id = ?`, requestID).
		Scan(&r.State, &r.SourceActor, &r.RootDigest, &r.AlgorithmID, &r.SerializerID, &r.Scope,
			&r.SignatureHex, &r.ResultDigest, &r.KeyID)
	if err != nil {
		if err == sql.ErrNoRows {
			return Domainf("The sign request is unknown.")
		}
		return err
	}
	return nil
}

func (r signRow) requestDigest(requestID string) (string, error) {
	return signnode.RequestDigest(requestID, r.RootDigest, r.AlgorithmID, r.SerializerID, r.Scope, r.SourceActor)
}

func closeSignRequest(db *sql.DB, requestID, actor, reason, detail string) (any, error) {
	closed := false
	if err := inTx(db, func(ctx context.Context, q Q) error {
		res, err := q.ExecContext(ctx, `UPDATE sign_requests SET state = 'closed', close_reason = ?
			WHERE request_id = ? AND state = 'requested'`, reason, requestID)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err == nil && n == 1 {
			closed = true
		}
		return recordActivity(ctx, q, actor, "sign.closed", requestID, detail)
	}); err != nil {
		return nil, classify(err)
	}
	if !closed {
		return nil, Domainf("The sign request state moved concurrently; reconcile it again.")
	}
	return map[string]any{"request_id": requestID, "state": "closed", "close_reason": reason}, nil
}

func finishSignRecorded(db *sql.DB, requestID, actor string, receipt *signnode.Receipt) (any, error) {
	recorded := false
	if err := inTx(db, func(ctx context.Context, q Q) error {
		res, err := q.ExecContext(ctx, `UPDATE sign_requests SET state = 'recorded'
			WHERE request_id = ? AND state = 'signed'`, requestID)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err == nil && n == 1 {
			recorded = true
		}
		return recordActivity(ctx, q, actor, "sign.recorded", requestID,
			fmt.Sprintf("Reconciliation recorded the verified receipt (result digest %s); the state machine terminates.", receipt.ResultDigest))
	}); err != nil {
		return nil, classify(err)
	}
	if !recorded {
		return nil, Domainf("The sign request state moved concurrently; reconcile it again.")
	}
	return map[string]any{"request_id": requestID, "state": "recorded", "verdict": "recorded",
		"result_digest": receipt.ResultDigest}, nil
}

// verifySignReceipt checks the node's receipt before the spine trusts it:
// the result digest matches the signature, the attestation verifies under
// the ring public key, and the binding tuple matches the request (P3).
func verifySignReceipt(home, requestID, requestDigest string, receipt signnode.Receipt) error {
	if receipt.RequestID != requestID || receipt.RequestDigest != requestDigest {
		return Domainf("The node receipt does not bind to the sign request.")
	}
	signature, err := hex.DecodeString(receipt.SignatureHex)
	if err != nil {
		return Domainf("The node receipt signature is malformed.")
	}
	sum := sha256.Sum256(signature)
	if hex.EncodeToString(sum[:]) != receipt.ResultDigest {
		return Domainf("The node receipt result digest does not match its signature.")
	}
	attestation, err := hex.DecodeString(receipt.AttestationHex)
	if err != nil {
		return Domainf("The node receipt attestation is malformed.")
	}
	dir, err := signnode.NodeDir(home)
	if err != nil {
		return err
	}
	public, err := signnode.PublicKey(dir, receipt.KeyID)
	if err != nil {
		return Domainf("The node key is unknown to the ring: %v", err)
	}
	if !ed25519.Verify(public, signnode.AttestationCanonical(requestID, requestDigest, receipt.ResultDigest, receipt.KeyID), attestation) {
		return Domainf("The node receipt attestation failed verification.")
	}
	return nil
}

// signNodeHome resolves the canonical deployment home the node and the key
// ring live under.
func signNodeHome() (string, error) {
	home := os.Getenv("MC_HOME")
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve default MC_HOME: %w", err)
		}
		home = filepath.Join(userHome, ".shorizon")
	}
	return deployment.CanonicalHome(home)
}

// signNodeCall runs one node operation through the serve seam and returns
// the structured answer. Any serve failure classifies as UNKNOWN: the node
// is unreachable or unhealthy, and the conservative branch applies.
func signNodeCall(home string, req signnode.Request) (signnode.Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return signnode.Response{}, errSignNodeUnknown
	}
	var stdout bytes.Buffer
	if err := signNodeServe(home, bytes.NewReader(body), &stdout); err != nil {
		return signnode.Response{}, errSignNodeUnknown
	}
	var resp signnode.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil || resp.Status == "" {
		return signnode.Response{}, errSignNodeUnknown
	}
	return resp, nil
}
