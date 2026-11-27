package verbs

import (
	"testing"

	"mc/substrate"
)

// B10 T5: the dispatch seam's commit-side receipts chain through the
// central append. The receipt JSON itself is the frozen consequence body —
// the rewiring passes it through unchanged (the zero-rewrite redline), and
// the chain evidence now lands in the same commit transaction.
func TestDispatchSeamReceiptsLandInTheActivityChain(t *testing.T) {
	f, task, run, application := pendingDispatchApplication(t)
	result := f.execute(RepositoryEnvelope{
		Operation: "apply", TaskID: task, RunID: run, WorkspaceRoot: f.workspace,
		DestinationBranch: "culler", Application: &application,
	})
	if result.Applied == nil {
		t.Fatalf("the actual merge failed: %+v", result)
	}
	if n := dfInt(t, f.db, `SELECT COUNT(*) FROM activity WHERE kind LIKE 'dispatch.%' OR kind LIKE 'task.%'`); n == 0 {
		t.Fatal("the seam committed no dispatch or task activity row; the flow under test changed")
	}
	// The B10 invariant on a spine built by this release: every activity row
	// is chained and mapped — nothing writes outside the central append.
	if n := dfInt(t, f.db, `SELECT COUNT(*) FROM activity a WHERE NOT EXISTS (
		SELECT 1 FROM activity_chain_map m WHERE m.activity_id = a.id)`); n != 0 {
		t.Fatalf("%d activity rows are unmapped; the central append must chain every row", n)
	}
	report, err := substrate.VerifyAuditChain(f.db, "activity")
	if err != nil {
		t.Fatalf("verify the activity chain: %v", err)
	}
	if len(report.Errors) != 0 {
		t.Fatalf("the seam receipts broke the activity chain: %v", report.Errors)
	}
	// A receipt body is valid stored JSON (the storable form is unchanged:
	// a rewritten encoder would corrupt this parse).
	var body string
	err = f.db.QueryRow(`SELECT dispatch_result FROM activity WHERE dispatch_result IS NOT NULL
		ORDER BY id DESC LIMIT 1`).Scan(&body)
	if err == nil {
		if len(body) == 0 || body[0] != '{' {
			t.Fatalf("receipt body is not the stored JSON object: %q", body)
		}
	} else if n := dfInt(t, f.db, `SELECT COUNT(*) FROM activity WHERE dispatch_result IS NOT NULL`); n != 0 {
		t.Fatalf("receipt rows exist but none scanned: %v", err)
	}
}
