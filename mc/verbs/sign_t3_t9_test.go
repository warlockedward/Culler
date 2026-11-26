package verbs

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"mc/identity"
	"mc/signnode"
)

// signFileSecret is an isolated, file-backed implementation of the node's
// host secret seam so the sign verbs' tests never touch the real keychain.
type signFileSecret struct {
	root string
	mu   sync.Mutex
}

func (s *signFileSecret) Write(home, kind string, value []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	name := kind + "-" + hex.EncodeToString(buf) + ".secret"
	if err := os.WriteFile(filepath.Join(s.root, name), value, 0o600); err != nil {
		return "", err
	}
	return name, nil
}

func (s *signFileSecret) Read(home, kind, reference string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.ReadFile(filepath.Join(s.root, reference))
}

// signTestEnv isolates one node home per test, bootstraps the node through
// the real serve (the same code the __sign-node child process runs), and
// grants alice the anchor signing capability. It returns the environment
// the sign verbs need: an identity spine, alice's session token, and the
// node home.
type signTestEnv struct {
	spine *sql.DB
	home  string
	token string
}

func newSignTestEnv(t *testing.T) signTestEnv {
	t.Helper()
	db := igT1Spine(t)
	home := t.TempDir()
	t.Setenv("MC_HOME", home)
	store := &signFileSecret{root: t.TempDir()}
	previous := signnode.HostSecretStore
	signnode.HostSecretStore = store
	t.Cleanup(func() { signnode.HostSecretStore = previous })
	env := signTestEnv{spine: db, home: home}
	// Bootstrap the node through the production serve path.
	body, _ := json.Marshal(signnode.Request{Op: "bootstrap"})
	var out bytes.Buffer
	if err := signnode.Serve(home, bytes.NewReader(body), &out); err != nil {
		t.Fatalf("node bootstrap: %v", err)
	}
	var boot signnode.Response
	if err := json.Unmarshal(out.Bytes(), &boot); err != nil || boot.Status != signnode.StatusOK {
		t.Fatalf("node bootstrap response = %s; want ok", out.String())
	}
	// Alice's session and her anchor signing capability (self-grant).
	env.token, _ = igT2Session(t, db, "alice")
	if _, err := IdentityGrantIssue(db, nil, IdentityGrantIssueRequest{
		Token: env.token, Grantee: "alice", Scope: "anchor.sign.request", TTLHours: 24,
	}); err != nil {
		t.Fatalf("capability self-grant: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return env
}

func anchorPayload() (rootDigest, payloadHex string) {
	payload := []byte("the anchor object for the test")
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), hex.EncodeToString(payload)
}

func signedEntryCount(t *testing.T, home, requestID string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, "sign-node", "ledger.jsonl"))
	if err != nil {
		t.Fatalf("read the node ledger: %v", err)
	}
	n := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, `"op":"signed"`) && strings.Contains(line, `"request_id":"`+requestID+`"`) {
			n++
		}
	}
	return n
}

// S1 + C2 + C3: the full happy path — create, execute (the node signs
// exactly once), reconcile (the receipt is fetched and verified, no
// re-sign), and re-reconcile (idempotent no-op). This is the SIGNED-un-
// RECORDED crash point exercised end to end: the node's ledger holds the
// result, and reconciliation retrieves it.
func TestSignLifecycleRecordsExactlyOnce(t *testing.T) {
	env := newSignTestEnv(t)
	rootDigest, payloadHex := anchorPayload()
	created, err := SignRequestCreate(env.db(), nil, SignRequestCreateRequest{
		Token: env.token, RootDigest: rootDigest, AlgorithmID: "shz-merkle-v1",
		SerializerID: "shz-canon-v1", Scope: "anchor", TTLHours: 24,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requestID := created.(map[string]any)["request_id"].(string)
	if _, err := SignRequestExecute(env.db(), nil, SignRequestExecuteRequest{Token: env.token, RequestID: requestID, PayloadHex: payloadHex}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if n := signedEntryCount(t, env.home, requestID); n != 1 {
		t.Fatalf("node ledger signed entries after execute = %d; want exactly one", n)
	}
	if _, err := SignRequestReconcile(env.db(), nil, SignRequestReconcileRequest{Token: env.token, RequestID: requestID}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	state := signState(t, env, requestID)
	if state != "recorded" {
		t.Fatalf("state after reconcile = %s; want recorded", state)
	}
	// The reconciliation did not re-sign: the ledger still holds exactly one
	// signed entry (IG-T3 key case: SIGNED 未 RECORDED 对账不重签).
	if n := signedEntryCount(t, env.home, requestID); n != 1 {
		t.Fatalf("node ledger signed entries after reconcile = %d; the node must not re-sign", n)
	}
	// Re-reconcile is an idempotent no-op.
	res, err := SignRequestReconcile(env.db(), nil, SignRequestReconcileRequest{Token: env.token, RequestID: requestID})
	if err != nil {
		t.Fatalf("re-reconcile: %v", err)
	}
	if res.(map[string]any)["verdict"] != "already_recorded" {
		t.Fatalf("re-reconcile verdict = %v; want already_recorded", res)
	}
}

// C1 + F4: the node never saw the request — reconciliation closes it
// NOT_ACCEPTED, and the supersedes chain continues with exactly one
// successor.
func TestSignReconcileNotAcceptedClosesForSupersession(t *testing.T) {
	env := newSignTestEnv(t)
	rootDigest, _ := anchorPayload()
	created, err := SignRequestCreate(env.db(), nil, SignRequestCreateRequest{
		Token: env.token, RootDigest: rootDigest, AlgorithmID: "shz-merkle-v1",
		SerializerID: "shz-canon-v1", Scope: "anchor", TTLHours: 24,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requestID := created.(map[string]any)["request_id"].(string)
	res, err := SignRequestReconcile(env.db(), nil, SignRequestReconcileRequest{Token: env.token, RequestID: requestID})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if res.(map[string]any)["state"] != "closed" || res.(map[string]any)["close_reason"] != "not_accepted" {
		t.Fatalf("reconcile result = %v; want closed/not_accepted", res)
	}
	sup, err := SignRequestSupersede(env.db(), nil, SignRequestSupersedeRequest{Token: env.token, RequestID: requestID, Reason: "the node never accepted it", TTLHours: 24})
	if err != nil {
		t.Fatalf("supersede: %v", err)
	}
	newID := sup.(map[string]any)["request_id"].(string)
	if newID == requestID {
		t.Fatal("the successor reused the closed request id")
	}
	var predecessor string
	if err := env.db().QueryRow(`SELECT supersedes_request_id FROM sign_requests WHERE request_id = ?`, newID).Scan(&predecessor); err != nil {
		t.Fatal(err)
	}
	if predecessor != requestID {
		t.Fatalf("successor supersedes %s; want the closed request %s", predecessor, requestID)
	}
	// The chain does not branch: a second successor is refused.
	if _, err := SignRequestSupersede(env.db(), nil, SignRequestSupersedeRequest{Token: env.token, RequestID: requestID, Reason: "branching attempt", TTLHours: 24}); err == nil {
		t.Fatal("a second successor of the same closed request succeeded; the chain must not branch")
	}
}

// F5: an unreachable node is UNKNOWN — reconciliation stays pending, the
// state does not move, and no substitute is issued.
func TestSignReconcileUnknownStaysPendingWithoutSubstitute(t *testing.T) {
	env := newSignTestEnv(t)
	rootDigest, _ := anchorPayload()
	created, err := SignRequestCreate(env.db(), nil, SignRequestCreateRequest{
		Token: env.token, RootDigest: rootDigest, AlgorithmID: "shz-merkle-v1",
		SerializerID: "shz-canon-v1", Scope: "anchor", TTLHours: 24,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requestID := created.(map[string]any)["request_id"].(string)
	previous := signNodeServe
	signNodeServe = func(home string, in io.Reader, out io.Writer) error {
		return os.ErrClosed
	}
	t.Cleanup(func() { signNodeServe = previous })
	res, err := SignRequestReconcile(env.db(), nil, SignRequestReconcileRequest{Token: env.token, RequestID: requestID})
	if err == nil {
		t.Fatalf("reconcile over an unreachable node = %v; want the pending classification", res)
	}
	if state := signState(t, env, requestID); state != "requested" {
		t.Fatalf("state after the UNKNOWN reconciliation = %s; want requested (unchanged)", state)
	}
	var total, closed int
	if err := env.db().QueryRow(`SELECT COUNT(*) FROM sign_requests`).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if err := env.db().QueryRow(`SELECT COUNT(*) FROM sign_requests WHERE state = 'closed'`).Scan(&closed); err != nil {
		t.Fatal(err)
	}
	if total != 1 || closed != 0 {
		t.Fatalf("requests after UNKNOWN = %d total / %d closed; want 1/0 (no substitute, no premature close)", total, closed)
	}
}

// The cancel path: a fenced request closes, its execution is refused, and
// the supersedes chain may continue (the fence is the verifiable outcome).
func TestSignCancelFencesAndCloses(t *testing.T) {
	env := newSignTestEnv(t)
	rootDigest, payloadHex := anchorPayload()
	created, err := SignRequestCreate(env.db(), nil, SignRequestCreateRequest{
		Token: env.token, RootDigest: rootDigest, AlgorithmID: "shz-merkle-v1",
		SerializerID: "shz-canon-v1", Scope: "anchor", TTLHours: 24,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requestID := created.(map[string]any)["request_id"].(string)
	res, err := SignRequestCancel(env.db(), nil, SignRequestCancelRequest{Token: env.token, RequestID: requestID, Reason: "the anchor was superseded upstream"})
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if res.(map[string]any)["state"] != "closed" || res.(map[string]any)["close_reason"] != "fenced" {
		t.Fatalf("cancel result = %v; want closed/fenced", res)
	}
	// Executing a fenced request is refused.
	if _, err := SignRequestExecute(env.db(), nil, SignRequestExecuteRequest{Token: env.token, RequestID: requestID, PayloadHex: payloadHex}); err == nil {
		t.Fatal("executing a fenced request succeeded; want the fence refusal")
	}
	// The chain continues from the fenced close.
	if _, err := SignRequestSupersede(env.db(), nil, SignRequestSupersedeRequest{Token: env.token, RequestID: requestID, Reason: "fresh anchor attempt", TTLHours: 24}); err != nil {
		t.Fatalf("supersede after the fence: %v", err)
	}
}

// The capability gate: without a live anchor.sign.request grant the request
// is not born (IG §5.1: the use subject of the anchor key is the request
// capability).
func TestSignCreateRequiresTheCapabilityGrant(t *testing.T) {
	env := newSignTestEnv(t)
	// Bob has no anchor.sign.request grant.
	bobToken, _ := igT2Session(t, env.db(), "bob")
	rootDigest, _ := anchorPayload()
	if _, err := SignRequestCreate(env.db(), nil, SignRequestCreateRequest{
		Token: bobToken, RootDigest: rootDigest, AlgorithmID: "shz-merkle-v1",
		SerializerID: "shz-canon-v1", Scope: "anchor", TTLHours: 24,
	}); err == nil {
		t.Fatal("a request was created without the anchor signing capability")
	}
	var n int
	if err := env.db().QueryRow(`SELECT COUNT(*) FROM sign_requests`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("sign_requests rows after the refused create = %d; want 0", n)
	}
}

// A forged receipt never records: the attestation must verify under the
// ring key before the state advances.
func TestSignExecuteRefusesAForgedReceipt(t *testing.T) {
	env := newSignTestEnv(t)
	rootDigest, payloadHex := anchorPayload()
	created, err := SignRequestCreate(env.db(), nil, SignRequestCreateRequest{
		Token: env.token, RootDigest: rootDigest, AlgorithmID: "shz-merkle-v1",
		SerializerID: "shz-canon-v1", Scope: "anchor", TTLHours: 24,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requestID := created.(map[string]any)["request_id"].(string)
	requestDigest := created.(map[string]any)["request_digest"].(string)
	previous := signNodeServe
	forged := signnode.Response{Status: signnode.StatusSigned, Receipt: &signnode.Receipt{
		RequestID: requestID, RequestDigest: requestDigest,
		ResultDigest:   strings.Repeat("0", 64),
		SignatureHex:   strings.Repeat("1", 128),
		AttestationHex: strings.Repeat("2", 128),
		KeyID:          strings.Repeat("3", 64),
	}}
	signNodeServe = func(home string, in io.Reader, out io.Writer) error {
		return json.NewEncoder(out).Encode(forged)
	}
	t.Cleanup(func() { signNodeServe = previous })
	if _, err := SignRequestExecute(env.db(), nil, SignRequestExecuteRequest{Token: env.token, RequestID: requestID, PayloadHex: payloadHex}); err == nil {
		t.Fatal("a forged receipt was accepted; the attestation must verify first")
	}
	if state := signState(t, env, requestID); state != "requested" {
		t.Fatalf("state after the forged receipt = %s; want requested (no state advance)", state)
	}
}

// The state machine moves one way only, and the request row is born only
// under the guard-bound context (S10 trigger face).
func TestSignRequestStateMachineAndGuardedBirth(t *testing.T) {
	env := newSignTestEnv(t)
	db := env.db()
	rootDigest, _ := anchorPayload()
	// Unguarded birth is refused.
	if _, err := db.Exec(`INSERT INTO sign_requests
		(request_id, root_digest, algorithm_id, serializer_id, source_actor, scope, expires_at)
		VALUES ('a1111111111111111111111111111111', ?, 'alg', 'ser', 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'anchor', datetime('now', '+24 hours'))`,
		rootDigest); err == nil {
		t.Fatal("an unguarded sign request insert passed; want the trigger refusal")
	}
	// A mismatched attribution is refused even under a bound context.
	err := inTx(db, func(ctx context.Context, q Q) error {
		if err := identity.BindTrustedContext(ctx, q, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", false); err != nil {
			return err
		}
		_, err := q.ExecContext(ctx, `INSERT INTO sign_requests
			(request_id, root_digest, algorithm_id, serializer_id, source_actor, scope, expires_at)
			VALUES ('a2222222222222222222222222222222', ?, 'alg', 'ser', 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'anchor', datetime('now', '+24 hours'))`,
			rootDigest)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "disagrees") {
		t.Fatalf("mismatched attribution insert = %v; want the attribution refusal", err)
	}
	// The one-way machine: skip requested->recorded is refused, and a
	// terminal state never revives.
	created, err := SignRequestCreate(db, nil, SignRequestCreateRequest{
		Token: env.token, RootDigest: rootDigest, AlgorithmID: "shz-merkle-v1",
		SerializerID: "shz-canon-v1", Scope: "anchor", TTLHours: 24,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	requestID := created.(map[string]any)["request_id"].(string)
	if _, err := db.Exec(`UPDATE sign_requests SET state = 'recorded' WHERE request_id = ?`, requestID); err == nil {
		t.Fatal("a requested request jumped straight to recorded; the state machine must refuse")
	}
	_, payload := anchorPayload()
	if _, err := SignRequestExecute(db, nil, SignRequestExecuteRequest{Token: env.token, RequestID: requestID, PayloadHex: payload}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, err := SignRequestReconcile(db, nil, SignRequestReconcileRequest{Token: env.token, RequestID: requestID}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if _, err := db.Exec(`UPDATE sign_requests SET state = 'signed' WHERE request_id = ?`, requestID); err == nil {
		t.Fatal("a recorded request re-entered signed; terminal states must not revive")
	}
}

// Test helpers.

func (e signTestEnv) db() *sql.DB { return e.spine }

func signState(t *testing.T, env signTestEnv, requestID string) string {
	t.Helper()
	var state string
	if err := env.db().QueryRow(`SELECT state FROM sign_requests WHERE request_id = ?`, requestID).Scan(&state); err != nil {
		t.Fatalf("read the sign request state: %v", err)
	}
	return state
}
