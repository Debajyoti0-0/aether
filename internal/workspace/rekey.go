package workspace

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
)

// recordRef identifies one encrypted record during rekey.
type recordRef struct{ bucket, key string }

// Rekey re-encrypts every record in the workspace under a new
// passphrase (open with the old key, seal with the new key).
//
// Stage 1 (F2) semantics:
//   - the old passphrase may be empty only when the workspace is
//     keyless (KEYLESS marker) or in the legacy deterministic layout —
//     this is the sanctioned migration path;
//   - the workspace receives a fresh random salt on every rekey and the
//     key file (salt.bin) is rewritten with a tag under the new key;
//   - migrating off keyless removes the KEYLESS marker;
//   - decryption of every record and the journal is verified before any
//     record is rewritten, so a wrong old passphrase cannot corrupt the
//     vault.
func (w *Workspace) Rekey(oldPassphrase, newPassphrase string) (int, error) {
	if err := ValidateName(w.Name); err != nil {
		return 0, err
	}
	if newPassphrase == "" {
		return 0, fmt.Errorf("new passphrase is required")
	}
	if oldPassphrase == "" {
		keyless, err := w.hasKeylessMarker()
		if err != nil {
			return 0, err
		}
		if !keyless && !w.legacyLayout() {
			return 0, fmt.Errorf(
				"empty old passphrase is only permitted for keyless or legacy workspaces; " +
					"supply the current passphrase instead")
		}
	}

	// Determine the OLD key: stored salt, or the legacy name-derived salt.
	salt, hasSaltFile, err := w.readSaltFile()
	if err != nil {
		return 0, err
	}
	oldSalt := salt
	if !hasSaltFile {
		oldSalt = legacySalt(w.Name)
	}
	w.salt = oldSalt
	if err := w.DeriveKey(oldPassphrase); err != nil {
		return 0, err
	}

	// Phase 1 — decrypt everything under the old key BEFORE writing
	// anything, so a wrong passphrase fails without corrupting the vault.
	buckets := []string{BucketTokens, BucketIdentities, BucketSessions, BucketEvidence}
	plaintexts := make(map[recordRef][]byte)
	migrated := 0

	for _, bucket := range buckets {
		recordKeys, err := w.ListRecords(bucket)
		if err != nil {
			return migrated, err
		}
		for _, key := range recordKeys {
			data, err := osReadFile(w.recordPath(bucket, key))
			if err != nil {
				return migrated, err
			}
			plain, err := w.Open(data)
			if err != nil {
				return migrated, fmt.Errorf("%s/%s: %w (wrong passphrase?)", bucket, key, err)
			}
			plaintexts[recordRef{bucket, key}] = plain
			migrated++
		}
	}
	events, err := w.loadEvents()
	if err != nil {
		return migrated, fmt.Errorf("journal: %w (wrong passphrase?)", err)
	}

	// Phase 2 — mint a fresh random salt, write the new key file, and
	// re-encrypt everything under the new key.
	newSalt := make([]byte, saltLen)
	if _, err := rand.Read(newSalt); err != nil {
		return migrated, fmt.Errorf("generate new salt: %w", err)
	}
	w.salt = newSalt
	if err := w.DeriveKey(newPassphrase); err != nil {
		return migrated, err
	}
	if err := writeSaltFile(w.saltFilePath(), newSalt, w.pass); err != nil {
		return migrated, fmt.Errorf("write workspace key file: %w", err)
	}

	for _, bucket := range buckets {
		for _, key := range recordKeysFor(plaintexts, bucket) {
			resealed, err := w.Seal(plaintexts[recordRef{bucket, key}])
			if err != nil {
				return migrated, err
			}
			if err := osWriteFile(w.recordPath(bucket, key), resealed); err != nil {
				return migrated, err
			}
		}
	}
	if err := w.storeEvents(events); err != nil {
		return migrated, err
	}

	// Migration off keyless removes the marker.
	if err := w.removeKeylessMarker(); err != nil {
		return migrated, err
	}
	w.Keyless = false
	return migrated, nil
}

// recordKeysFor returns the record keys recorded for a bucket during
// the decrypt phase.
func recordKeysFor(plaintexts map[recordRef][]byte, bucket string) []string {
	var out []string
	for ref := range plaintexts {
		if ref.bucket == bucket {
			out = append(out, ref.key)
		}
	}
	return out
}

func (w *Workspace) hasKeylessMarker() (bool, error) {
	_, err := os.Stat(filepath.Join(w.Root, keylessMarker))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (w *Workspace) removeKeylessMarker() error {
	if err := os.Remove(filepath.Join(w.Root, keylessMarker)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (w *Workspace) legacyLayout() bool {
	_, err := os.Stat(w.saltFilePath())
	return os.IsNotExist(err)
}
