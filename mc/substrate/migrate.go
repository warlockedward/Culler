package substrate

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"sort"
)

// MigrationStep is one registered schema boundary crossing. Statement is the
// exact statement text the step executes; Checksum is derived from the step
// name and that text, never written by hand, so a later edit of the statement
// under an unchanged name changes the checksum and any spine that already
// applied the old text stops fail-closed at the conflict state.
type MigrationStep struct {
	Name         string
	Version      int
	FromSchemaID string
	ToSchemaID   string
	Checksum     string
	Statement    string
	// Verify, when set, must pass inside the step's transaction after the
	// statement ran and before the ledger row, the chain event, and the
	// stamp commit. A failure rolls the whole step back, so a spine never
	// carries state its step cannot prove. The checksum deliberately does
	// not cover it: the ledger records what was applied, not how it was
	// proven.
	Verify func(tx *sql.Tx) error
	// Migrate, when set, moves data inside the step's transaction after the
	// statement and before Verify: the same-transaction hook for steps whose
	// payload cannot be produced in SQL (chain digests). Like Verify, it is
	// release code the ledger does not checksum.
	Migrate func(tx *sql.Tx) error
}

// verifyStep runs the step's optional same-transaction proof: it must pass
// after the statement and before the ledger row, the chain event, and the
// stamp commit; a failure rolls the whole step back.
func verifyStep(tx *sql.Tx, step MigrationStep) error {
	if step.Verify == nil {
		return nil
	}
	return step.Verify(tx)
}

// runStepMigration runs the step's optional same-transaction data movement,
// positioned between the statement and the proof so the proof covers the
// moved state.
func runStepMigration(tx *sql.Tx, step MigrationStep) error {
	if step.Migrate == nil {
		return nil
	}
	return step.Migrate(tx)
}

func newMigrationStep(version int, name, fromSchemaID, toSchemaID, statement string) MigrationStep {
	sum := sha256.Sum256([]byte(name + "\n" + statement))
	return MigrationStep{
		Name:         name,
		Version:      version,
		FromSchemaID: fromSchemaID,
		ToSchemaID:   toSchemaID,
		Checksum:     fmt.Sprintf("%x", sum),
		Statement:    statement,
	}
}

// preLedgerBaselineIdentity is the schema identity of spines provisioned by
// releases before the migration ledger existed in the baseline schema.
const preLedgerBaselineIdentity = "5c56d3132ea33fb85f9204462df94396c9dfb5c4801271b9b6019ff108ba0963"

// b01BaselineIdentity is the schema identity the first step produced: the
// baseline schema with the ledger table and before the health alert body was
// renamed. It is pinned as a historical fact; a step's endpoints never follow
// CurrentSchemaID, only the last step's target does.
const b01BaselineIdentity = "362e88eaaed576fc0ef26d80403069c911f2c9b1017461d8988618d3d1cbd87a"

// b02BaselineIdentity is the schema identity the second step produced: the
// ledger-baseline schema with the renamed health alert body, before the audit
// chain table existed. Pinned as a historical fact like b01BaselineIdentity.
const b02BaselineIdentity = "eb7ebcf932c3f52895412c9aef0b3c5f38e43c06d8ef64a6e0e33f273b243bcd"

// b07BaselineIdentity is the schema identity the third step produced: the
// ledger-baseline schema with the audit chain table, before the usage
// observation tables existed. Pinned as a historical fact like
// b02BaselineIdentity.
const b07BaselineIdentity = "4a3159e3cef11986fd68eac67c0d53f7cadc6a992664b9f61b7c5a266150bf9a"

// c03UsageIdentity is the schema identity the fourth step produced: the
// usage accumulation tables, before the v2 consumption projection existed.
// Pinned as a historical fact like b07BaselineIdentity.
const c03UsageIdentity = "6ce879af3bbea92540934d467976d8a0356b5285919193dde4eff15513d3d085"

// b09ChainIdentity is the schema identity the fifth step produced: the v2
// usage projection, before the activity chain mapping table existed.
// Pinned as a historical fact like c03UsageIdentity.
const b09ChainIdentity = "93f0515ee82941d3c884a21732c9ee4afebd569b027d18b408f5f20066b87fa8"

// b08MapIdentity is the schema identity the sixth step produced: the
// activity chain mapping table, before the identity foundation existed.
// Pinned as a historical fact like b09ChainIdentity.
const b08MapIdentity = "bc32d91b58a17aaa81217c11b8dc26a24e2c355842e2ca43e2fe717327d9da0c"

// d01FoundationIdentity is the schema identity the seventh step produced:
// the identity foundation, before the legacy actor sealing existed. Pinned
// as a historical fact like b08MapIdentity.
const d01FoundationIdentity = "6be6f49be7555017684d7ce38ab08cf88fc2c7fb53f90f009670913f4751c535"

// d03SealIdentity is the schema identity the eighth step produced: the
// legacy actor sealing, before the degraded grants registry existed. Pinned
// as a historical fact like d01FoundationIdentity.
const d03SealIdentity = "bd9ebc9c6a112e38410185ee1693b9966a3f48df9a58f24518cb77d366b049a8"

// d05RegistryIdentity is the schema identity the ninth step produced: the
// degraded grants registry, before the signing request state machine
// existed. Pinned as a historical fact like d03SealIdentity.
const d05RegistryIdentity = "f9d1090e71e57130bcb7821a25a7959177098bd435c9756fcef8a13c0681e615"

// d06SigningIdentity is the schema identity the tenth step produced: the
// signing request state machine, before the anchoring registry existed.
// Pinned as a historical fact like d05RegistryIdentity.
const d06SigningIdentity = "9e322e35834ccffe95c6205252af12552b414596e7194c3a507d48456f9001a0"

// ProductionMigrationRegistry returns the ordered step registry this release
// carries. The registry always terminates at CurrentSchemaID: a release that
// changes the schema text must register the step that crosses that boundary,
// so a spine at any previously published identity can reach this release.
func ProductionMigrationRegistry() []MigrationStep {
	registry := []MigrationStep{
		newMigrationStep(1, "add-schema-history-ledger", preLedgerBaselineIdentity, b01BaselineIdentity, schemaHistoryDDL),
		newMigrationStep(2, "rename-health-alert-body", b01BaselineIdentity, b02BaselineIdentity, renameHealthAlertBodySQL),
		newMigrationStep(3, "add-audit-events-chain", b02BaselineIdentity, b07BaselineIdentity, auditEventsDDL),
		newMigrationStep(4, "add-usage-observations", b07BaselineIdentity, c03UsageIdentity, usageObservationsDDL),
	}
	last := newMigrationStep(5, "migrate-usage-to-v2", c03UsageIdentity, b09ChainIdentity, usageV2DDL)
	last.Verify = VerifyUsageBackfill
	registry = append(registry, last)
	sixth := newMigrationStep(6, "migrate-activity-to-chain", b09ChainIdentity, b08MapIdentity, activityChainMapDDL)
	sixth.Migrate = migrateActivityToChain
	sixth.Verify = VerifyActivityBackfill
	registry = append(registry, sixth)
	seventh := newMigrationStep(7, "add-identity-foundation", b08MapIdentity, d01FoundationIdentity, identityFoundationDDL)
	registry = append(registry, seventh)
	eighth := newMigrationStep(8, "add-legacy-actor-sealing", d01FoundationIdentity, d03SealIdentity, legacySealDDL)
	registry = append(registry, eighth)
	ninth := newMigrationStep(9, "add-degraded-grants", d03SealIdentity, d05RegistryIdentity, degradedGrantsDDL)
	registry = append(registry, ninth)
	tenth := newMigrationStep(10, "add-sign-requests", d05RegistryIdentity, d06SigningIdentity, signRequestsDDL)
	registry = append(registry, tenth)
	eleventh := newMigrationStep(11, "add-anchor-checkpoints", d06SigningIdentity, CurrentSchemaID, anchorCheckpointsDDL)
	return append(registry, eleventh)
}

// MigrationReport describes one runner pass over a spine.
type MigrationReport struct {
	Migrated     bool
	FromSchemaID string
	ToSchemaID   string
	Applied      []string
	Skipped      []string
}

type recordedStep struct {
	Version      int
	StepName     string
	StepChecksum string
	FromID       string
	ToID         string
}

func validateRegistry(registry []MigrationStep) error {
	if len(registry) == 0 {
		return fmt.Errorf("%w: registry is empty", ErrMigrationSequenceAnomaly)
	}
	seen := map[string]bool{}
	for i, step := range registry {
		if step.Version != i+1 {
			return fmt.Errorf("%w: registry version %d out of order at position %d", ErrMigrationSequenceAnomaly, step.Version, i+1)
		}
		if seen[step.Name] {
			return fmt.Errorf("%w: registry step %q appears twice", ErrMigrationSequenceAnomaly, step.Name)
		}
		seen[step.Name] = true
		want := sha256.Sum256([]byte(step.Name + "\n" + step.Statement))
		if fmt.Sprintf("%x", want) != step.Checksum {
			return fmt.Errorf("%w: registry step %q checksum does not match its definition", ErrMigrationSequenceAnomaly, step.Name)
		}
		if i > 0 && registry[i-1].ToSchemaID != step.FromSchemaID {
			return fmt.Errorf("%w: registry steps %q and %q do not chain", ErrMigrationSequenceAnomaly, registry[i-1].Name, step.Name)
		}
	}
	return nil
}

func recordedStepByName(rows []recordedStep, name string) (recordedStep, bool) {
	for _, r := range rows {
		if r.StepName == name {
			return r, true
		}
	}
	return recordedStep{}, false
}

// planMigrationSteps resolves the four-state semantics (DC §4):
//   - a step absent from the ledger is planned (execute);
//   - a recorded step whose checksum matches is never planned (skip);
//   - a recorded step whose checksum differs returns
//     ErrMigrationChecksumConflict (fail-closed stop);
//   - a structurally unsound ledger returns ErrMigrationSequenceAnomaly
//     (stop and diagnose).
//
// It also resolves the starting point: a spine without the ledger table is a
// pre-history baseline and must sit exactly at the first step's source
// identity; a spine with ledger rows must sit at the terminal recorded
// identity. Any other identity is unknown (ErrSchemaUnknown).
func planMigrationSteps(registry []MigrationStep, recorded []recordedStep, ledgerExists bool, deployedID, targetSchemaID string) ([]MigrationStep, error) {
	byName := map[string]MigrationStep{}
	for _, s := range registry {
		byName[s.Name] = s
	}

	var terminalID string
	if !ledgerExists {
		terminalID = registry[0].FromSchemaID
	} else {
		if len(recorded) == 0 {
			// Ledger table present but empty: a spine created by this
			// release's baseline. Its identity must already be current.
			if deployedID != targetSchemaID {
				return nil, fmt.Errorf("%w: empty ledger at identity %s", ErrSchemaUnknown, deployedID)
			}
			return nil, nil
		}
		// State 3 takes precedence: a tampered historical step stops the
		// migration before any structural reasoning.
		for _, r := range recorded {
			step, known := byName[r.StepName]
			if !known {
				continue
			}
			if r.StepChecksum != step.Checksum {
				return nil, fmt.Errorf("%w: step %q recorded %s, this release carries %s", ErrMigrationChecksumConflict, r.StepName, r.StepChecksum, step.Checksum)
			}
		}
		versions := make([]int, 0, len(recorded))
		for _, r := range recorded {
			step, known := byName[r.StepName]
			if !known {
				return nil, fmt.Errorf("%w: ledger records unknown step %q", ErrMigrationSequenceAnomaly, r.StepName)
			}
			if r.Version != step.Version {
				return nil, fmt.Errorf("%w: step %q recorded at version %d, defined at %d", ErrMigrationSequenceAnomaly, r.StepName, r.Version, step.Version)
			}
			if r.FromID != step.FromSchemaID || r.ToID != step.ToSchemaID {
				return nil, fmt.Errorf("%w: step %q records identities that this release does not define", ErrMigrationSequenceAnomaly, r.StepName)
			}
			versions = append(versions, r.Version)
		}
		sort.Ints(versions)
		for i, v := range versions {
			if v != i+1 {
				return nil, fmt.Errorf("%w: ledger versions are not a prefix of the registry (have %v)", ErrMigrationSequenceAnomaly, versions)
			}
		}
		terminalID = byName[recorded[len(recorded)-1].StepName].ToSchemaID
	}

	if deployedID != terminalID {
		return nil, fmt.Errorf("%w: meta.schema_id %s is not the identity the ledger evidences (%s)", ErrSchemaUnknown, deployedID, terminalID)
	}

	plan := []MigrationStep{}
	for _, step := range registry {
		if _, already := recordedStepByName(recorded, step.Name); already {
			continue
		}
		if len(plan) == 0 && step.FromSchemaID != deployedID {
			return nil, fmt.Errorf("%w: step %q does not start from the deployed identity", ErrSchemaUnknown, step.Name)
		}
		plan = append(plan, step)
		if step.ToSchemaID == targetSchemaID {
			return plan, nil
		}
	}
	return nil, fmt.Errorf("%w: registry does not reach %s", ErrSchemaUnknown, targetSchemaID)
}
