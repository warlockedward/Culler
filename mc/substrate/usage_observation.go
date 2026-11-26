package substrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ErrUsageEventUnknown identifies an observation that references a raw
// usage event this spine never persisted (phase two before phase one).
var ErrUsageEventUnknown = errors.New("the usage observation references a raw usage event that was never persisted")

// ErrUsageVersionCollision identifies a content conflict at a fixed
// (identity, sequence, version) slot. The conflict is refused, never
// last-write-wins: the stored row is kept and the claimant must move to
// the next version, where the divergence is recorded as reconciliation.
var ErrUsageVersionCollision = errors.New("a usage observation version already carries different content")

// ErrUsageOriginClaim identifies a broken identity-origin red line: a
// provider-native claim without a provider request ID, or a local origin
// carrying one. Locally generated identities never impersonate
// provider-native ones.
var ErrUsageOriginClaim = errors.New("the usage observation origin does not match its request identity")

// ErrUsageSupersedesUnknown identifies a supersedes reference to an
// observation that does not exist.
var ErrUsageSupersedesUnknown = errors.New("the usage observation supersedes an unknown observation")

const usageAggregateID = "usage"

// usageWriter is the statement surface the usage paths run on inside the
// caller's open transaction. It is the same shape as the query surface the
// verbs layer passes down, so the two are interchangeable.
type usageWriter interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// canonicalTime renders a moment in the chain's canonical time form:
// RFC3339 UTC with fixed-width microseconds, the shape the 27-character
// schema fence and the serializer's time grammar both require.
func canonicalTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000000Z")
}

func sqlNullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// EncodeUsageObservationPayload renders a usage_observation payload under
// shz-canon-v1 (the B03 frozen field table) and returns the SHA-256
// digest of those canonical bytes: the payload_digest value, and the
// content identity the version-collision check compares.
func EncodeUsageObservationPayload(payload map[string]any) (string, error) {
	var out []byte
	if err := encObject(&out, payloadSchemas["usage_observation"], payload, "usage_observation"); err != nil {
		return "", err
	}
	sum := sha256.Sum256(out)
	return hex.EncodeToString(sum[:]), nil
}

// UsageEventInput is the raw provider usage event, phase one.
type UsageEventInput struct {
	LocalRequestID    string
	ProviderRequestID string
	NativeEventID     string
	UsageFamily       string
	RawJSON           string
	OccurredAt        time.Time
}

// UsageEventResult reports the persisted, or re-entered, raw event.
type UsageEventResult struct {
	RequestIdentity string
	EventSeq        int64
	Created         bool
}

// RecordUsageEvent persists the raw event and assigns its sequence number.
// The number is assigned on first persistence (max+1 under the single
// writer's transaction) and never reassigned: a redelivered input that is
// addressable through a provider-native event ID or an upstream cursor
// re-enters its existing row (Created=false) instead of taking a new
// number. Events without such an identity each take the next number, and
// their sameness is not claimed — that is the provability=none channel.
func RecordUsageEvent(ctx context.Context, q usageWriter, in UsageEventInput) (UsageEventResult, error) {
	identity := in.ProviderRequestID
	origin := "provider"
	if identity == "" {
		identity, origin = in.LocalRequestID, "local"
	}
	occurred := canonicalTime(in.OccurredAt)
	recorded := canonNow()

	reenter := func() (UsageEventResult, error) {
		var seq int64
		err := q.QueryRowContext(ctx,
			`SELECT event_seq FROM usage_events WHERE request_identity = ? AND native_event_id = ?`,
			identity, in.NativeEventID).Scan(&seq)
		if err != nil {
			return UsageEventResult{}, fmt.Errorf("re-enter raw usage event: %w", err)
		}
		return UsageEventResult{RequestIdentity: identity, EventSeq: seq, Created: false}, nil
	}
	if in.NativeEventID != "" {
		if result, err := reenter(); err == nil {
			return result, nil
		} else if !errors.Is(err, sql.ErrNoRows) {
			return UsageEventResult{}, err
		}
	}

	var maxSeq int64
	if err := q.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(event_seq), 0) FROM usage_events WHERE request_identity = ?`,
		identity).Scan(&maxSeq); err != nil {
		return UsageEventResult{}, fmt.Errorf("assign usage event sequence: %w", err)
	}
	seq := maxSeq + 1
	_, err := q.ExecContext(ctx, `INSERT INTO usage_events (
		request_identity, identity_origin, local_request_id, provider_request_id,
		native_event_id, event_seq, usage_family, raw_json, occurred_at, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		identity, origin, in.LocalRequestID, sqlNullable(in.ProviderRequestID),
		sqlNullable(in.NativeEventID), seq, in.UsageFamily, in.RawJSON, occurred, recorded)
	if err != nil {
		if in.NativeEventID != "" && strings.Contains(err.Error(), "UNIQUE constraint failed: usage_events") {
			// A concurrent first persistence of the same native event won
			// the unique index; the redelivery re-enters that row.
			if result, reenterErr := reenter(); reenterErr == nil {
				return result, nil
			}
		}
		return UsageEventResult{}, fmt.Errorf("persist raw usage event: %w", err)
	}
	return UsageEventResult{RequestIdentity: identity, EventSeq: seq, Created: true}, nil
}

// UsageObservationInput is the derived usage fact, phase two. The token
// counts carry their decimal lexical form; the form survives to the
// digest input, so 1 and 1.0 remain different facts.
type UsageObservationInput struct {
	LocalRequestID          string
	ProviderRequestID       string
	EventSeq                int64
	SourceVersion           int64
	Origin                  string
	UsageFamily             string
	ModelID                 string
	OutputTokens            string
	InputTokens             string
	ReportKind              string
	Finality                string
	IdentityProvability     string
	SupersedesObservationID string
	OccurredAt              time.Time
}

// UsageObservationResult reports the persisted, or re-entered,
// observation and the state its channel assigns.
type UsageObservationResult struct {
	ObservationID string
	State         string
	Created       bool
}

// RecordUsageObservation persists the derived fact and appends its chain
// event in the same transaction: the usage state and its Class A evidence
// commit together, and neither can exist without the other. The usage
// aggregate opens automatically on first use, in the same transaction.
func RecordUsageObservation(ctx context.Context, q usageWriter, in UsageObservationInput) (UsageObservationResult, error) {
	if (in.Origin == "provider") != (in.ProviderRequestID != "") {
		return UsageObservationResult{}, ErrUsageOriginClaim
	}
	identity := in.ProviderRequestID
	if identity == "" {
		identity = in.LocalRequestID
	}

	var raw int
	err := q.QueryRowContext(ctx,
		`SELECT 1 FROM usage_events WHERE request_identity = ? AND event_seq = ?`,
		identity, in.EventSeq).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return UsageObservationResult{}, ErrUsageEventUnknown
		}
		return UsageObservationResult{}, fmt.Errorf("verify raw usage event: %w", err)
	}

	payload := usageObservationPayload(in)
	digest, err := EncodeUsageObservationPayload(payload)
	if err != nil {
		return UsageObservationResult{}, err
	}

	var existingDigest, existingState, existingID string
	err = q.QueryRowContext(ctx,
		`SELECT payload_digest, state, observation_id FROM usage_observations
		 WHERE request_identity = ? AND event_seq = ? AND source_version = ?`,
		identity, in.EventSeq, in.SourceVersion).Scan(&existingDigest, &existingState, &existingID)
	if err == nil {
		if existingDigest == digest {
			return UsageObservationResult{ObservationID: existingID, State: existingState, Created: false}, nil
		}
		return UsageObservationResult{}, ErrUsageVersionCollision
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return UsageObservationResult{}, fmt.Errorf("verify usage observation version: %w", err)
	}

	state := "reported"
	if in.IdentityProvability == "none" {
		// Events whose sameness cannot be proven never enter the
		// reported channel, whatever their content.
		state = "unknown"
	} else if in.Finality == "final" {
		// A second terminal claim conflicts only when its content
		// differs from the latest terminal's: the version is part of
		// the row identity, not of the claim's content, so the
		// comparison runs over the content fields.
		var prevFinality, prevOutput, prevInput, prevKind, prevOrigin, prevProvability, prevFamily, prevModel string
		err := q.QueryRowContext(ctx,
			`SELECT finality, output_tokens, input_tokens, report_kind, origin, identity_provability, usage_family, model_id
			 FROM usage_observations
			 WHERE request_identity = ? AND event_seq = ? AND source_version < ?
			 ORDER BY source_version DESC LIMIT 1`,
			identity, in.EventSeq, in.SourceVersion).
			Scan(&prevFinality, &prevOutput, &prevInput, &prevKind, &prevOrigin, &prevProvability, &prevFamily, &prevModel)
		if err == nil {
			contentMatches := prevOutput == in.OutputTokens && prevInput == in.InputTokens &&
				prevKind == in.ReportKind && prevOrigin == in.Origin &&
				prevProvability == in.IdentityProvability && prevFamily == in.UsageFamily &&
				prevModel == in.ModelID
			if prevFinality == "final" && !contentMatches {
				// Both terminal claims stay; the newer is flagged for
				// reconciliation, nothing is overwritten.
				state = "reconciliation"
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return UsageObservationResult{}, fmt.Errorf("read usage observation slot history: %w", err)
		}
	}

	if in.SupersedesObservationID != "" {
		var n int
		err := q.QueryRowContext(ctx,
			`SELECT 1 FROM usage_observations WHERE observation_id = ?`,
			in.SupersedesObservationID).Scan(&n)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return UsageObservationResult{}, ErrUsageSupersedesUnknown
			}
			return UsageObservationResult{}, fmt.Errorf("verify supersedes observation: %w", err)
		}
	}

	observationID, err := usageObservationID(identity, in.EventSeq, in.SourceVersion, digest)
	if err != nil {
		return UsageObservationResult{}, err
	}
	_, err = q.ExecContext(ctx, `INSERT INTO usage_observations (
		observation_id, request_identity, local_request_id, provider_request_id,
		event_seq, source_version, origin, usage_family, model_id,
		output_tokens, input_tokens, report_kind, finality, identity_provability,
		state, payload_digest, supersedes_observation_id, occurred_at, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		observationID, identity, in.LocalRequestID, sqlNullable(in.ProviderRequestID),
		in.EventSeq, in.SourceVersion, in.Origin, in.UsageFamily, in.ModelID,
		in.OutputTokens, in.InputTokens, in.ReportKind, in.Finality, in.IdentityProvability,
		state, digest, sqlNullable(in.SupersedesObservationID),
		canonicalTime(in.OccurredAt), canonNow())
	if err != nil {
		return UsageObservationResult{}, fmt.Errorf("persist usage observation: %w", err)
	}
	// The v2 current-row projection commits with the v1 evidence: the slot's
	// previous current row is replaced by this one, all or nothing.
	if err := UpsertUsageCurrent(ctx, q, identity, in, observationID, state, digest); err != nil {
		return UsageObservationResult{}, err
	}
	if err := appendUsageChainEvent(ctx, q, identity, in, payload); err != nil {
		return UsageObservationResult{}, err
	}
	return UsageObservationResult{ObservationID: observationID, State: state, Created: true}, nil
}

// unverifiableProviderRequestID is the registered form of an absent
// provider request ID: the fact that it is unavailable is itself a
// recorded value, never an omission and never an invention (PS §4.1④).
const unverifiableProviderRequestID = "unverifiable"

// usageObservationPayload renders the phase-two input against the B03 frozen
// field table: the required facts always present, the provider identity in
// its registered form, and the decimal lexical forms untouched.
func usageObservationPayload(in UsageObservationInput) map[string]any {
	providerID := in.ProviderRequestID
	if providerID == "" {
		providerID = unverifiableProviderRequestID
	}
	payload := map[string]any{
		"model_id":             in.ModelID,
		"output_tokens":        in.OutputTokens,
		"input_tokens":         in.InputTokens,
		"provider_request_id":  providerID,
		"local_request_id":     in.LocalRequestID,
		"origin":               in.Origin,
		"event_seq":            strconv.FormatInt(in.EventSeq, 10),
		"source_version":       strconv.FormatInt(in.SourceVersion, 10),
		"report_kind":          in.ReportKind,
		"finality":             in.Finality,
		"identity_provability": in.IdentityProvability,
		"usage_family":         in.UsageFamily,
	}
	return payload
}

// PendingUsageEvent is a raw usage event that never got its observation:
// the crash window between the two phases, recoverable at startup.
type PendingUsageEvent struct {
	LocalRequestID    string
	ProviderRequestID string
	UsageFamily       string
	EventSeq          int64
	RawJSON           string
	OccurredAt        string
}

// PendingUsageEvents lists the raw events without any observation row,
// oldest first, capped. The startup recovery re-derives their
// observations from the stored raw content.
func PendingUsageEvents(ctx context.Context, q usageWriter, limit int) ([]PendingUsageEvent, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := q.QueryContext(ctx, `SELECT e.local_request_id, e.provider_request_id, e.usage_family,
		e.event_seq, e.raw_json, e.occurred_at
	 FROM usage_events e
	 WHERE NOT EXISTS (
		 SELECT 1 FROM usage_observations o
		 WHERE o.request_identity = e.request_identity AND o.event_seq = e.event_seq)
	 ORDER BY e.recorded_at ASC, e.request_identity ASC, e.event_seq ASC
	 LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending usage events: %w", err)
	}
	defer rows.Close()
	out := []PendingUsageEvent{}
	for rows.Next() {
		var p PendingUsageEvent
		var providerRequestID sql.NullString
		if err := rows.Scan(&p.LocalRequestID, &providerRequestID, &p.UsageFamily, &p.EventSeq, &p.RawJSON, &p.OccurredAt); err != nil {
			return nil, fmt.Errorf("read pending usage event: %w", err)
		}
		p.ProviderRequestID = providerRequestID.String
		out = append(out, p)
	}
	return out, rows.Err()
}

// usageObservationID derives the row identity deterministically from its
// slot and content: a redelivered same-version claim addresses the same
// ID, which the idempotency check above then confirms.
func usageObservationID(identity string, seq, version int64, digest string) (string, error) {
	sum := sha256.Sum256([]byte("shz-usage-observation-v1|" + identity + "|" +
		strconv.FormatInt(seq, 10) + "|" + strconv.FormatInt(version, 10) + "|" + digest))
	return hex.EncodeToString(sum[:]), nil
}

// appendUsageChainEvent lands the observation's chain event on the caller's
// transaction, opening the usage aggregate first when this is the
// aggregate's first event.
func appendUsageChainEvent(ctx context.Context, q usageWriter, identity string, in UsageObservationInput, payload map[string]any) error {
	// The chain append runs on the same open transaction through the
	// auditor surface; the transaction ownership stays with the caller.
	tx := q.(auditor)
	headSeq, headDigest, err := usageChainHead(ctx, q)
	if err != nil {
		return err
	}
	if headSeq == 0 {
		openAt := canonNow()
		if err := AppendAuditEvent(tx, map[string]any{
			"serializer_id":  canonicalSerializerID,
			"event_id":       newUsageAggregateOpenEventID(),
			"aggregate_id":   usageAggregateID,
			"aggregate_type": usageAggregateID,
			"event_type":     "aggregate_open",
			"occurred_at":    openAt,
			"recorded_at":    openAt,
			"prev_hash":      GenesisHash(usageAggregateID),
			"payload_family": "aggregate_open",
			"payload":        map[string]any{"actor": "resident", "reason": "usage observations"},
		}); err != nil {
			return err
		}
		headSeq, headDigest, err = usageChainHead(ctx, q)
		if err != nil {
			return err
		}
	}
	eventID, err := NewEventID()
	if err != nil {
		return err
	}
	now := canonNow()
	return AppendAuditEvent(tx, map[string]any{
		"serializer_id":  canonicalSerializerID,
		"event_id":       eventID,
		"aggregate_id":   usageAggregateID,
		"aggregate_type": usageAggregateID,
		"event_type":     "usage.observation",
		"occurred_at":    canonicalTime(in.OccurredAt),
		"recorded_at":    now,
		"prev_hash":      headDigest,
		"correlation_id": identity,
		"payload_family": "usage_observation",
		"payload":        payload,
	})
}

// newUsageAggregateOpenEventID gives the aggregate-open event a stable
// identity: the open belongs to the aggregate, not to the opening
// observation, so the same spine always registers it under the same
// event ID.
func newUsageAggregateOpenEventID() string {
	sum := sha256.Sum256([]byte("shz-usage-aggregate-open-v1|" + usageAggregateID))
	return hex.EncodeToString(sum[:])
}

func usageChainHead(ctx context.Context, q usageWriter) (int, string, error) {
	var headSeq int
	if err := q.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(seq), 0) FROM audit_events WHERE aggregate_id = ?`,
		usageAggregateID).Scan(&headSeq); err != nil {
		return 0, "", fmt.Errorf("read usage chain head: %w", err)
	}
	if headSeq == 0 {
		return 0, "", nil
	}
	var headDigest string
	if err := q.QueryRowContext(ctx,
		`SELECT chain_digest FROM audit_events WHERE aggregate_id = ? AND seq = ?`,
		usageAggregateID, headSeq).Scan(&headDigest); err != nil {
		return 0, "", fmt.Errorf("read usage chain head digest: %w", err)
	}
	return headSeq, headDigest, nil
}
