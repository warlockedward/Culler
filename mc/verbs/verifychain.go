package verbs

import (
	"database/sql"

	"mc/substrate"
)

// VerifyChain runs the layer-1 chain verification and the shz-merkle-v1
// root over the resulting chain-head set. It is a pure read against the
// read-only spine entry. Findings are reported in the result rather than
// refused: a corrupted chain is the verification's output, and refusing
// it would discard the report the operator came to read. Only a spine
// that cannot be read, or a head set that is structurally malformed, is
// an error. The §7.1 binding field evidence_digest stays reserved to the
// verification-record process, which owns the chain-head set file this
// verb does not write.
func VerifyChain(db *sql.DB, aggregateID string) (any, error) {
	report, err := substrate.VerifyAuditChain(db, aggregateID)
	if err != nil {
		return nil, classify(err)
	}
	algorithmID, leafCount, rootHex, err := substrate.ComputeRoot(report.Heads)
	if err != nil {
		return nil, Domainf("chain-head set is malformed: %v", err)
	}
	status := "ok"
	corrupted := map[string]bool{}
	for _, e := range report.Errors {
		corrupted[e.AggregateID] = true
	}
	if len(report.Errors) > 0 {
		status = "corrupted"
	}
	aggregates := make([]map[string]any, 0, len(report.Heads))
	for _, h := range report.Heads {
		aggStatus := "ok"
		if corrupted[h.AggregateID] {
			aggStatus = "corrupted"
		}
		aggregates = append(aggregates, map[string]any{
			"aggregate_id": h.AggregateID,
			"seq_max":      h.SeqMax,
			"chain_digest": h.ChainDigest,
			"status":       aggStatus,
		})
	}
	findings := make([]map[string]any, 0, len(report.Errors))
	for _, e := range report.Errors {
		findings = append(findings, map[string]any{
			"aggregate_id": e.AggregateID,
			"seq":          e.Seq,
			"kind":         e.Kind,
			"message":      e.Message,
		})
	}
	return map[string]any{
		"status":       status,
		"algorithm_id": algorithmID,
		"leaf_count":   leafCount,
		"root_hex":     rootHex,
		"aggregates":   aggregates,
		"errors":       findings,
	}, nil
}
