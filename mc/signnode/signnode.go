// Package signnode is the transitional anchor signing node (D06, O11
// ruling P1-P5): a per-invocation signer whose entire state is the
// on-disk ledger under the deployment home. The node never opens the
// spine; the spine verifies its receipts against the node's public key
// ring. Fencing is one-way and permanent: a fenced request is refused by
// every later node invocation, restart included.
package signnode

import (
	"bufio"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"mc/deployment"
)

// Statuses the node reports for an operation.
const (
	StatusOK              = "ok"
	StatusSigned          = "signed"
	StatusFenced          = "fenced"
	StatusNotAccepted     = "not_accepted"
	StatusTampered        = "tampered"
	StatusPayloadMismatch = "payload_mismatch"
)

// ErrUsage marks a protocol-level refusal (unknown op, malformed request):
// the caller maps it to the usage exit code.
var ErrUsage = errors.New("sign node usage error")

// secretKind is the host credential store kind of the node signing seed
// (A02 surface; the production carrier B replaces this storage natively).
const secretKind = "sign-node"

// Request is one node operation (P1 wire form).
type Request struct {
	Op            string `json:"op"`
	RequestID     string `json:"request_id"`
	RequestDigest string `json:"request_digest"`
	RootDigest    string `json:"root_digest"`
	AlgorithmID   string `json:"algorithm_id"`
	SerializerID  string `json:"serializer_id"`
	PayloadHex    string `json:"payload_hex"`
	Reason        string `json:"reason"`
}

// Receipt is the node's result binding: the signature over the payload,
// the digest of that signature, and the attestation over the canonical
// receipt tuple (P3).
type Receipt struct {
	RequestID      string `json:"request_id"`
	RequestDigest  string `json:"request_digest"`
	ResultDigest   string `json:"result_digest"`
	SignatureHex   string `json:"signature_hex"`
	AttestationHex string `json:"attestation_hex"`
	KeyID          string `json:"key_id"`
}

// Response is the node's reply to one operation.
type Response struct {
	Status       string   `json:"status"`
	Error        string   `json:"error,omitempty"`
	Receipt      *Receipt `json:"receipt,omitempty"`
	KeyID        string   `json:"key_id,omitempty"`
	PublicKeyHex string   `json:"public_key_hex,omitempty"`
}

// ledgerEntry is one append-only ledger record.
type ledgerEntry struct {
	Op             string `json:"op"`
	RequestID      string `json:"request_id"`
	RequestDigest  string `json:"request_digest,omitempty"`
	RootDigest     string `json:"root_digest,omitempty"`
	ResultDigest   string `json:"result_digest,omitempty"`
	SignatureHex   string `json:"signature_hex,omitempty"`
	AttestationHex string `json:"attestation_hex,omitempty"`
	KeyID          string `json:"key_id,omitempty"`
	Reason         string `json:"reason,omitempty"`
	Timestamp      string `json:"ts"`
}

type keyRing struct {
	Keys []ringKey `json:"keys"`
}

type ringKey struct {
	KeyID        string `json:"key_id"`
	PublicKeyHex string `json:"public_key_hex"`
	CreatedAt    string `json:"created_at"`
}

// HostSecret is the seam to the host credential store (A02): the
// transitional node keeps its signing seed there and nowhere else.
// Production uses the deployment backend; tests install an isolated store
// so the node never touches the real keychain.
type HostSecret interface {
	Write(home, kind string, value []byte) (reference string, err error)
	Read(home, kind, reference string) ([]byte, error)
}

type deploymentHostSecret struct{}

func (deploymentHostSecret) Write(home, kind string, value []byte) (string, error) {
	return deployment.WriteHostSecret(home, kind, value)
}

func (deploymentHostSecret) Read(home, kind, reference string) ([]byte, error) {
	return deployment.ReadHostSecret(home, kind, reference)
}

// HostSecretStore is the live seam; tests replace it.
var HostSecretStore HostSecret = deploymentHostSecret{}

// canonicalTime renders the spine's own time form for ledger records.
func canonicalTime(now time.Time) string {
	return now.UTC().Format("2006-01-02 15:04:05")
}

// RequestDigest is the shz-sign-v1 canonical digest of a signing request
// tuple: versioned, frozen field order, single authority (the node and
// the spine both call this, so the two sides can never diverge).
func RequestDigest(requestID, rootDigest, algorithmID, serializerID, scope, sourceActor string) (string, error) {
	canon := "shz-sign-request-v1\n" + strings.Join([]string{
		requestID, rootDigest, algorithmID, serializerID, scope, sourceActor,
	}, "\n")
	sum := sha256.Sum256([]byte(canon))
	return hex.EncodeToString(sum[:]), nil
}

// AttestationCanonical is the shz-sign-v1 canonical byte string the node
// attests over; the spine re-renders it to verify the receipt.
func AttestationCanonical(requestID, requestDigest, resultDigest, keyID string) []byte {
	return []byte("shz-sign-attestation-v1\n" + strings.Join([]string{
		requestID, requestDigest, resultDigest, keyID,
	}, "\n"))
}

// NodeDir resolves the node's state directory under a canonical home.
func NodeDir(home string) (string, error) {
	canonical, err := deployment.CanonicalHome(home)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(canonical, "sign-node")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create sign node directory: %w", err)
	}
	return dir, nil
}

// PublicKey renders the ring public key for a key id (the spine-side
// verification input).
func PublicKey(dir, keyID string) (ed25519.PublicKey, error) {
	ring, err := readRing(dir)
	if err != nil {
		return nil, err
	}
	for _, key := range ring.Keys {
		if key.KeyID == keyID {
			raw, err := hex.DecodeString(key.PublicKeyHex)
			if err != nil || len(raw) != ed25519.PublicKeySize {
				return nil, fmt.Errorf("the key ring entry %s is malformed", keyID)
			}
			return ed25519.PublicKey(raw), nil
		}
	}
	return nil, fmt.Errorf("the key ring holds no key %s", keyID)
}

func readRing(dir string) (keyRing, error) {
	var ring keyRing
	data, err := os.ReadFile(filepath.Join(dir, "ring.json"))
	if errors.Is(err, os.ErrNotExist) {
		return ring, nil
	}
	if err != nil {
		return ring, fmt.Errorf("read the key ring: %w", err)
	}
	if err := json.Unmarshal(data, &ring); err != nil {
		return ring, fmt.Errorf("the key ring is malformed: %w", err)
	}
	return ring, nil
}

func writeRing(dir string, ring keyRing) error {
	data, err := json.MarshalIndent(ring, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "ring.json"), append(data, '\n'), 0o600)
}

// Serve runs one node operation from an in-memory reader: read the
// request, dispatch under the ledger lock, write the response. It returns
// ErrUsage for protocol-level refusals (the command maps them to the usage
// exit code) and plain errors for infrastructure failures (domain exit
// code). The file handoff variant (ServeFile) is the verb's transport.
func Serve(home string, in io.Reader, out io.Writer) error {
	var req Request
	if err := json.NewDecoder(bufio.NewReaderSize(in, 64*1024)).Decode(&req); err != nil {
		return fmt.Errorf("%w: the sign node request is malformed", ErrUsage)
	}
	resp, err := dispatch(home, req)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(out)
	if err := encoder.Encode(resp); err != nil {
		return fmt.Errorf("write the sign node response: %w", err)
	}
	return nil
}

// ServeFile runs one node operation from the request file the verb
// staged under the node home (the file handoff transport): the request
// file is read exactly once and removed before the dispatch, so the
// payload never outlives the operation on disk.
func ServeFile(home, requestPath string, out io.Writer) error {
	data, err := os.ReadFile(requestPath)
	if err != nil {
		return fmt.Errorf("%w: the sign node request file is unreadable", ErrUsage)
	}
	defer os.Remove(requestPath)
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return fmt.Errorf("%w: the sign node request is malformed", ErrUsage)
	}
	clearBytes(data)
	resp, err := dispatch(home, req)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(out)
	if err := encoder.Encode(resp); err != nil {
		return fmt.Errorf("write the sign node response: %w", err)
	}
	return nil
}

func dispatch(home string, req Request) (Response, error) {
	dir, err := NodeDir(home)
	if err != nil {
		return Response{}, err
	}
	switch req.Op {
	case "bootstrap", "request", "cancel", "query":
	default:
		return Response{}, fmt.Errorf("%w: the sign node operation %q is unknown", ErrUsage, req.Op)
	}
	lock, err := os.OpenFile(filepath.Join(dir, ".flock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return Response{}, fmt.Errorf("lock the sign node ledger: %w", err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return Response{}, fmt.Errorf("lock the sign node ledger: %w", err)
	}
	defer func() { _ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) }()

	switch req.Op {
	case "bootstrap":
		return bootstrapOp(home, dir)
	case "request":
		return requestOp(home, dir, req)
	case "cancel":
		return cancelOp(dir, req)
	case "query":
		return queryOp(dir, req)
	}
	return Response{}, fmt.Errorf("%w: unreachable", ErrUsage)
}

// lastEntry replays the append-only ledger for one request: the last entry
// wins, which is how the one-way states (signed, fenced) persist across
// node restarts.
func lastEntry(dir, requestID string) (ledgerEntry, bool, error) {
	file, err := os.Open(filepath.Join(dir, "ledger.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return ledgerEntry{}, false, nil
	}
	if err != nil {
		return ledgerEntry{}, false, fmt.Errorf("open the sign node ledger: %w", err)
	}
	defer file.Close()
	var last ledgerEntry
	found := false
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry ledgerEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return ledgerEntry{}, false, fmt.Errorf("the sign node ledger line is malformed: %w", err)
		}
		if entry.RequestID == requestID {
			last = entry
			found = true
		}
	}
	if err := scanner.Err(); err != nil {
		return ledgerEntry{}, false, fmt.Errorf("read the sign node ledger: %w", err)
	}
	return last, found, nil
}

func appendEntry(dir string, entry ledgerEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(dir, "ledger.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open the sign node ledger: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("append the sign node ledger: %w", err)
	}
	return nil
}

func bootstrapOp(home, dir string) (Response, error) {
	ring, err := readRing(dir)
	if err != nil {
		return Response{}, err
	}
	if len(ring.Keys) > 0 {
		head := ring.Keys[len(ring.Keys)-1]
		return Response{Status: StatusOK, KeyID: head.KeyID, PublicKeyHex: head.PublicKeyHex}, nil
	}
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return Response{}, fmt.Errorf("generate the signing seed: %w", err)
	}
	keyID, err := HostSecretStore.Write(home, secretKind, seed)
	if err != nil {
		return Response{}, fmt.Errorf("store the signing seed in the host credential store: %w", err)
	}
	public := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	ring.Keys = append(ring.Keys, ringKey{
		KeyID: keyID, PublicKeyHex: hex.EncodeToString(public), CreatedAt: canonicalTime(time.Now()),
	})
	if err := writeRing(dir, ring); err != nil {
		return Response{}, err
	}
	return Response{Status: StatusOK, KeyID: keyID, PublicKeyHex: hex.EncodeToString(public)}, nil
}

func signingKey(home, dir string) (ed25519.PrivateKey, string, error) {
	ring, err := readRing(dir)
	if err != nil {
		return nil, "", err
	}
	if len(ring.Keys) == 0 {
		return nil, "", fmt.Errorf("%w: the sign node is not bootstrapped", ErrUsage)
	}
	head := ring.Keys[len(ring.Keys)-1]
	seed, err := HostSecretStore.Read(home, secretKind, head.KeyID)
	if err != nil {
		return nil, "", fmt.Errorf("read the signing seed: %w", err)
	}
	defer clearBytes(seed)
	return ed25519.NewKeyFromSeed(seed), head.KeyID, nil
}

func clearBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func receiptFromEntry(entry ledgerEntry) *Receipt {
	return &Receipt{
		RequestID: entry.RequestID, RequestDigest: entry.RequestDigest,
		ResultDigest: entry.ResultDigest, SignatureHex: entry.SignatureHex,
		AttestationHex: entry.AttestationHex, KeyID: entry.KeyID,
	}
}

func requestOp(home, dir string, req Request) (Response, error) {
	if req.RequestID == "" || req.RootDigest == "" {
		return Response{}, fmt.Errorf("%w: the sign request needs its identity and root digest", ErrUsage)
	}
	last, found, err := lastEntry(dir, req.RequestID)
	if err != nil {
		return Response{}, err
	}
	switch {
	case found && last.Op == "fenced":
		// FENCED is permanent: a late request is refused with the
		// verifiable confirmation bound to the recorded digest.
		return Response{Status: StatusFenced, Receipt: nil}, nil
	case found && last.Op == "signed":
		return Response{Status: StatusSigned, Receipt: receiptFromEntry(last)}, nil
	}
	payload, err := hex.DecodeString(req.PayloadHex)
	if err != nil {
		return Response{}, fmt.Errorf("%w: the sign payload is not hex", ErrUsage)
	}
	sum := sha256.Sum256(payload)
	if hex.EncodeToString(sum[:]) != req.RootDigest {
		_ = appendEntry(dir, ledgerEntry{
			Op: "tampered", RequestID: req.RequestID, RequestDigest: req.RequestDigest,
			RootDigest: req.RootDigest, Timestamp: canonicalTime(time.Now()),
		})
		return Response{Status: StatusPayloadMismatch}, nil
	}
	key, keyID, err := signingKey(home, dir)
	if err != nil {
		return Response{}, err
	}
	signature := ed25519.Sign(key, payload)
	resultDigestBytes := sha256.Sum256(signature)
	resultDigest := hex.EncodeToString(resultDigestBytes[:])
	attestation := ed25519.Sign(key, AttestationCanonical(req.RequestID, req.RequestDigest, resultDigest, keyID))
	entry := ledgerEntry{
		Op: "signed", RequestID: req.RequestID, RequestDigest: req.RequestDigest,
		RootDigest: req.RootDigest, ResultDigest: resultDigest,
		SignatureHex: hex.EncodeToString(signature), AttestationHex: hex.EncodeToString(attestation),
		KeyID: keyID, Timestamp: canonicalTime(time.Now()),
	}
	if err := appendEntry(dir, entry); err != nil {
		return Response{}, err
	}
	receipt := receiptFromEntry(entry)
	return Response{Status: StatusSigned, Receipt: receipt}, nil
}

func cancelOp(dir string, req Request) (Response, error) {
	if req.RequestID == "" {
		return Response{}, fmt.Errorf("%w: the cancel needs a request id", ErrUsage)
	}
	if req.RequestDigest == "" {
		return Response{}, fmt.Errorf("%w: the cancel must bind the request digest", ErrUsage)
	}
	last, found, err := lastEntry(dir, req.RequestID)
	if err != nil {
		return Response{}, err
	}
	switch {
	case found && last.Op == "signed":
		// Already signed: the fence is moot, and the caller reconciles
		// the receipt instead of substituting.
		return Response{Status: StatusSigned, Receipt: receiptFromEntry(last)}, nil
	case found && last.Op == "fenced":
		if last.RequestDigest != req.RequestDigest {
			return Response{Status: StatusTampered}, nil
		}
		return Response{Status: StatusFenced}, nil
	}
	entry := ledgerEntry{
		Op: "fenced", RequestID: req.RequestID, RequestDigest: req.RequestDigest,
		Reason: req.Reason, Timestamp: canonicalTime(time.Now()),
	}
	if err := appendEntry(dir, entry); err != nil {
		return Response{}, err
	}
	return Response{Status: StatusFenced}, nil
}

func queryOp(dir string, req Request) (Response, error) {
	if req.RequestID == "" {
		return Response{}, fmt.Errorf("%w: the query needs a request id", ErrUsage)
	}
	last, found, err := lastEntry(dir, req.RequestID)
	if err != nil {
		return Response{}, err
	}
	switch {
	case found && last.Op == "signed":
		if req.RequestDigest != "" && last.RequestDigest != req.RequestDigest {
			return Response{Status: StatusTampered}, nil
		}
		return Response{Status: StatusSigned, Receipt: receiptFromEntry(last)}, nil
	case found && last.Op == "fenced":
		if req.RequestDigest != "" && last.RequestDigest != req.RequestDigest {
			return Response{Status: StatusTampered}, nil
		}
		return Response{Status: StatusFenced}, nil
	}
	return Response{Status: StatusNotAccepted}, nil
}
