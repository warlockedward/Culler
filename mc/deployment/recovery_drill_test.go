package deployment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mc/substrate"
)

// secureScratchHome gives the drill tests an MC_HOME with the owner-only
// shape the host credential path requires (a plain TempDir is not 0700 here).
func secureScratchHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if err := os.Chmod(home, 0o700); err != nil {
		t.Fatal(err)
	}
	return home
}

// fakeSpineRun simulates the container arm of StageSpineCopy: it reads the
// --mount arguments, finds the volume source and the destination bind, and
// copies the spine files between them the way the container script would.
func fakeSpineRun(volumeDir string) RuntimeDocker {
	return func(_ context.Context, stdin string, args ...string) ([]byte, error) {
		var source, dest string
		for i := 0; i < len(args); i++ {
			arg := args[i]
			if arg == "--mount" && i+1 < len(args) {
				spec := args[i+1]
				for _, field := range strings.Split(spec, ",") {
					if strings.HasPrefix(field, "src=") {
						if strings.HasPrefix(spec, "type=volume") {
							source = strings.TrimPrefix(field, "src=")
						} else if strings.HasPrefix(spec, "type=bind") {
							dest = strings.TrimPrefix(field, "src=")
						}
					}
				}
			}
		}
		if source == "" || dest == "" {
			return nil, errors.New("fake run: missing mount arguments")
		}
		if _, err := os.Stat(source); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(dest, 0o700); err != nil {
			return nil, err
		}
		for _, name := range []string{"spine.db", "spine.db-wal", "spine.db-shm"} {
			body, err := os.ReadFile(filepath.Join(source, name))
			if err != nil {
				continue
			}
			if err := os.WriteFile(filepath.Join(dest, name), body, 0o600); err != nil {
				return nil, err
			}
		}
		return []byte("ok"), nil
	}
}

func buildFixtureSpine(t *testing.T) string {
	t.Helper()
	volumeDir := t.TempDir()
	spine := filepath.Join(volumeDir, "spine.db")
	db, err := substrate.Open(spine)
	if err != nil {
		t.Fatal(err)
	}
	if err := substrate.Init(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO meta (id,deployment_uuid,schema_id) VALUES (1,?,?)",
		strings.Repeat("f", 32), substrate.CurrentSchemaID); err != nil {
		t.Fatal(err)
	}
	db.Close()
	return volumeDir
}

func TestEnsureBackupKeyringMintsThenReuses(t *testing.T) {
	home := secureScratchHome(t)
	key, ring, err := EnsureBackupKeyring(home)
	if err != nil || len(key) != 32 {
		t.Fatalf("mint keyring: %v len=%d", err, len(key))
	}
	if ring.Version != 1 || ring.KeyID != backupKeyID || len(ring.Reference) != 64 || len(ring.Fingerprint) != 64 {
		t.Fatalf("ring shape = %+v", ring)
	}
	info, err := os.Stat(filepath.Join(home, "backups", "keyring.json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("keyring.json mode = %v err %v", info, err)
	}
	key2, ring2, err := EnsureBackupKeyring(home)
	if err != nil || ring2.Reference != ring.Reference || !bytes.Equal(key, key2) {
		t.Fatalf("second call re-keyed: %v %q vs %q", err, ring2.Reference, ring.Reference)
	}
}

func TestEnsureBackupKeyringRefusesFingerprintDrift(t *testing.T) {
	home := secureScratchHome(t)
	if _, _, err := EnsureBackupKeyring(home); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "backups", "keyring.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var ring backupKeyring
	if err := json.Unmarshal(raw, &ring); err != nil {
		t.Fatal(err)
	}
	ring.Fingerprint = strings.Repeat("0", 64)
	body, _ := json.Marshal(ring)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := EnsureBackupKeyring(home); !errors.Is(err, errBackupKeyringCorrupt) {
		t.Fatalf("fingerprint drift accepted: %v", err)
	}
}

func TestEnsureBackupKeyringRefusesSymlinkedRecord(t *testing.T) {
	home := secureScratchHome(t)
	if _, _, err := EnsureBackupKeyring(home); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "backups", "keyring.json")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(home, "backups"), path); err != nil {
		t.Fatal(err)
	}
	if _, _, err := EnsureBackupKeyring(home); err == nil {
		t.Fatal("symlinked keyring record accepted")
	}
}

func TestSHB1RoundTripAndTamperRefusal(t *testing.T) {
	home := secureScratchHome(t)
	key, ring, err := EnsureBackupKeyring(home)
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("spine bytes for the drill")
	sealed, err := sealSHB1(plain, key, ring.Reference)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := openSHB1(sealed, home)
	if err != nil || !bytes.Equal(opened, plain) {
		t.Fatalf("round trip: %v", err)
	}
	tampered := append([]byte(nil), sealed...)
	tampered[len(tampered)-1] ^= 0x01
	if _, err := openSHB1(tampered, home); err == nil {
		t.Fatal("tampered ciphertext decrypted")
	}
	if _, err := openSHB1(plain, home); err == nil {
		t.Fatal("non-SHB1 input accepted")
	}
}

func TestRunRecoveryDrillFullCycle(t *testing.T) {
	home := secureScratchHome(t)
	volumeDir := buildFixtureSpine(t)
	fixtureBefore, err := os.ReadFile(filepath.Join(volumeDir, "spine.db"))
	if err != nil {
		t.Fatal(err)
	}

	result, err := RunRecoveryDrill(context.Background(), home, "mc-prod:latest", volumeDir, "test-release", "29.4.0", fakeSpineRun(volumeDir))
	if err != nil {
		t.Fatalf("drill: %v (result=%+v)", err, result)
	}
	if result.Verdict != "PASS" {
		t.Fatalf("verdict = %s: %+v", result.Verdict, result)
	}
	if result.Identity.UUID != strings.Repeat("f", 32) || result.Identity.Tables == 0 {
		t.Fatalf("identity = %+v", result.Identity)
	}
	if result.Integrity != "ok" || result.SpineDigest == "" {
		t.Fatalf("integrity=%s digest=%q", result.Integrity, result.SpineDigest)
	}

	body, err := os.ReadFile(result.RecordPath)
	if err != nil {
		t.Fatalf("record missing: %v", err)
	}
	var record map[string]string
	if err := json.Unmarshal(body, &record); err != nil {
		t.Fatal(err)
	}
	// TM 7.1: every required field present, none dropped.
	for _, field := range []string{
		"verification_id", "target_component", "tested_version", "environment",
		"attack_scenario", "expected_result", "actual_result", "evidence_ref",
		"evidence_digest", "test_script_digest", "execution_id",
		"supersedes_verification_id", "verdict", "reviewed_by", "verified_at", "z5_replication",
	} {
		if record[field] == "" && field != "supersedes_verification_id" {
			t.Errorf("record field %q empty", field)
		}
		if _, present := record[field]; !present {
			t.Errorf("record field %q missing", field)
		}
	}
	if record["evidence_digest"] != result.SpineDigest {
		t.Fatalf("evidence_digest %q != spine digest %q", record["evidence_digest"], result.SpineDigest)
	}

	// The fixture volume is never written by the drill.
	fixtureAfter, err := os.ReadFile(filepath.Join(volumeDir, "spine.db"))
	if err != nil || !bytes.Equal(fixtureBefore, fixtureAfter) {
		t.Fatal("the drill touched the production volume fixture")
	}
	if _, err := os.Stat(filepath.Join(home, "backups", "encrypted")); err != nil {
		t.Fatalf("encrypted backup dir missing")
	}
}
