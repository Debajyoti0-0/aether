package workspace

import (
	"fmt"
)

// Rekey re-encrypts every record in the workspace under a new
// passphrase (open with the old key, seal with the new key).
func (w *Workspace) Rekey(oldPassphrase, newPassphrase string) (int, error) {
	if err := ValidateName(w.Name); err != nil {
		return 0, err
	}
	if newPassphrase == "" {
		return 0, fmt.Errorf("new passphrase is required")
	}

	// Verify the old passphrase decrypts the journal (or any record).
	w.DeriveKey(oldPassphrase)

	buckets := []string{BucketTokens, BucketIdentities, BucketSessions, BucketEvidence}
	migrated := 0

	for _, bucket := range buckets {
		keys, err := w.ListRecords(bucket)
		if err != nil {
			return migrated, err
		}
		for _, key := range keys {
			sealed, err := osReadFile(w.recordPath(bucket, key))
			if err != nil {
				return migrated, err
			}
			plain, err := w.Open(sealed)
			if err != nil {
				return migrated, fmt.Errorf("%s/%s: %w (wrong passphrase?)", bucket, key, err)
			}

			// Switch to the new key and re-seal.
			w.DeriveKey(newPassphrase)
			newSealed, err := w.Seal(plain)
			if err != nil {
				return migrated, err
			}
			if err := osWriteFile(w.recordPath(bucket, key), newSealed); err != nil {
				return migrated, err
			}
			migrated++

			// Restore the old key for the next record's decryption.
			w.DeriveKey(oldPassphrase)
		}
	}

	// Re-encrypt the journal under the new key.
	events, err := w.loadEvents()
	if err != nil {
		return migrated, fmt.Errorf("journal: %w (wrong passphrase?)", err)
	}
	w.DeriveKey(newPassphrase)
	if err := w.storeEvents(events); err != nil {
		return migrated, err
	}

	// Leave the workspace object usable under the NEW passphrase.
	// (DeriveKey(newPassphrase) was already called above.)
	return migrated, nil
}
