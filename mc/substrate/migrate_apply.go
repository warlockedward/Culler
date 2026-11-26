package substrate

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// ErrSchemaUnknown identifies a spine whose recorded schema identity has no
// registered migration path to this release's identity.
var ErrSchemaUnknown = errors.New("spine schema identity has no registered migration path")

// ErrMigrationChecksumConflict identifies a recorded migration step whose
// stored checksum no longer matches this release's canonical step definition.
// The ledger may have been altered after the fact; the migration stops
// fail-closed rather than proceeding over a ledger it cannot trust.
var ErrMigrationChecksumConflict = errors.New("a recorded migration step checksum conflicts with this release")

// ErrMigrationSequenceAnomaly identifies a migration ledger whose recorded
// sequence is structurally unsound (a missing version, an unknown step, an
// identity that no recorded step produced). The spine requires manual
// diagnosis; no migration step is applied.
var ErrMigrationSequenceAnomaly = errors.New("the migration step ledger is anomalous and requires manual diagnosis")

func schemaHistoryTableExists(db *sql.DB) (bool, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_history'`).Scan(&n)
	return n > 0, err
}

func readSchemaHistory(db *sql.DB) ([]recordedStep, error) {
	rows, err := db.Query(`SELECT version, step_name, step_checksum, from_schema_id, to_schema_id FROM schema_history ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("read migration ledger: %w", err)
	}
	defer rows.Close()
	out := []recordedStep{}
	for rows.Next() {
		var r recordedStep
		if err := rows.Scan(&r.Version, &r.StepName, &r.StepChecksum, &r.FromID, &r.ToID); err != nil {
			return nil, fmt.Errorf("read migration ledger row: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Migrate advances db from its recorded schema identity to targetSchemaID by
// applying the registry's steps in version order. Each step is one
// transaction: the step's statement, its ledger row, its activity event, and
// the meta.schema_id stamp commit together, so a crash between steps never
// leaves the spine stamped at an identity its ledger does not evidence.
func Migrate(db *sql.DB, registry []MigrationStep, targetSchemaID string) (MigrationReport, error) {
	report := MigrationReport{Applied: []string{}, Skipped: []string{}}
	if err := validateRegistry(registry); err != nil {
		return report, err
	}

	var deployedID string
	if err := db.QueryRow(`SELECT schema_id FROM meta WHERE id = 1`).Scan(&deployedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return report, fmt.Errorf("%w: deployment metadata is absent", ErrSchemaUnknown)
		}
		return report, fmt.Errorf("read spine schema identity: %w", err)
	}
	report.FromSchemaID = deployedID
	report.ToSchemaID = targetSchemaID
	if deployedID == targetSchemaID {
		return report, nil
	}

	ledgerExists, err := schemaHistoryTableExists(db)
	if err != nil {
		return report, err
	}

	var recorded []recordedStep
	if ledgerExists {
		recorded, err = readSchemaHistory(db)
		if err != nil {
			return report, err
		}
	}

	plan, err := planMigrationSteps(registry, recorded, ledgerExists, deployedID, targetSchemaID)
	if err != nil {
		return report, err
	}
	if len(plan) == 0 {
		// Recorded at a step's target yet stamped elsewhere: the ledger and
		// meta disagree. That is a sequence anomaly, not an empty plan.
		return report, fmt.Errorf("%w: ledger terminal identity does not match meta.schema_id", ErrMigrationSequenceAnomaly)
	}

	for _, step := range plan {
		if _, already := recordedStepByName(recorded, step.Name); already {
			// planMigrationSteps never plans a recorded step; reaching this
			// line would mean the plan ignored the ledger.
			return report, fmt.Errorf("%w: step %q is recorded but planned", ErrMigrationSequenceAnomaly, step.Name)
		}
		if _, err := applyMigrationStep(db, step); err != nil {
			return report, err
		}
		report.Applied = append(report.Applied, step.Name)
	}

	// The caller verifies the final schema state (CheckSchema): the runner
	// proves the ledger and the stamp; the deployment flow proves the whole.
	report.Migrated = true
	return report, nil
}

// applyMigrationStep executes one step atomically: step statement, ledger
// row, activity event, and the meta stamp commit or roll back as a unit.
func applyMigrationStep(db *sql.DB, step MigrationStep) (bool, error) {
	tx, err := db.Begin()
	if err != nil {
		return false, fmt.Errorf("begin migration step %s: %w", step.Name, err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(step.Statement); err != nil {
		return false, fmt.Errorf("execute migration step %s: %w", step.Name, err)
	}
	if err := verifyStep(tx, step); err != nil {
		return false, fmt.Errorf("verify migration step %s: %w", step.Name, err)
	}
	appliedAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec(
		`INSERT INTO schema_history (version, step_name, step_checksum, from_schema_id, to_schema_id, applied_at) VALUES (?, ?, ?, ?, ?, ?)`,
		step.Version, step.Name, step.Checksum, step.FromSchemaID, step.ToSchemaID, appliedAt); err != nil {
		return false, fmt.Errorf("record migration step %s: %w", step.Name, err)
	}
	detail := fmt.Sprintf("from %s to %s", step.FromSchemaID, step.ToSchemaID)
	if _, err := tx.Exec(
		`INSERT INTO activity (actor, kind, subject, detail) VALUES (?, ?, ?, ?)`,
		"mc", "schema.migration.step", step.Name, detail); err != nil {
		return false, fmt.Errorf("record migration event for step %s: %w", step.Name, err)
	}
	if err := appendMigrationAuditRow(tx, step); err != nil {
		return false, fmt.Errorf("record migration chain event for step %s: %w", step.Name, err)
	}
	if _, err := tx.Exec(`UPDATE meta SET schema_id = ? WHERE id = 1`, step.ToSchemaID); err != nil {
		return false, fmt.Errorf("stamp migrated schema identity for step %s: %w", step.Name, err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit migration step %s: %w", step.Name, err)
	}
	return true, nil
}

// appendMigrationAuditRow lands the step's chain event in the same
// transaction as the step statement, the ledger row, and the stamp: the
// schema's current value and its evidence commit together, so the spine can
// never show a migrated identity whose migration evidence is missing.
// Steps that run before the audit_events table exists (the first two on a
// legacy spine) have no chain to record into; the chain begins at the step
// that creates the table.
func appendMigrationAuditRow(tx *sql.Tx, step MigrationStep) error {
	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'audit_events'`).Scan(&exists); err != nil {
		return fmt.Errorf("probe audit chain table: %w", err)
	}
	if exists == 0 {
		return nil
	}
	// The schema aggregate's predecessor: the genesis value when this step
	// starts the aggregate's chain, the stored head's chain_digest when it
	// extends one.
	var headSeq int
	if err := tx.QueryRow(`SELECT COALESCE(MAX(seq), 0) FROM audit_events WHERE aggregate_id = 'schema'`).Scan(&headSeq); err != nil {
		return fmt.Errorf("read schema aggregate head: %w", err)
	}
	var prevHash string
	if headSeq == 0 {
		prevHash = GenesisHash("schema")
	} else {
		if err := tx.QueryRow(`SELECT chain_digest FROM audit_events WHERE aggregate_id = 'schema' AND seq = ?`, headSeq).Scan(&prevHash); err != nil {
			return fmt.Errorf("read schema aggregate predecessor: %w", err)
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
		"aggregate_id":   "schema",
		"aggregate_type": "schema",
		"event_type":     "schema.migration.step",
		"occurred_at":    now,
		"recorded_at":    now,
		"prev_hash":      prevHash,
		"payload_family": "migration",
		"payload": map[string]any{
			"step_name":      step.Name,
			"from_schema_id": step.FromSchemaID,
			"to_schema_id":   step.ToSchemaID,
			"version":        strconv.Itoa(step.Version),
		},
	})
}
