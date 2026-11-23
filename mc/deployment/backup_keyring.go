package deployment

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// The backup encryption keyring keeps one backup encryption key per
// installation. The key material lives in the host credential store (the same
// backend every other host secret uses), and this file records the ring's
// identity metadata: which reference holds the key and its fingerprint, so a
// restore can prove which key a backup was made with before trusting it.

const backupKeyID = "backup-key"

const backupKeyringPathName = "keyring.json"

type backupKeyring struct {
	Version     int    `json:"version"`
	KeyID       string `json:"key_id"`
	Reference   string `json:"reference"`
	Fingerprint string `json:"fingerprint"`
	CreatedAt   string `json:"created_at"`
}

var errBackupKeyringCorrupt = errors.New("the backup keyring is corrupt or was altered; refusing to continue with an untrusted key")

func backupKeyringPath(home string) (string, error) {
	canonical, err := CanonicalHome(home)
	if err != nil {
		return "", err
	}
	return filepath.Join(canonical, "backups", backupKeyringPathName), nil
}

// EnsureBackupKeyring returns the installation's backup encryption key. When
// no keyring exists yet it mints a 256-bit key into the host credential store
// and records the keyring atomically. When one exists it verifies the stored
// reference still resolves and the material still matches the recorded
// fingerprint; any drift is a refusal, never a silent re-key.
func EnsureBackupKeyring(home string) ([]byte, backupKeyring, error) {
	path, err := backupKeyringPath(home)
	if err != nil {
		return nil, backupKeyring{}, err
	}
	if body, statErr := os.Lstat(path); statErr == nil {
		if err := requireSecureBackupFile(path, body); err != nil {
			return nil, backupKeyring{}, err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, backupKeyring{}, err
		}
		var ring backupKeyring
		if err := json.Unmarshal(raw, &ring); err != nil ||
			ring.Version != 1 || ring.KeyID != backupKeyID ||
			!isLowerHex64(ring.Reference) || !isLowerHex64(ring.Fingerprint) || ring.CreatedAt == "" {
			return nil, backupKeyring{}, errBackupKeyringCorrupt
		}
		key, err := ReadHostSecret(home, backupKeyID, ring.Reference)
		if err != nil {
			return nil, backupKeyring{}, errBackupKeyringCorrupt
		}
		if sha256.Sum256(key) != decodeHex(ring.Fingerprint) {
			clear(key)
			return nil, backupKeyring{}, errBackupKeyringCorrupt
		}
		return key, ring, nil
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		return nil, backupKeyring{}, fmt.Errorf("read the backup keyring: %w", err)
	}

	backupDir := filepath.Dir(path)
	if _, statErr := os.Lstat(backupDir); statErr != nil {
		if !errors.Is(statErr, os.ErrNotExist) {
			return nil, backupKeyring{}, statErr
		}
		if err := os.Mkdir(backupDir, 0o700); err != nil {
			return nil, backupKeyring{}, err
		}
	} else if err := requireOwnerOnlyDirectory(backupDir, "backup keyring directory"); err != nil {
		return nil, backupKeyring{}, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, backupKeyring{}, err
	}
	reference, err := WriteHostSecret(home, backupKeyID, key)
	if err != nil {
		clear(key)
		return nil, backupKeyring{}, err
	}
	sum := sha256.Sum256(key)
	ring := backupKeyring{
		Version:     1,
		KeyID:       backupKeyID,
		Reference:   reference,
		Fingerprint: hex.EncodeToString(sum[:]),
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	body, err := json.MarshalIndent(ring, "", "  ")
	if err != nil {
		clear(key)
		_ = DeleteHostSecret(home, backupKeyID, reference)
		return nil, backupKeyring{}, err
	}
	if err := writeFileExclusiveSync(path, append(body, '\n'), 0o600); err != nil {
		clear(key)
		_ = DeleteHostSecret(home, backupKeyID, reference)
		return nil, backupKeyring{}, err
	}
	return key, ring, nil
}

func isLowerHex64(v string) bool {
	if len(v) != 64 {
		return false
	}
	_, err := hex.DecodeString(v)
	return err == nil
}

// decodeHex returns the decoded bytes of a 64-character hex string, or a zero
// digest when the string is malformed, so a corrupt fingerprint fails the
// equality check instead of panicking.
func decodeHex(v string) [32]byte {
	var out [32]byte
	decoded, err := hex.DecodeString(v)
	if err != nil || len(decoded) != 32 {
		return out
	}
	copy(out[:], decoded)
	return out
}

// requireSecureBackupFile applies the same file-level trust checks the
// plaintext snapshot path uses: regular file, owner-only, single hard link.
func requireSecureBackupFile(path string, info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() ||
		info.Mode().Perm() != 0o600 || stat.Uid != uint32(os.Getuid()) || stat.Nlink != 1 {
		return fmt.Errorf("the backup keyring file has an untrusted shape")
	}
	return nil
}

// writeFileExclusiveSync writes through a temp name and renames, syncing both,
// so a crash cannot leave a half-written keyring record.
func writeFileExclusiveSync(path string, body []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".keyring-next-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
