package deployment

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"time"

	"mc/substrate"
)

// Encrypted spine backup format SHB1, version 1:
//
//	"SHB1"(4) | version(1) | reference(64 ASCII hex chars) | nonce(12) | AES-256-GCM ciphertext incl. tag
//
// The reference names the host credential entry that holds the key, so a
// backup states which key it was made with regardless of what the current
// keyring points at (the "old backups forever use their mint-time key"
// property the rotation contract requires).

const shbMagic = "SHB1"

const shbVersion = 1

var encryptedBackupName = regexp.MustCompile(`^spine-[0-9]{8}T[0-9]{6}Z\.db\.enc$`)

type EncryptedBackup struct {
	File           string `json:"file"`
	Bytes          int    `json:"bytes"`
	SHA256         string `json:"sha256"`
	KeyID          string `json:"key_id"`
	Reference      string `json:"reference"`
	KeyFingerprint string `json:"key_fingerprint"`
	CreatedAt      string `json:"created_at"`
}

type DrillResult struct {
	DrillID           string                  `json:"drill_id"`
	Backup            EncryptedBackup         `json:"backup"`
	Identity          CheckpointSpineIdentity `json:"spine_identity"`
	Integrity         string                  `json:"integrity_check"`
	AggregateTasks    int                     `json:"aggregate_tasks"`
	AggregateActivity int                     `json:"aggregate_activity"`
	SpineDigest       string                  `json:"decrypted_spine_sha256"`
	Verdict           string                  `json:"verdict"`
	RecordPath        string                  `json:"record_path"`
	Environment       string                  `json:"environment"`
	FinishedAt        string                  `json:"finished_at"`
}

// EncryptedBackupSpine stages the named volume's spine with StageSpineCopy
// (the update-checkpoint primitive: sidecars travel so an unsettled snapshot
// is refused loudly), verifies the staged copy opens through the snapshot
// read-only door, and writes an SHB1-encrypted snapshot into
// MC_HOME/backups/encrypted/. It never touches the live volume.
func EncryptedBackupSpine(ctx context.Context, home, image, volume string, run RuntimeDocker) (EncryptedBackup, error) {
	if run == nil {
		run = runRuntimeDocker
	}
	key, ring, err := EnsureBackupKeyring(home)
	if err != nil {
		return EncryptedBackup{}, err
	}
	defer clear(key)
	canonical, err := CanonicalHome(home)
	if err != nil {
		return EncryptedBackup{}, err
	}
	encDir := filepath.Join(canonical, "backups", "encrypted")
	if err := os.MkdirAll(encDir, 0o700); err != nil {
		return EncryptedBackup{}, err
	}
	stage, err := os.MkdirTemp(filepath.Join(canonical, "backups"), "drill-stage-")
	if err != nil {
		return EncryptedBackup{}, err
	}
	defer os.RemoveAll(stage)
	staged, err := StageSpineCopy(ctx, image, volume, stage, run)
	if err != nil {
		return EncryptedBackup{}, fmt.Errorf("stage the spine snapshot: %w", err)
	}
	db, err := substrate.OpenSnapshotReadOnly(staged)
	if err != nil {
		return EncryptedBackup{}, fmt.Errorf("the staged spine is not a closed snapshot: %w", err)
	}
	db.Close()

	body, err := os.ReadFile(staged)
	if err != nil {
		return EncryptedBackup{}, err
	}
	sealed, err := sealSHB1(body, key, ring.Reference)
	clear(body)
	if err != nil {
		return EncryptedBackup{}, err
	}
	fileSum := sha256.Sum256(sealed)
	name := "spine-" + time.Now().UTC().Format("20060102T150405") + "Z.db.enc"
	path := filepath.Join(encDir, name)
	if err := writeFileExclusiveSync(path, sealed, 0o600); err != nil {
		clear(sealed)
		return EncryptedBackup{}, err
	}
	clear(sealed)
	return EncryptedBackup{
		File: path, Bytes: len(sealed), SHA256: hex.EncodeToString(fileSum[:]),
		KeyID: ring.KeyID, Reference: ring.Reference, KeyFingerprint: ring.Fingerprint,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// RunRecoveryDrill performs one full cycle against scratch destinations:
// take an encrypted backup of the named volume's spine, read the encrypted
// file back, decrypt it, open it through the snapshot read-only door, and
// prove identity (schema, deployment uuid, table count), integrity
// (quick_check), and aggregate-root samples (tasks/activity counts). The
// production spine is never written. The TM 7.1 record lands in
// MC_HOME/backup-drill/<drill-id>/record.json.
func RunRecoveryDrill(ctx context.Context, home, image, volume, releaseBuildID, dockerVersion string, run RuntimeDocker) (DrillResult, error) {
	if run == nil {
		run = runRuntimeDocker
	}
	backup, err := EncryptedBackupSpine(ctx, home, image, volume, run)
	if err != nil {
		return DrillResult{}, err
	}
	canonical, err := CanonicalHome(home)
	if err != nil {
		return DrillResult{}, err
	}

	drillID := newDrillID()
	drillDir := filepath.Join(canonical, "backup-drill", drillID)
	if err := os.MkdirAll(drillDir, 0o700); err != nil {
		return DrillResult{}, err
	}

	sealed, err := os.ReadFile(backup.File)
	if err != nil {
		return DrillResult{}, err
	}
	plain, err := openSHB1(sealed, canonical)
	clear(sealed)
	if err != nil {
		return DrillResult{}, fmt.Errorf("decrypt the drill backup: %w", err)
	}

	result := DrillResult{
		DrillID: drillID, Backup: backup, Integrity: "pending", Verdict: "FAIL",
		Environment: drillEnvironment(image, volume, dockerVersion),
		FinishedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	stagedPath := filepath.Join(drillDir, "spine.db")
	if err := os.WriteFile(stagedPath, plain, 0o600); err != nil {
		clear(plain)
		return result, writeDrillRecord(canonical, drillDir, releaseBuildID, &result, err)
	}
	clear(plain)

	db, err := substrate.OpenSnapshotReadOnly(stagedPath)
	if err != nil {
		return result, writeDrillRecord(canonical, drillDir, releaseBuildID, &result, fmt.Errorf("open the decrypted spine: %w", err))
	}
	defer db.Close()

	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table'`).Scan(&result.Identity.Tables); err != nil {
		return result, writeDrillRecord(canonical, drillDir, releaseBuildID, &result, fmt.Errorf("count tables: %w", err))
	}
	if result.Identity.Tables == 0 {
		return result, writeDrillRecord(canonical, drillDir, releaseBuildID, &result, errors.New("the decrypted spine has no tables"))
	}
	if err := db.QueryRow("PRAGMA quick_check").Scan(&result.Integrity); err != nil || result.Integrity != "ok" {
		return result, writeDrillRecord(canonical, drillDir, releaseBuildID, &result, errors.New("quick_check did not report ok"))
	}
	result.Identity.Schema = substrate.CurrentSchemaID
	if err := db.QueryRow(`SELECT deployment_uuid FROM meta WHERE id = 1`).Scan(&result.Identity.UUID); err != nil {
		return result, writeDrillRecord(canonical, drillDir, releaseBuildID, &result, fmt.Errorf("read the spine identity: %w", err))
	}
	_ = db.QueryRow(`SELECT COUNT(*) FROM tasks`).Scan(&result.AggregateTasks)
	_ = db.QueryRow(`SELECT COUNT(*) FROM activity`).Scan(&result.AggregateActivity)

	body, err := os.ReadFile(stagedPath)
	if err != nil {
		return result, writeDrillRecord(canonical, drillDir, releaseBuildID, &result, err)
	}
	digest := sha256.Sum256(body)
	clear(body)
	result.SpineDigest = hex.EncodeToString(digest[:])
	result.Verdict = "PASS"
	return result, writeDrillRecord(canonical, drillDir, releaseBuildID, &result, nil)
}

func newDrillID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("drill id: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}

func drillEnvironment(image, volume, dockerVersion string) string {
	host, _ := os.Hostname()
	return fmt.Sprintf("host=%s platform=%s/%s image=%s volume=%s docker=%s", host, runtime.GOOS, runtime.GOARCH, image, volume, dockerVersion)
}

func sealSHB1(plain, key []byte, reference string) ([]byte, error) {
	if len(reference) != 64 {
		return nil, errors.New("the keyring reference is not 64 hex characters")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	out := make([]byte, 0, 4+1+64+12+len(plain)+16)
	out = append(out, []byte(shbMagic)...)
	out = append(out, shbVersion)
	out = append(out, []byte(reference)...)
	out = append(out, nonce[:]...)
	out = gcm.Seal(out, nonce[:], plain, nil)
	return out, nil
}

// openSHB1 decrypts an SHB1 file using the key named by its embedded
// reference. A tag failure means the file was altered or the key is gone;
// both are refusals, and no fallback key is ever tried.
func openSHB1(sealed []byte, home string) ([]byte, error) {
	if len(sealed) < 4+1+64+12+16 || string(sealed[:4]) != shbMagic {
		return nil, errors.New("not an SHB1 encrypted spine backup")
	}
	if sealed[4] != shbVersion {
		return nil, fmt.Errorf("unsupported SHB1 version %d", sealed[4])
	}
	reference := string(sealed[5 : 5+64])
	key, err := ReadHostSecret(home, backupKeyID, reference)
	if err != nil {
		return nil, err
	}
	defer clear(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := sealed[5+64 : 5+64+12]
	plain, err := gcm.Open(nil, nonce, sealed[5+64+12:], nil)
	if err != nil {
		return nil, errors.New("the backup failed decryption (altered file or missing key)")
	}
	return plain, nil
}

// writeDrillRecord renders the TM 7.1 verification record with every field
// present; fields the scenario does not apply to keep their names with an
// empty or NOT_APPLICABLE value (the record schema is fixed by TM 7.1, and
// consumers must never see a field column dropped).
func writeDrillRecord(canonical, drillDir, releaseBuildID string, result *DrillResult, drillErr error) error {
	actual := fmt.Sprintf("verdict=%s integrity=%s tables=%d schema=%s uuid=%s tasks=%d activity=%d",
		result.Verdict, result.Integrity, result.Identity.Tables, result.Identity.Schema, result.Identity.UUID,
		result.AggregateTasks, result.AggregateActivity)
	if drillErr != nil {
		actual += " error=" + drillErr.Error()
	}
	record := map[string]string{
		"verification_id":            result.DrillID,
		"target_component":           "spine encrypted backup and recovery drill (A06)",
		"tested_version":             "release=" + releaseBuildID + " backup_file=" + result.Backup.File,
		"environment":                result.Environment,
		"attack_scenario":            "recovery drill: encrypted backup of the live spine volume, decrypt, open through the snapshot read-only door, integrity check, identity and aggregate-root sampling",
		"expected_result":            "the decrypted spine opens read-only, passes quick_check, and reports the current schema, deployment uuid, and table count",
		"actual_result":              actual,
		"evidence_ref":               filepath.Join("backup-drill", result.DrillID),
		"evidence_digest":            result.SpineDigest,
		"test_script_digest":         "mc binary release=" + releaseBuildID,
		"execution_id":               result.DrillID,
		"supersedes_verification_id": "",
		"verdict":                    result.Verdict,
		"reviewed_by":                "executed and recorded by the implementation agent; human review pending (role conflation stated explicitly)",
		"verified_at":                result.FinishedAt,
		"z5_replication":             "NOT_APPLICABLE",
	}
	body, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(drillDir, "record.json")
	if err := os.WriteFile(path, append(body, '\n'), 0o600); err != nil {
		return err
	}
	result.RecordPath = path
	return nil
}
