package signnode

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// testSecretStore is the file-backed host secret stand-in for tests.
type testSecretStore struct {
	root string
	mu   sync.Mutex
}

func newTestSecretStore(t *testing.T) *testSecretStore {
	t.Helper()
	return &testSecretStore{root: t.TempDir()}
}

func (s *testSecretStore) Write(home, kind string, value []byte) (string, error) {
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

func (s *testSecretStore) Read(home, kind, reference string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.ReadFile(filepath.Join(s.root, reference))
}

// testHome isolates one node home per test and swaps the host secret seam
// for the file-backed store (the node never touches the real keychain).
func testHome(t *testing.T) string {
	t.Helper()
	store := newTestSecretStore(t)
	previous := HostSecretStore
	HostSecretStore = store
	t.Cleanup(func() { HostSecretStore = previous })
	return t.TempDir()
}

// serveOnce runs one node operation in-process (the same serve the
// __sign-node child process and the verb seam use) and decodes the answer.
func serveOnce(t *testing.T, home string, req Request) Response {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Serve(home, bytes.NewReader(body), &out); err != nil {
		t.Fatalf("serve %s: %v", req.Op, err)
	}
	var resp Response
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func serveNoFatal(home string, req Request) Response {
	body, _ := json.Marshal(req)
	var out bytes.Buffer
	_ = Serve(home, bytes.NewReader(body), &out)
	var resp Response
	_ = json.Unmarshal(out.Bytes(), &resp)
	return resp
}

func bootstrap(t *testing.T, home string) Response {
	t.Helper()
	resp := serveOnce(t, home, Request{Op: "bootstrap"})
	if resp.Status != StatusOK || resp.KeyID == "" || resp.PublicKeyHex == "" {
		t.Fatalf("bootstrap = %+v; want ok with key material", resp)
	}
	return resp
}

func repeatHex(byteValue byte) string {
	return string(bytes.Repeat([]byte{byteValue}, 64))
}

func hexDigest(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func sampleRequest(idSuffix byte) Request {
	payload := []byte("the anchor object bytes")
	return Request{
		Op:            "request",
		RequestID:     "a000000000000000000000000000000" + string(idSuffix),
		RequestDigest: repeatHex(idSuffix + 1),
		RootDigest:    hexDigest(payload),
		AlgorithmID:   "shz-merkle-v1",
		SerializerID:  "shz-canon-v1",
		PayloadHex:    hex.EncodeToString(payload),
	}
}

// F1a: a fenced request is refused with the verifiable confirmation, and a
// signed request returns its stored receipt idempotently (no re-sign).
func TestFencedRequestIsRefusedAndSignedIsIdempotent(t *testing.T) {
	home := testHome(t)
	bootstrap(t, home)
	req := sampleRequest('1')
	fence := serveOnce(t, home, Request{Op: "cancel", RequestID: req.RequestID, RequestDigest: req.RequestDigest, Reason: "superseded upstream"})
	if fence.Status != StatusFenced {
		t.Fatalf("cancel = %+v; want fenced", fence)
	}
	late := serveOnce(t, home, req)
	if late.Status != StatusFenced {
		t.Fatalf("late request after the confirmed cancel = %+v; want the fenced refusal", late)
	}
	// A signed request returns the same receipt on repeat.
	other := sampleRequest('2')
	first := serveOnce(t, home, other)
	if first.Status != StatusSigned || first.Receipt == nil {
		t.Fatalf("request = %+v; want signed with a receipt", first)
	}
	again := serveOnce(t, home, other)
	if again.Status != StatusSigned || again.Receipt.SignatureHex != first.Receipt.SignatureHex {
		t.Fatalf("re-request = %+v; want the identical stored receipt (no re-sign)", again)
	}
}

// F1b: the cancel and the request racing on the same request id have a
// definite order — the one-way ledger admits exactly one terminal state, so
// the two racers always report the same outcome; a confirmed cancel can
// never coexist with a completed signature.
func TestCancelAndRequestRaceHasDefiniteOrder(t *testing.T) {
	for trial := 0; trial < 25; trial++ {
		home := testHome(t)
		bootstrap(t, home)
		req := sampleRequest('8')
		var wg sync.WaitGroup
		outcomes := make(chan string, 2)
		wg.Add(2)
		go func() {
			defer wg.Done()
			outcomes <- serveNoFatal(home, Request{Op: "cancel", RequestID: req.RequestID, RequestDigest: req.RequestDigest, Reason: "race"}).Status
		}()
		go func() {
			defer wg.Done()
			outcomes <- serveNoFatal(home, req).Status
		}()
		wg.Wait()
		close(outcomes)
		first, second := <-outcomes, <-outcomes
		if first != second {
			t.Fatalf("race trial %d: the racers reported different outcomes (%s vs %s); the ledger is one-way", trial, first, second)
		}
		if first != StatusFenced && first != StatusSigned {
			t.Fatalf("race trial %d: outcome %s is not a terminal state", trial, first)
		}
		final := serveOnce(t, home, Request{Op: "query", RequestID: req.RequestID, RequestDigest: req.RequestDigest})
		if final.Status != first {
			t.Fatalf("race trial %d: the query sees %s; the racers saw %s", trial, final.Status, first)
		}
	}
}

// F2: fencing persists across node restarts — a fresh serve over the same
// home still refuses the fenced request.
func TestFencingSurvivesNodeRestart(t *testing.T) {
	home := testHome(t)
	bootstrap(t, home)
	req := sampleRequest('3')
	fence := serveOnce(t, home, Request{Op: "cancel", RequestID: req.RequestID, RequestDigest: req.RequestDigest, Reason: "pre-accept fence"})
	if fence.Status != StatusFenced {
		t.Fatalf("cancel = %+v; want fenced", fence)
	}
	late := serveOnce(t, home, req)
	if late.Status != StatusFenced {
		t.Fatalf("post-restart request = %+v; want the fence to hold", late)
	}
	lateQuery := serveOnce(t, home, Request{Op: "query", RequestID: req.RequestID, RequestDigest: req.RequestDigest})
	if lateQuery.Status != StatusFenced {
		t.Fatalf("post-restart query = %+v; want fenced", lateQuery)
	}
}

// F3: the cancel confirmation and the query are bound to the request digest
// — a tampered digest is refused and recorded, never confirmed.
func TestFencingConfirmationBindsTheRequestDigest(t *testing.T) {
	home := testHome(t)
	bootstrap(t, home)
	req := sampleRequest('4')
	serveOnce(t, home, Request{Op: "cancel", RequestID: req.RequestID, RequestDigest: req.RequestDigest, Reason: "bound"})
	tampered := serveOnce(t, home, Request{Op: "query", RequestID: req.RequestID, RequestDigest: repeatHex('f')})
	if tampered.Status != StatusTampered {
		t.Fatalf("tampered query = %+v; want the tamper refusal", tampered)
	}
	correct := serveOnce(t, home, Request{Op: "query", RequestID: req.RequestID, RequestDigest: req.RequestDigest})
	if correct.Status != StatusFenced {
		t.Fatalf("bound query = %+v; want the fenced confirmation", correct)
	}
}

// F4: a request the node never saw is NOT_ACCEPTED — a present-tense
// admission fact, categorically not a fence.
func TestNeverSeenRequestIsNotAcceptedNotFenced(t *testing.T) {
	home := testHome(t)
	bootstrap(t, home)
	resp := serveOnce(t, home, Request{Op: "query", RequestID: "a0000000000000000000000000000009", RequestDigest: repeatHex('e')})
	if resp.Status != StatusNotAccepted {
		t.Fatalf("query of an unseen request = %+v; want not_accepted (never fenced)", resp)
	}
}

// The payload must bind to the root digest: a mismatch is refused, recorded
// as tampered, and never signed.
func TestPayloadMustBindToRootDigest(t *testing.T) {
	home := testHome(t)
	bootstrap(t, home)
	req := sampleRequest('5')
	req.RootDigest = repeatHex('0') // wrong binding
	resp := serveOnce(t, home, req)
	if resp.Status != StatusPayloadMismatch {
		t.Fatalf("mismatched payload = %+v; want the binding refusal", resp)
	}
	// The request stays signable with the correct binding.
	req = sampleRequest('5')
	good := serveOnce(t, home, req)
	if good.Status != StatusSigned {
		t.Fatalf("corrected request = %+v; want signed", good)
	}
}

// The attestation verifies under the ring public key and binds the tuple.
func TestAttestationVerifiesUnderRingKey(t *testing.T) {
	home := testHome(t)
	boot := bootstrap(t, home)
	req := sampleRequest('6')
	resp := serveOnce(t, home, req)
	if resp.Status != StatusSigned || resp.Receipt == nil {
		t.Fatalf("request = %+v; want a signed receipt", resp)
	}
	receipt := *resp.Receipt
	if receipt.KeyID != boot.KeyID {
		t.Fatalf("receipt key %s; want the ring key %s", receipt.KeyID, boot.KeyID)
	}
	attestation, err := hex.DecodeString(receipt.AttestationHex)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := NodeDir(home)
	if err != nil {
		t.Fatal(err)
	}
	public, err := PublicKey(dir, receipt.KeyID)
	if err != nil {
		t.Fatal(err)
	}
	canon := AttestationCanonical(receipt.RequestID, receipt.RequestDigest, receipt.ResultDigest, receipt.KeyID)
	if !ed25519.Verify(public, canon, attestation) {
		t.Fatal("the attestation failed verification under the ring key")
	}
	if ed25519.Verify(public, AttestationCanonical(receipt.RequestID, receipt.RequestDigest, repeatHex('0'), receipt.KeyID), attestation) {
		t.Fatal("the attestation verified under a mutated result digest")
	}
}

// Bootstrap is idempotent: repeat calls return the same head key, and the
// ring keeps every historical key (old receipts stay verifiable).
func TestBootstrapIsIdempotentAndRingKeepsHistory(t *testing.T) {
	home := testHome(t)
	first := bootstrap(t, home)
	second := bootstrap(t, home)
	if first.KeyID != second.KeyID || first.PublicKeyHex != second.PublicKeyHex {
		t.Fatalf("bootstrap drifted: %s -> %s", first.KeyID, second.KeyID)
	}
}
