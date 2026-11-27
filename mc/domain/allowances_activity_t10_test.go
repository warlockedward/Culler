package domain_test

import (
	"context"
	"testing"

	"mc/domain"
)

// B10 T7: the allowances' detail column was an inline SQLite json_object()
// call; the central append receives the renderer's output pre-read in the
// same transaction. The stored bytes must equal what SQLite itself renders
// for the same arguments — byte for byte, key order and spacing included.
func TestAllowanceDetailsKeepTheSQLiteRendererBytes(t *testing.T) {
	db := openSpine(t)
	// The allowance pairs condition with the submitted version, and opens on
	// the third failed verification.
	task := mkTask(t, db, "task", "seeded")
	seedVerifiedVersion(t, db, task)
	mustExec(t, db, `UPDATE tasks SET correction_count=3, correction_reason='the report',
		allowance_condition='verifier', allowance_reason='three failed verifications',
		allowance_version=submitted_version WHERE id=?`, task)
	reason := `He said "why" — 中文`
	if err := tx(t, db, func(ctx context.Context, q domain.Q) error {
		return domain.GrantCorrectionAllowance(ctx, q, task, reason)
	}); err != nil {
		t.Fatalf("grant correction allowance: %v", err)
	}
	var stored string
	if err := db.QueryRow(`SELECT detail FROM activity WHERE kind = 'task.correction_allowance'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	var want string
	if err := db.QueryRow(`SELECT json_object('reason',?,'attempts',3)`, reason).Scan(&want); err != nil {
		t.Fatal(err)
	}
	if stored != want {
		t.Fatalf("stored detail = %q; want the SQLite renderer bytes %q", stored, want)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM activity a
		JOIN activity_chain_map m ON m.activity_id = a.id WHERE a.kind = 'task.correction_allowance'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("chained allowance rows = %d; want 1", n)
	}
}
