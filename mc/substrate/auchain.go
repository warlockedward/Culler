package substrate

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"
)

// ErrNotConfirmed is the serializer's fail-closed verdict: a grammar,
// vocabulary, unknown-field, or family-table violation means the canonical
// bytes cannot be established. The event is refused rather than encoded
// under a guessed interpretation.
var ErrNotConfirmed = errors.New("canonical encoding not confirmed")

// ErrPrevHashMismatch identifies an append whose carried prev_hash does not
// agree with the chain state (the genesis value for a first event, the
// stored chain_digest of seq-1 otherwise). The append is refused.
var ErrPrevHashMismatch = errors.New("event prev_hash does not match the chain state")

// canonicalSerializerID is this release's serializer identity. It is the
// first field of the hash domain, so a change of encoding rules yields a
// different digest under a new id rather than silently reinterpreting
// stored events.
const canonicalSerializerID = "shz-canon-v1"

// genesisSalt is the domain-separation constant of the chain genesis.
const genesisSalt = "shz-genesis-v1:"

// emptyRootDomain is the domain constant of the empty chain-head set root.
const emptyRootDomain = "shz-root-empty-v1"

// merkleAlgorithmID identifies the root algorithm. A root is interpreted
// only under the id that produced it.
const merkleAlgorithmID = "shz-merkle-v1"

var (
	decimalRe   = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)
	decIntRe    = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)
	canonTimeRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z$`)
	hex32Re     = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

const (
	tagAbsent byte = 0x00
	tagNull   byte = 0x01
	tagValue  byte = 0x02
)

// fieldSpec is one row of a declaration-ordered field table. The
// declaration order is the hash order: a sorted or alphabetical encoding
// diverges from the frozen vectors.
type fieldSpec struct {
	name     string
	kind     string
	optional bool
}

// envelopeFields is the 14-field envelope table in declaration order. The
// payload slot resolves to the family's table at encode time from the
// event's payload_family.
var envelopeFields = []fieldSpec{
	{"serializer_id", "enum:" + canonicalSerializerID + "|shz-canon-v0", false},
	{"event_id", "hex32", false},
	{"aggregate_id", "str", false},
	{"aggregate_type", "str", false},
	{"seq", "dec_int", false},
	{"event_type", "str", false},
	{"occurred_at", "time", false},
	{"recorded_at", "time", false},
	{"effective_at", "time", true},
	{"supersedes_event_id", "hex32", true},
	{"causation_id", "str", true},
	{"correlation_id", "str", true},
	{"prev_hash", "hex32", false},
	{"payload", "obj", false},
}

// payloadSchemas is the registered v1 payload family table. The three
// vector-backed families match the registered design piece; aggregate_open
// and migration are addition-only v1 appendix families registered by the
// chain implementation — their field tables exist, their vectors do not
// yet (registered-pending-vectors, not verified).
var payloadSchemas = map[string][]fieldSpec{
	"usage_observation": {
		{"model_id", "str", true},
		{"output_tokens", "dec", true},
		{"input_tokens", "dec", true},
		{"provider_request_id", "str", false},
		{"local_request_id", "str", true},
		{"origin", "enum:provider|local", false},
		{"event_seq", "dec_int", false},
		{"source_version", "dec_int", false},
		{"report_kind", "enum:cumulative|incremental", false},
		{"finality", "enum:interim|final", false},
		{"identity_provability", "enum:native|cursor|none", false},
		{"usage_family", "str", false},
	},
	"usage_settle": {
		{"source_report_id", "str", false},
		{"authorization_id", "str", false},
		{"provider_request_id", "str", false},
		{"source_version", "dec_int", false},
		{"payload_digest", "hex32", false},
		{"supersedes_report_id", "str", true},
		{"effect", "enum:reserved|confirmed|released|expired|partial_settled|reconciliation|written_off", false},
	},
	"admission": {
		{"grant_id", "str", false},
		{"reason", "str", true},
		{"expires_at", "time", true},
	},
	"probe_order": {
		{"zeta", "str", false},
		{"alpha", "str", false},
	},
	"aggregate_open": {
		{"actor", "str", false},
		{"reason", "str", true},
	},
	"migration": {
		{"step_name", "str", false},
		{"from_schema_id", "str", false},
		{"to_schema_id", "str", false},
		{"version", "dec_int", false},
	},
}

// encLengthPrefixed appends the u64be byte-length prefix and the raw UTF-8
// bytes. There is no escape layer: the prefix frames the payload.
func encLengthPrefixed(out *[]byte, s string) {
	b := []byte(s)
	var prefix [8]byte
	binary.BigEndian.PutUint64(prefix[:], uint64(len(b)))
	*out = append(*out, prefix[:]...)
	*out = append(*out, b...)
}

// encScalar renders one VALUE payload under its kind's grammar. A grammar
// violation is ErrNotConfirmed, never a repair.
func encScalar(kind string, v any, name string) ([]byte, error) {
	s, ok := v.(string)
	if !ok {
		return nil, fmt.Errorf("%w: field %q must carry a string value", ErrNotConfirmed, name)
	}
	switch {
	case kind == "dec":
		if !decimalRe.MatchString(s) {
			return nil, fmt.Errorf("%w: decimal lexical form rejected: %q", ErrNotConfirmed, s)
		}
	case kind == "dec_int":
		if !decIntRe.MatchString(s) {
			return nil, fmt.Errorf("%w: decimal-integer form rejected: %q", ErrNotConfirmed, s)
		}
	case kind == "time":
		if !canonTimeRe.MatchString(s) {
			return nil, fmt.Errorf("%w: time must be canonical RFC3339 UTC microsecond 'Z' form: %q", ErrNotConfirmed, s)
		}
	case kind == "hex32":
		if !hex32Re.MatchString(s) {
			return nil, fmt.Errorf("%w: hex32 must be 64 lowercase hex chars: %q", ErrNotConfirmed, s)
		}
	case strings.HasPrefix(kind, "enum:"):
		if !slices.Contains(strings.Split(kind[len("enum:"):], "|"), s) {
			return nil, fmt.Errorf("%w: enum rejects %q in field %q", ErrNotConfirmed, s, name)
		}
	case kind == "str":
		// NFC normalization is part of the canonical form.
		s = norm.NFC.String(s)
	default:
		return nil, fmt.Errorf("%w: unknown scalar kind %s", ErrNotConfirmed, kind)
	}
	var out []byte
	encLengthPrefixed(&out, s)
	return out, nil
}

// encObject renders the fields of table in declaration order against obj.
// A key absent from obj is ABSENT (legal only for optional fields), a nil
// value is NULL (legal only for optional fields), anything else is a VALUE.
// Unknown keys are refused.
func encObject(out *[]byte, table []fieldSpec, obj map[string]any, ctx string) error {
	declared := make(map[string]bool, len(table))
	for _, f := range table {
		declared[f.name] = true
	}
	for key := range obj {
		if !declared[key] {
			return fmt.Errorf("%w: unknown field %q in %s", ErrNotConfirmed, key, ctx)
		}
	}
	for _, f := range table {
		value, present := obj[f.name]
		switch {
		case !present:
			if !f.optional {
				return fmt.Errorf("%w: required field %q missing in %s", ErrNotConfirmed, f.name, ctx)
			}
			*out = append(*out, tagAbsent)
		case value == nil:
			if !f.optional {
				return fmt.Errorf("%w: required field %q cannot be explicit NULL in %s", ErrNotConfirmed, f.name, ctx)
			}
			*out = append(*out, tagNull)
		default:
			*out = append(*out, tagValue)
			b, err := encScalar(f.kind, value, f.name)
			if err != nil {
				return err
			}
			*out = append(*out, b...)
		}
	}
	return nil
}

// EncodeCanonicalEvent renders ev under shz-canon-v1. ev carries the 14
// envelope keys plus "payload_family" (which selects the payload table)
// and "payload" itself. An absent key is ABSENT, a nil value NULL,
// anything else VALUE.
func EncodeCanonicalEvent(ev map[string]any) ([]byte, error) {
	family, _ := ev["payload_family"].(string)
	table, known := payloadSchemas[family]
	if !known {
		return nil, fmt.Errorf("%w: payload family %q has no registered field table", ErrNotConfirmed, family)
	}
	declared := make(map[string]bool, len(envelopeFields)+1)
	for _, f := range envelopeFields {
		declared[f.name] = true
	}
	declared["payload_family"] = true
	for key := range ev {
		if !declared[key] {
			return nil, fmt.Errorf("%w: unknown envelope field %q", ErrNotConfirmed, key)
		}
	}
	var out []byte
	for _, f := range envelopeFields {
		if f.name == "payload" {
			payload, present := ev["payload"]
			if !present {
				return nil, fmt.Errorf("%w: required envelope field payload missing", ErrNotConfirmed)
			}
			obj, ok := payload.(map[string]any)
			if payload == nil || !ok {
				return nil, fmt.Errorf("%w: required envelope field payload cannot be explicit NULL", ErrNotConfirmed)
			}
			out = append(out, tagValue)
			if err := encObject(&out, table, obj, "payload"); err != nil {
				return nil, err
			}
			continue
		}
		value, present := ev[f.name]
		switch {
		case !present:
			if !f.optional {
				return nil, fmt.Errorf("%w: required envelope field %q missing", ErrNotConfirmed, f.name)
			}
			out = append(out, tagAbsent)
		case value == nil:
			if !f.optional {
				return nil, fmt.Errorf("%w: required envelope field %q cannot be explicit NULL", ErrNotConfirmed, f.name)
			}
			out = append(out, tagNull)
		default:
			out = append(out, tagValue)
			b, err := encScalar(f.kind, value, f.name)
			if err != nil {
				return nil, err
			}
			out = append(out, b...)
		}
	}
	return out, nil
}

// GenesisHash is the prev_hash a chain's first event carries: SHA-256 over
// the domain-separation salt and the NFC aggregate type. Different types
// derive different genesis values, so a chain cannot be transplanted
// across aggregate types without the verifier noticing.
func GenesisHash(aggregateType string) string {
	sum := sha256.Sum256([]byte(genesisSalt + norm.NFC.String(aggregateType)))
	return hex.EncodeToString(sum[:])
}

// EventDigest is SHA-256 over the canonical event bytes.
func EventDigest(canonical []byte) string {
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

// ChainDigest is one recursion step: SHA-256 over the raw 32-byte
// predecessor digest and the raw 32-byte event digest.
func ChainDigest(prevHash, eventDigest string) (string, error) {
	prev, err := hex.DecodeString(prevHash)
	if err != nil || len(prev) != 32 {
		return "", fmt.Errorf("%w: predecessor digest must be 32 bytes", ErrNotConfirmed)
	}
	digest, err := hex.DecodeString(eventDigest)
	if err != nil || len(digest) != 32 {
		return "", fmt.Errorf("%w: event digest must be 32 bytes", ErrNotConfirmed)
	}
	combined := make([]byte, 0, 64)
	combined = append(combined, prev...)
	combined = append(combined, digest...)
	sum := sha256.Sum256(combined)
	return hex.EncodeToString(sum[:]), nil
}

// NewEventID allocates a random 32-byte event identity.
func NewEventID() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("allocate event id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// canonNow is the fixed-width RFC3339 UTC microsecond wall time.
func canonNow() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000000Z")
}

// auditor is the statement surface a chain path runs on: *sql.DB,
// *sql.Tx, or the dedicated connection AppendAuditEvent takes when it
// owns the transaction.
type auditor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// AppendAuditEvent appends one event to ev's aggregate's chain. seq is
// assigned as max(seq)+1 under the transaction's serialization, and ev
// must carry the expected predecessor in "prev_hash" (GenesisHash of the
// aggregate type for a first event, the stored chain_digest of seq-1
// otherwise); disagreement is refused fail-closed rather than repaired.
//
// When q is a *sql.Tx the append joins that transaction and the caller
// owns the commit — this is the atomic-audit path, where the state change
// and its chain evidence commit together. Otherwise q is a *sql.DB and
// the append runs under its own BEGIN IMMEDIATE.
func AppendAuditEvent(q auditor, ev map[string]any) error {
	ctx := context.Background()
	event := make(map[string]any, len(ev)+1)
	for k, v := range ev {
		event[k] = v
	}

	aggregateID, _ := event["aggregate_id"].(string)
	if aggregateID == "" {
		return fmt.Errorf("%w: aggregate_id is required", ErrNotConfirmed)
	}

	var runner auditor
	var conn *sql.Conn
	ownsTx := false
	switch surface := q.(type) {
	case *sql.Tx:
		runner = surface
	case *sql.DB:
		c, err := surface.Conn(ctx)
		if err != nil {
			return fmt.Errorf("acquire audit connection: %w", err)
		}
		defer c.Close()
		if _, err := c.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
			return fmt.Errorf("begin audit transaction: %w", err)
		}
		runner = c
		conn = c
		ownsTx = true
	default:
		return fmt.Errorf("audit append needs a spine database or an open transaction, got %T", q)
	}
	finish := func(err error) error {
		if !ownsTx {
			return err
		}
		if err == nil {
			if _, commitErr := conn.ExecContext(ctx, "COMMIT"); commitErr != nil {
				_, _ = conn.ExecContext(ctx, "ROLLBACK")
				return fmt.Errorf("commit audit append: %w", commitErr)
			}
			return nil
		}
		_, _ = conn.ExecContext(ctx, "ROLLBACK")
		return err
	}

	var seq int
	{
		rows, err := runner.QueryContext(ctx, `SELECT COALESCE(MAX(seq), 0) + 1 FROM audit_events WHERE aggregate_id = ?`, aggregateID)
		if err != nil {
			return finish(fmt.Errorf("assign audit seq for %s: %w", aggregateID, err))
		}
		defer rows.Close()
		if !rows.Next() {
			return finish(fmt.Errorf("assign audit seq for %s: no result row", aggregateID))
		}
		if err := rows.Scan(&seq); err != nil {
			return finish(fmt.Errorf("assign audit seq for %s: %w", aggregateID, err))
		}
		if err := rows.Err(); err != nil {
			return finish(err)
		}
	}

	var expected string
	if seq == 1 {
		aggregateType, _ := event["aggregate_type"].(string)
		if aggregateType == "" {
			return finish(fmt.Errorf("%w: aggregate_type is required", ErrNotConfirmed))
		}
		expected = GenesisHash(aggregateType)
	} else {
		rows, err := runner.QueryContext(ctx, `SELECT chain_digest FROM audit_events WHERE aggregate_id = ? AND seq = ?`, aggregateID, seq-1)
		if err != nil {
			return finish(fmt.Errorf("read audit predecessor for %s: %w", aggregateID, err))
		}
		defer rows.Close()
		found := false
		for rows.Next() {
			if err := rows.Scan(&expected); err != nil {
				return finish(fmt.Errorf("read audit predecessor for %s: %w", aggregateID, err))
			}
			found = true
		}
		if err := rows.Err(); err != nil {
			return finish(err)
		}
		if !found {
			return finish(fmt.Errorf("audit chain state broken: aggregate %s has no row at seq %d", aggregateID, seq-1))
		}
	}

	carried, _ := event["prev_hash"].(string)
	if carried != expected {
		return finish(fmt.Errorf("%w: event carries %s, the chain state requires %s", ErrPrevHashMismatch, carried, expected))
	}

	event["seq"] = strconv.Itoa(seq)
	canonical, err := EncodeCanonicalEvent(event)
	if err != nil {
		return finish(err)
	}
	eventDigest := EventDigest(canonical)
	chainDigest, err := ChainDigest(expected, eventDigest)
	if err != nil {
		return finish(err)
	}

	payload, _ := event["payload"].(map[string]any)
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return finish(fmt.Errorf("encode audit payload: %w", err))
	}
	_, err = runner.ExecContext(ctx, `INSERT INTO audit_events (
		aggregate_id, seq, event_id, aggregate_type, event_type, serializer_id,
		occurred_at, recorded_at, effective_at, supersedes_event_id,
		causation_id, correlation_id, prev_hash, event_digest, chain_digest,
		payload_family, payload_json
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		aggregateID,
		seq,
		eventString(event, "event_id"),
		eventString(event, "aggregate_type"),
		eventString(event, "event_type"),
		eventString(event, "serializer_id"),
		eventString(event, "occurred_at"),
		eventString(event, "recorded_at"),
		eventNullable(event, "effective_at"),
		eventNullable(event, "supersedes_event_id"),
		eventNullable(event, "causation_id"),
		eventNullable(event, "correlation_id"),
		carried,
		eventDigest,
		chainDigest,
		eventString(event, "payload_family"),
		string(payloadJSON),
	)
	if err != nil {
		return finish(fmt.Errorf("insert audit event for %s: %w", aggregateID, err))
	}
	return finish(nil)
}

func eventString(ev map[string]any, name string) string {
	s, _ := ev[name].(string)
	return s
}

// eventNullable renders an optional field for storage: absent or nil both
// store SQL NULL (the ABSENT form under the storage convention).
func eventNullable(ev map[string]any, name string) any {
	v, present := ev[name]
	if !present || v == nil {
		return nil
	}
	s, _ := v.(string)
	return s
}

// OpenAggregate records the lifecycle opening of aggregateID under
// aggregateType: an aggregate_open event whose prev_hash is the genesis
// value, so it is appendable only as the aggregate's first event. Once the
// aggregate carries any row the call is refused (ErrPrevHashMismatch).
//
// The aggregate_open row registers lifecycle only. It is not a basis for
// whole-root disappearance detection: with no independently anchored
// chain-head set in place, an aggregate deleted together with its open
// event leaves no local trace, and this path does not claim otherwise.
func OpenAggregate(db *sql.DB, aggregateID, aggregateType, actor string, reason string) error {
	payload := map[string]any{"actor": actor}
	if reason != "" {
		payload["reason"] = reason
	}
	eventID, err := NewEventID()
	if err != nil {
		return err
	}
	now := canonNow()
	return AppendAuditEvent(db, map[string]any{
		"serializer_id":  canonicalSerializerID,
		"event_id":       eventID,
		"aggregate_id":   aggregateID,
		"aggregate_type": aggregateType,
		"event_type":     "aggregate_open",
		"occurred_at":    now,
		"recorded_at":    now,
		"prev_hash":      GenesisHash(aggregateType),
		"payload_family": "aggregate_open",
		"payload":        payload,
	})
}

// ChainHead is one aggregate's chain head: its last stored row's seq and
// chain_digest.
type ChainHead struct {
	AggregateID string
	SeqMax      int
	ChainDigest string
}

// ChainError is one layer-1 finding. Kind is a stable slug: broken-chain
// (the prev_hash double relation fails, or the recursion does not close),
// digest-mismatch (self-computed vs stored event_digest — the fork /
// corruption signal under a single writer), seq-gap (a hole in the seq
// prefix), duplicate-event-id, unserializable-row (a stored row the
// canonical rules cannot render).
type ChainError struct {
	AggregateID string
	Seq         int
	Kind        string
	Message     string
}

// ChainReport is the verifier's full output: one head per aggregate and
// every finding. An empty report over an empty table is a valid result:
// the root of the empty chain-head set is the EMPTY_ROOT constant.
type ChainReport struct {
	Heads  []ChainHead
	Errors []ChainError
}

type auditRow struct {
	AggregateID       string
	Seq               int
	EventID           string
	AggregateType     string
	EventType         string
	SerializerID      string
	OccurredAt        string
	RecordedAt        string
	EffectiveAt       sql.NullString
	SupersedesEventID sql.NullString
	CausationID       sql.NullString
	CorrelationID     sql.NullString
	PrevHash          string
	EventDigest       string
	ChainDigest       string
	PayloadFamily     string
	PayloadJSON       string
}

// VerifyAuditChain recomputes every chain from the stored rows and reports
// the layer-1 findings. Both prev_hash relations are checked: the stored
// field against the predecessor's stored chain_digest, and the recursion
// itself against the recomputed digests. aggregateFilter restricts the
// report to one aggregate.
func VerifyAuditChain(db *sql.DB, aggregateFilter string) (ChainReport, error) {
	report := ChainReport{Heads: []ChainHead{}, Errors: []ChainError{}}
	var (
		rows *sql.Rows
		err  error
	)
	if aggregateFilter != "" {
		rows, err = db.Query(`SELECT aggregate_id, seq, event_id, aggregate_type, event_type, serializer_id,
			occurred_at, recorded_at, effective_at, supersedes_event_id, causation_id,
			correlation_id, prev_hash, event_digest, chain_digest, payload_family, payload_json
			FROM audit_events WHERE aggregate_id = ? ORDER BY aggregate_id, seq`, aggregateFilter)
	} else {
		rows, err = db.Query(`SELECT aggregate_id, seq, event_id, aggregate_type, event_type, serializer_id,
			occurred_at, recorded_at, effective_at, supersedes_event_id, causation_id,
			correlation_id, prev_hash, event_digest, chain_digest, payload_family, payload_json
			FROM audit_events ORDER BY aggregate_id, seq`)
	}
	if err != nil {
		return report, fmt.Errorf("read audit events: %w", err)
	}
	defer rows.Close()
	stored := []auditRow{}
	for rows.Next() {
		var r auditRow
		if err := rows.Scan(&r.AggregateID, &r.Seq, &r.EventID, &r.AggregateType, &r.EventType,
			&r.SerializerID, &r.OccurredAt, &r.RecordedAt, &r.EffectiveAt,
			&r.SupersedesEventID, &r.CausationID, &r.CorrelationID, &r.PrevHash,
			&r.EventDigest, &r.ChainDigest, &r.PayloadFamily, &r.PayloadJSON); err != nil {
			return report, fmt.Errorf("read audit event row: %w", err)
		}
		stored = append(stored, r)
	}
	if err := rows.Err(); err != nil {
		return report, fmt.Errorf("read audit events: %w", err)
	}

	byAggregate := map[string][]auditRow{}
	order := []string{}
	for _, r := range stored {
		if _, seen := byAggregate[r.AggregateID]; !seen {
			order = append(order, r.AggregateID)
		}
		byAggregate[r.AggregateID] = append(byAggregate[r.AggregateID], r)
	}
	for _, aggID := range order {
		verifyAggregateChain(&report, byAggregate[aggID])
	}
	return report, nil
}

func verifyAggregateChain(report *ChainReport, rows []auditRow) {
	aggID := rows[0].AggregateID
	seenEventIDs := map[string]bool{}
	for i, r := range rows {
		// The k-th stored row of an aggregate must sit at seq k: anything
		// else is a hole in the prefix (missing events).
		if r.Seq != i+1 {
			report.Errors = append(report.Errors, ChainError{
				AggregateID: aggID,
				Seq:         r.Seq,
				Kind:        "seq-gap",
				Message:     fmt.Sprintf("row sits at seq %d but the stored prefix reaches seq %d", r.Seq, i+1),
			})
		}
		// Relation 1: the stored prev_hash field against the expected
		// predecessor (genesis for seq 1, the predecessor's stored
		// chain_digest otherwise).
		expectedPrev := ""
		if r.Seq == 1 {
			expectedPrev = GenesisHash(r.AggregateType)
		} else {
			for j := i - 1; j >= 0; j-- {
				if rows[j].Seq == r.Seq-1 {
					expectedPrev = rows[j].ChainDigest
					break
				}
			}
		}
		if r.PrevHash != expectedPrev {
			report.Errors = append(report.Errors, ChainError{
				AggregateID: aggID,
				Seq:         r.Seq,
				Kind:        "broken-chain",
				Message:     fmt.Sprintf("prev_hash %s does not match the chain state %s", r.PrevHash, expectedPrev),
			})
		}
		// The UNIQUE constraint makes a repeated event_id unreachable
		// through SQL; a file-level forgery can still present it, so the
		// re-verification stays.
		if seenEventIDs[r.EventID] {
			report.Errors = append(report.Errors, ChainError{
				AggregateID: aggID,
				Seq:         r.Seq,
				Kind:        "duplicate-event-id",
				Message:     fmt.Sprintf("event_id %s appears more than once in the chain", r.EventID),
			})
		}
		seenEventIDs[r.EventID] = true

		// Recompute the canonical bytes from the stored row. Optional
		// columns read back as ABSENT (the storage convention).
		ev := map[string]any{
			"serializer_id":  r.SerializerID,
			"event_id":       r.EventID,
			"aggregate_id":   r.AggregateID,
			"aggregate_type": r.AggregateType,
			"seq":            strconv.Itoa(r.Seq),
			"event_type":     r.EventType,
			"occurred_at":    r.OccurredAt,
			"recorded_at":    r.RecordedAt,
			"prev_hash":      r.PrevHash,
			"payload_family": r.PayloadFamily,
		}
		if r.EffectiveAt.Valid {
			ev["effective_at"] = r.EffectiveAt.String
		}
		if r.SupersedesEventID.Valid {
			ev["supersedes_event_id"] = r.SupersedesEventID.String
		}
		if r.CausationID.Valid {
			ev["causation_id"] = r.CausationID.String
		}
		if r.CorrelationID.Valid {
			ev["correlation_id"] = r.CorrelationID.String
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(r.PayloadJSON), &payload); err != nil || payload == nil {
			payload = map[string]any{}
		}
		ev["payload"] = payload
		canonical, err := EncodeCanonicalEvent(ev)
		if err != nil {
			report.Errors = append(report.Errors, ChainError{
				AggregateID: aggID,
				Seq:         r.Seq,
				Kind:        "unserializable-row",
				Message:     err.Error(),
			})
			continue
		}
		recomputed := EventDigest(canonical)
		if recomputed != r.EventDigest {
			report.Errors = append(report.Errors, ChainError{
				AggregateID: aggID,
				Seq:         r.Seq,
				Kind:        "digest-mismatch",
				Message:     fmt.Sprintf("stored event_digest %s, recomputed %s", r.EventDigest, recomputed),
			})
		}
		// Relation 2: the recursion must close over the stored row.
		closed, cerr := ChainDigest(r.PrevHash, recomputed)
		if cerr != nil || closed != r.ChainDigest {
			report.Errors = append(report.Errors, ChainError{
				AggregateID: aggID,
				Seq:         r.Seq,
				Kind:        "broken-chain",
				Message:     fmt.Sprintf("stored chain_digest %s does not close the recursion (recomputed %s)", r.ChainDigest, closed),
			})
		}
	}
	report.Heads = append(report.Heads, ChainHead{
		AggregateID: aggID,
		SeqMax:      rows[len(rows)-1].Seq,
		ChainDigest: rows[len(rows)-1].ChainDigest,
	})
}

// ComputeRoot builds the shz-merkle-v1 root over the chain-head set. The
// heads are ordered by NFC(aggregate_id) raw UTF-8 bytes ascending (case-
// and locale-blind); duplicate ids and non-32-byte digests are refused,
// never silently deduplicated or truncated. An empty set yields the
// EMPTY_ROOT domain constant; a single-leaf set yields that leaf itself,
// un-rehashed.
func ComputeRoot(heads []ChainHead) (algorithmID string, leafCount int, rootHex string, err error) {
	if len(heads) == 0 {
		sum := sha256.Sum256([]byte(emptyRootDomain))
		return merkleAlgorithmID, 0, hex.EncodeToString(sum[:]), nil
	}
	type entry struct {
		id     []byte // NFC form
		seq    int
		digest []byte // raw 32 bytes
	}
	entries := make([]entry, 0, len(heads))
	for _, h := range heads {
		id := []byte(norm.NFC.String(h.AggregateID))
		digest, derr := hex.DecodeString(h.ChainDigest)
		if derr != nil || len(digest) != 32 {
			return "", 0, "", fmt.Errorf("chain_digest must be 32 bytes")
		}
		entries = append(entries, entry{id: id, seq: h.SeqMax, digest: digest})
	}
	sort.Slice(entries, func(i, j int) bool {
		return bytes.Compare(entries[i].id, entries[j].id) < 0
	})
	leaves := make([][]byte, 0, len(entries))
	for i, e := range entries {
		if i > 0 && bytes.Equal(entries[i-1].id, e.id) {
			// The message shape matches the reference derivation's, so
			// the cross-implementation vectors pin it.
			return "", 0, "", fmt.Errorf("duplicate aggregate_id in chain-head set: '%s'", string(e.id))
		}
		leaf := sha256.New()
		leaf.Write([]byte{0x00})
		var lenPrefix [8]byte
		binary.BigEndian.PutUint64(lenPrefix[:], uint64(len(e.id)))
		leaf.Write(lenPrefix[:])
		leaf.Write(e.id)
		var seqPrefix [8]byte
		binary.BigEndian.PutUint64(seqPrefix[:], uint64(e.seq))
		leaf.Write(seqPrefix[:])
		leaf.Write(e.digest)
		leaves = append(leaves, leaf.Sum(nil))
	}
	level := leaves
	for len(level) > 1 {
		if len(level)%2 == 1 {
			level = append(level, level[len(level)-1])
		}
		next := make([][]byte, 0, len(level)/2)
		for i := 0; i < len(level); i += 2 {
			combined := make([]byte, 0, 65)
			combined = append(combined, 0x01)
			combined = append(combined, level[i]...)
			combined = append(combined, level[i+1]...)
			sum := sha256.Sum256(combined)
			next = append(next, sum[:])
		}
		level = next
	}
	return merkleAlgorithmID, len(entries), hex.EncodeToString(level[0]), nil
}
