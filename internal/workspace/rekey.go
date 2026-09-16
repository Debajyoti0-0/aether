package workspace

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Rekey re-encrypts every record and journal entry in the workspace
// under a new passphrase. Stage 2: all persistence flows through the
// vault — the fresh salt is minted first, then records are re-sealed
// one by one and the journal is atomically replaced (single bbolt
// transaction), so an interrupted rekey cannot mix keys.
//
// Stage 1 (F2) semantics preserved:
//   - the old passphrase may be empty only when the workspace is
//     keyless (KEYLESS marker) or in the legacy deterministic layout;
//   - decryption of everything is verified BEFORE anything is
//     rewritten, so a wrong old passphrase cannot corrupt the vault;
//   - migrating off keyless removes the KEYLESS marker.
func (w *Workspace) Rekey(oldPassphrase, newPassphrase string) (int, error) {
	w.rekeyMu.Lock()
	defer w.rekeyMu.Unlock()

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
	if w.vault == nil {
		return 0, fmt.Errorf("workspace %q: vault not open", w.Name)
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
	type recordRef struct{ bucket, key string }
	plaintexts := make(map[recordRef][]byte)
	migrated := 0

	for _, bucket := range buckets {
		recordKeys, err := w.ListRecords(bucket)
		if err != nil {
			return migrated, err
		}
		for _, key := range recordKeys {
			sealed, err := w.vault.GetRecord(bucket, key)
			if err != nil {
				return migrated, err
			}
			plain, err := w.Open(sealed)
			if err != nil {
				return migrated, fmt.Errorf("%s/%s: %w (wrong passphrase?)", bucket, key, err)
			}
			plaintexts[recordRef{bucket, key}] = plain
			migrated++
		}
	}
	oldEvents, err := w.Events()
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
		for ref, plain := range plaintexts {
			if ref.bucket != bucket {
				continue
			}
			resealed, err := w.Seal(plain)
			if err != nil {
				return migrated, err
			}
			if err := w.vault.PutRecord(bucket, ref.key, resealed); err != nil {
				return migrated, err
			}
		}
	}

	// Journal: re-seal every event and swap atomically.
	resealed := make([][]byte, 0, len(oldEvents))
	for _, ev := range oldEvents {
		data, err := json.Marshal(ev)
		if err != nil {
			return migrated, err
		}
		s, err := w.Seal(data)
		if err != nil {
			return migrated, err
		}
		resealed = append(resealed, s)
	}
	if err := w.vault.ReplaceJournal(resealed); err != nil {
		return migrated, err
	}
	if err := w.vault.Sync(); err != nil {
		return migrated, err
	}

	// Migration off keyless removes the marker.
	if err := w.removeKeylessMarker(); err != nil {
		return migrated, err
	}
	w.Keyless = false
	return migrated, nil
}

// recordKeysFor was removed with the vault migration: rekey iterates
// the plaintext map directly.

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
