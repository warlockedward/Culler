package verbs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"mc/domain"
	"mc/substrate"
)

var usageUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var usageHex64Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var usageTokenPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)
var usageFamilyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// The closed usage family set: the four provider families with frozen
// per-family generation rules plus the unknown family, whose events only
// ever produce undecidable interim observations.
var usageFamilies = map[string]bool{
	"openai-chat":      true,
	"openai-responses": true,
	"anthropic":        true,
	"google":           true,
	"unknown":          true,
}

// UsageEventRequest is the phase-one raw usage event on the wire.
type UsageEventRequest struct {
	LocalRequestID    string          `json:"local_request_id"`
	ProviderRequestID string          `json:"provider_request_id,omitempty"`
	NativeEventID     string          `json:"native_event_id,omitempty"`
	UsageFamily       string          `json:"usage_family"`
	Raw               json.RawMessage `json:"raw"`
	OccurredAt        string          `json:"occurred_at,omitempty"`
}

// UsageObservationRequest is the phase-two derived usage fact on the wire.
// Token counts carry their decimal lexical form; the form survives to the
// digest input.
type UsageObservationRequest struct {
	LocalRequestID          string `json:"local_request_id"`
	ProviderRequestID       string `json:"provider_request_id,omitempty"`
	EventSeq                int64  `json:"event_seq"`
	SourceVersion           int64  `json:"source_version"`
	Origin                  string `json:"origin"`
	UsageFamily             string `json:"usage_family"`
	ModelID                 string `json:"model_id"`
	OutputTokens            string `json:"output_tokens"`
	InputTokens             string `json:"input_tokens"`
	ReportKind              string `json:"report_kind"`
	Finality                string `json:"finality"`
	IdentityProvability     string `json:"identity_provability"`
	SupersedesObservationID string `json:"supersedes_observation_id,omitempty"`
	OccurredAt              string `json:"occurred_at,omitempty"`
}

// UsageRecordEvent persists the raw provider usage event. A redelivered
// input addressable through a native event ID or cursor re-enters its
// existing row; created=false reports that.
func UsageRecordEvent(db *sql.DB, id *RunIdentity, a UsageEventRequest) (any, error) {
	if err := RequireHostScope(id, "usage event recording"); err != nil {
		return nil, err
	}
	if !usageUUIDPattern.MatchString(a.LocalRequestID) {
		return nil, Usagef("The usage local request identity is invalid.")
	}
	if !usageFamilies[a.UsageFamily] {
		return nil, Usagef("The usage family is invalid.")
	}
	if len(a.Raw) == 0 || !json.Valid(a.Raw) {
		return nil, Usagef("The raw usage event must be a JSON object.")
	}
	if a.ProviderRequestID != "" && !providerReferencePattern.MatchString(a.ProviderRequestID) {
		return nil, Usagef("The provider request identity is invalid.")
	}
	if a.NativeEventID != "" && !providerReferencePattern.MatchString(a.NativeEventID) {
		return nil, Usagef("The native usage event identity is invalid.")
	}
	occurred, err := usageOccurredAt(a.OccurredAt)
	if err != nil {
		return nil, err
	}
	var result substrate.UsageEventResult
	if err := inTx(db, func(ctx context.Context, q Q) error {
		var err error
		result, err = substrate.RecordUsageEvent(ctx, q, substrate.UsageEventInput{
			LocalRequestID:    a.LocalRequestID,
			ProviderRequestID: a.ProviderRequestID,
			NativeEventID:     a.NativeEventID,
			UsageFamily:       a.UsageFamily,
			RawJSON:           string(a.Raw),
			OccurredAt:        occurred,
		})
		return err
	}); err != nil {
		return nil, err
	}
	return map[string]any{
		"request_identity": result.RequestIdentity,
		"event_seq":        result.EventSeq,
		"created":          result.Created,
	}, nil
}

// UsageRecordObservation persists the derived usage fact and its chain
// event as one transaction: the fact and its audit evidence commit
// together or not at all.
func UsageRecordObservation(db *sql.DB, id *RunIdentity, a UsageObservationRequest) (any, error) {
	if err := RequireHostScope(id, "usage observation recording"); err != nil {
		return nil, err
	}
	if !usageUUIDPattern.MatchString(a.LocalRequestID) {
		return nil, Usagef("The usage local request identity is invalid.")
	}
	if !usageFamilies[a.UsageFamily] {
		return nil, Usagef("The usage family is invalid.")
	}
	if !usageTokenPattern.MatchString(a.OutputTokens) || !usageTokenPattern.MatchString(a.InputTokens) {
		return nil, Usagef("The usage token counts must be non-negative integers.")
	}
	if !providerReferencePattern.MatchString(a.ModelID) {
		return nil, Usagef("The usage model identity is invalid.")
	}
	switch a.Origin {
	case "provider", "local":
	default:
		return nil, Usagef("The usage observation origin is invalid.")
	}
	switch a.ReportKind {
	case "cumulative", "incremental":
	default:
		return nil, Usagef("The usage observation report kind is invalid.")
	}
	switch a.Finality {
	case "interim", "final":
	default:
		return nil, Usagef("The usage observation finality is invalid.")
	}
	switch a.IdentityProvability {
	case "native", "cursor", "none":
	default:
		return nil, Usagef("The usage observation identity provability is invalid.")
	}
	if a.ProviderRequestID != "" && !providerReferencePattern.MatchString(a.ProviderRequestID) {
		return nil, Usagef("The provider request identity is invalid.")
	}
	if a.EventSeq < 1 || a.SourceVersion < 1 {
		return nil, Usagef("The usage observation sequence and version are invalid.")
	}
	if a.SupersedesObservationID != "" && !usageHex64Pattern.MatchString(a.SupersedesObservationID) {
		return nil, Usagef("The usage observation supersedes reference is invalid.")
	}
	occurred, err := usageOccurredAt(a.OccurredAt)
	if err != nil {
		return nil, err
	}
	var result substrate.UsageObservationResult
	if err := inTx(db, func(ctx context.Context, q Q) error {
		var err error
		result, err = substrate.RecordUsageObservation(ctx, q, substrate.UsageObservationInput{
			LocalRequestID:          a.LocalRequestID,
			ProviderRequestID:       a.ProviderRequestID,
			EventSeq:                a.EventSeq,
			SourceVersion:           a.SourceVersion,
			Origin:                  a.Origin,
			UsageFamily:             a.UsageFamily,
			ModelID:                 a.ModelID,
			OutputTokens:            a.OutputTokens,
			InputTokens:             a.InputTokens,
			ReportKind:              a.ReportKind,
			Finality:                a.Finality,
			IdentityProvability:     a.IdentityProvability,
			SupersedesObservationID: a.SupersedesObservationID,
			OccurredAt:              occurred,
		})
		return err
	}); err != nil {
		return nil, classifyUsageError(err)
	}
	return map[string]any{
		"observation_id": result.ObservationID,
		"state":          result.State,
		"created":        result.Created,
	}, nil
}

// UsagePendingRequest asks for the raw events that never got their
// observation (the startup recovery input).
type UsagePendingRequest struct {
	Limit int `json:"limit,omitempty"`
}

// UsagePending lists the recoverable raw usage events. It is read-only by
// construction: it changes no state and appends no chain event.
func UsagePending(db *sql.DB, id *RunIdentity, a UsagePendingRequest) (any, error) {
	if err := RequireHostScope(id, "usage recovery listing"); err != nil {
		return nil, err
	}
	if a.Limit <= 0 || a.Limit > 1000 {
		a.Limit = 200
	}
	pending, err := substrate.PendingUsageEvents(context.Background(), db, a.Limit)
	if err != nil {
		return nil, classify(err)
	}
	out := make([]map[string]any, 0, len(pending))
	for _, p := range pending {
		var raw any
		if err := json.Unmarshal([]byte(p.RawJSON), &raw); err != nil {
			return nil, Domainf("The stored raw usage event is unreadable.")
		}
		entry := map[string]any{
			"local_request_id": p.LocalRequestID,
			"usage_family":     p.UsageFamily,
			"event_seq":        p.EventSeq,
			"raw":              raw,
			"occurred_at":      p.OccurredAt,
		}
		if p.ProviderRequestID != "" {
			entry["provider_request_id"] = p.ProviderRequestID
		}
		out = append(out, entry)
	}
	return map[string]any{"pending": out}, nil
}

// usageOccurredAt normalizes the wire time to the production clock: an
// absent time is the spine's own now, a present one must parse.
func usageOccurredAt(value string) (time.Time, error) {
	if value == "" {
		return time.Now().UTC(), nil
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, Usagef("The usage event time is invalid.")
	}
	return t, nil
}

// classifyUsageError keeps the substrate's stable rejections coded: an
// unknown raw event is not-found, a version conflict and an origin claim
// are domain rejections on their own.
func classifyUsageError(err error) error {
	switch {
	case errors.Is(err, substrate.ErrUsageEventUnknown):
		return domain.Errf(domain.CodeNotFound, "The usage observation references a raw usage event that was never persisted.")
	case errors.Is(err, substrate.ErrUsageVersionCollision):
		return domain.Errf("observation-version-collision", "A usage observation version already carries different content.")
	case errors.Is(err, substrate.ErrUsageOriginClaim):
		return domain.Errf("origin-claim", "The usage observation origin does not match its request identity.")
	case errors.Is(err, substrate.ErrUsageSupersedesUnknown):
		return domain.Errf(domain.CodeNotFound, "The usage observation supersedes an unknown observation.")
	}
	return err
}
