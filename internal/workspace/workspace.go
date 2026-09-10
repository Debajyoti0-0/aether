package workspace

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Debajyoti0-0/aether/internal/engine/rollback"
	"github.com/Debajyoti0-0/aether/internal/paths"
	"github.com/Debajyoti0-0/aether/internal/store"
	"golang.org/x/crypto/argon2"
)

// Bucket layout inside a workspace (KV namespaces stored as directories).
var (
	BucketIdentities = "identities"
	BucketTokens     = "tokens"
	BucketSessions   = "sessions"
	BucketEvidence   = "evidence"
	BucketEvents     = "events"
)

// Key material layout (Stage 1, forensic F2):
//
//   - Every workspace has a random 16-byte Argon2id salt, persisted at
//     <root>/salt.bin as: version(2 BE) || salt(16) || tag(14), where tag
//     is HMAC-SHA256(key, version||salt) truncated to 14 bytes and keyed
//     with the derived key. This makes the salt tamper-evident for
//     anyone who does not know the passphrase (a swapped salt fails the
//     tag check and the GCM record authentication).
//   - A workspace created with an empty passphrase writes a KEYLESS
//     marker file. Open rejects empty passphrases unless that marker
//     exists (explicit opt-in) and warns loudly on every keyless open.
//   - Workspaces created before Stage 1 have no salt.bin; they used a
//     deterministic name-derived key. Open FAILS CLOSED on them and
//     directs the operator to `aether workspace rekey` (migration path:
//     OpenForMigration + Rekey).

const (
	saltFileName    = "salt.bin"
	saltFileVersion = 1
	saltLen         = 16
	saltTagLen      = 14
	keylessMarker   = "KEYLESS"
	argon2Time      = 3
	argon2MemoryKiB = 64 * 1024
	argon2Threads   = 4
	argon2KeyLen    = 32
)

// Workspace is an engagement-scoped data plane. All artifacts, tokens,
// and logs belong to exactly one workspace. Stage 2: all state persists
// through the canonical vault (vault.db, bbolt) behind the store.Store
// contract — records and journal entries remain per-record AES-256-GCM
// sealed with the master key.
type Workspace struct {
	Name    string
	Root    string // OS config dir /aether/workspaces/<Name>
	Keyless bool   // opened with the explicit keyless marker
	pass    []byte // derived key (never persisted)
	salt    []byte // per-workspace random salt
	vault   *store.Vault

	auditMu     sync.Mutex
	auditChain  *store.Log // lazily created, shared by all spine runs
	rollbackStk *rollback.Stack
}

// Dir returns the aether workspace root for the current OS profile
// (migrates the legacy ~/.config layout transparently).
func Dir() string {
	return paths.WorkspacesDir()
}

// Paths of interest inside a workspace.
func (w *Workspace) Artifacts() string { return filepath.Join(w.Root, "artifacts") }
func (w *Workspace) Reports() string   { return filepath.Join(w.Root, "reports") }

// VaultPath returns the canonical storage file for this workspace.
func (w *Workspace) VaultPath() string { return filepath.Join(w.Root, "vault.db") }

// Vault exposes the workspace's canonical store (read-only handle for
// subsystems that need vault-backed persistence, e.g. the teamserver
// event store). Callers must Close the workspace, not the vault.
func (w *Workspace) Vault() *store.Vault { return w.vault }

// saltFilePath returns the workspace key-file path.
func (w *Workspace) saltFilePath() string { return filepath.Join(w.Root, saltFileName) }

// Create makes the workspace directory structure and provisions its
// random key salt. An empty passphrase creates an explicitly keyless
// workspace (KEYLESS marker); production workspaces must use a real
// passphrase.
func Create(name, passphrase string) (*Workspace, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	root, err := SafeJoin(Dir(), name)
	if err != nil {
		return nil, fmt.Errorf("workspace %q: %w", name, err)
	}

	for _, dir := range []string{
		filepath.Join(root, "artifacts"),
		filepath.Join(root, "reports"),
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create workspace %s: %w", dir, err)
		}
	}

	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate workspace salt: %w", err)
	}
	key := deriveKeyArgon2(passphrase, salt)
	if err := writeSaltFile(filepath.Join(root, saltFileName), salt, key); err != nil {
		return nil, fmt.Errorf("write workspace key file: %w", err)
	}
	if passphrase == "" {
		if err := os.WriteFile(filepath.Join(root, keylessMarker), nil, 0o600); err != nil {
			return nil, fmt.Errorf("write keyless marker: %w", err)
		}
	}

	w := &Workspace{Name: name, Root: root, pass: key, salt: salt, Keyless: passphrase == ""}
	if err := w.attachVault(); err != nil {
		return nil, err
	}
	return w, nil
}

// attachVault opens (creating if needed) the workspace vault and runs
// the legacy-layout migration. It is called by Create and Open.
func (w *Workspace) attachVault() error {
	if w.vault != nil {
		return nil
	}
	v, err := store.OpenVault(w.VaultPath())
	if err != nil {
		return err
	}
	w.vault = v
	return w.migrateLegacyLayout()
}

// migrateLegacyLayout imports the pre-Stage-2 `db/` directory layout
// (per-file records, journal blob, audit.jsonl, rollback.jsonl) into
// the vault, then renames the directory to db.pre-vault-imported. The
// migration is idempotent: a completed import leaves no `db/` dir.
// No user data is deleted — the original directory is preserved.
func (w *Workspace) migrateLegacyLayout() error {
	legacyDir := filepath.Join(w.Root, "db")
	if _, err := os.Stat(legacyDir); os.IsNotExist(err) {
		return nil
	}

	// 1. Records: db/<bucket>/<key> files.
	buckets, err := os.ReadDir(legacyDir)
	if err != nil {
		return err
	}
	for _, b := range buckets {
		if !b.IsDir() || b.Name() == "events" {
			continue
		}
		keys, err := os.ReadDir(filepath.Join(legacyDir, b.Name()))
		if err != nil {
			return err
		}
		for _, k := range keys {
			if k.IsDir() {
				continue
			}
			sealed, err := os.ReadFile(filepath.Join(legacyDir, b.Name(), k.Name()))
			if err != nil {
				return err
			}
			if err := w.vault.PutRecord(b.Name(), k.Name(), sealed); err != nil {
				return err
			}
		}
	}

	// 2. Journal blob → individual sealed entries.
	journalPath := filepath.Join(legacyDir, BucketEvents, "journal")
	if sealed, err := os.ReadFile(journalPath); err == nil {
		data, err := w.Open(sealed)
		if err != nil {
			return fmt.Errorf("journal migration: %w", err)
		}
		var events []Event
		if err := json.Unmarshal(data, &events); err != nil {
			return fmt.Errorf("journal migration parse: %w", err)
		}
		for _, ev := range events {
			if err := w.appendEvent(ev); err != nil {
				return err
			}
		}
	}

	// 3. Audit chain.
	auditPath := filepath.Join(legacyDir, "audit.jsonl")
	if entries, err := store.ReadJSONLEntries(auditPath); err == nil && len(entries) > 0 {
		for _, e := range entries {
			raw, err := json.Marshal(e)
			if err != nil {
				return err
			}
			if err := w.vault.AppendAuditEntry(raw, uint64(e.Seq)); err != nil {
				return fmt.Errorf("audit migration: %w", err)
			}
		}
	}
	keyPath := filepath.Join(legacyDir, "audit.key")
	if keyRaw, err := os.ReadFile(keyPath); err == nil {
		if cur, _ := w.vault.AuditMetaGet(store.MetaAuditKeyName); len(cur) == 0 {
			_ = w.vault.AuditMetaSet(store.MetaAuditKeyName, bytes.TrimSpace(keyRaw))
		}
	}

	// 4. Rollback stack.
	rbPath := filepath.Join(legacyDir, "rollback.jsonl")
	if raw, err := os.ReadFile(rbPath); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			if line = strings.TrimSpace(line); line == "" {
				continue
			}
			if _, err := w.vault.RollbackPush([]byte(line)); err != nil {
				return fmt.Errorf("rollback migration: %w", err)
			}		}
	}

	// 5. Preserve the original directory (no user data destroyed).
	return os.Rename(legacyDir, filepath.Join(w.Root, "db.pre-vault-imported"))
}

// Close releases the workspace vault and its file lock.
func (w *Workspace) Close() error {
	if w.vault == nil {
		return nil
	}
	err := w.vault.Close()
	w.vault = nil
	return err
}

// AuditLog returns the signed audit chain persisted in the vault. The
// chain instance is cached per workspace so concurrent spine runs
// serialize on one Log (its mutex preserves seq/hash continuity);
// the signing key itself is initialized atomically in the vault.
func (w *Workspace) AuditLog() (*store.Log, error) {
	w.auditMu.Lock()
	defer w.auditMu.Unlock()
	if w.auditChain != nil {
		return w.auditChain, nil
	}
	if w.vault == nil {
		return nil, fmt.Errorf("workspace %q: vault not open", w.Name)
	}
	l, err := store.NewVaultLog(w.vault)
	if err != nil {
		return nil, err
	}
	w.auditChain = l
	return l, nil
}

// RollbackStack returns the rollback stack persisted in the vault.
func (w *Workspace) RollbackStack() *rollback.Stack {
	w.auditMu.Lock()
	defer w.auditMu.Unlock()
	if w.rollbackStk == nil {
		w.rollbackStk = rollback.New(w.vault)
	}
	return w.rollbackStk
}

// Open loads an existing workspace and derives its crypto key from the
// passphrase (Argon2id over the workspace's random salt).
//
// Fail-closed policy:
//   - empty passphrase is rejected unless the workspace carries the
//     explicit KEYLESS marker (and then a prominent warning is printed);
//   - workspaces in the deprecated deterministic-key layout (no
//     salt.bin) are rejected with migration instructions;
//   - a wrong passphrase or a tampered key file fails verification.
func Open(name, passphrase string) (*Workspace, error) {
	w, err := openWorkspace(name, passphrase, false)
	if err != nil {
		return nil, err
	}
	return w, nil
}

// OpenForMigration opens a workspace for rekey purposes. Unlike Open it
// tolerates the legacy deterministic-key layout (no salt.bin) and
// keyless markers without warning, because migration is the only
// sanctioned way to move off those layouts.
func OpenForMigration(name, passphrase string) (*Workspace, error) {
	return openWorkspace(name, passphrase, true)
}

func openWorkspace(name, passphrase string, migration bool) (*Workspace, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	root, err := SafeJoin(Dir(), name)
	if err != nil {
		return nil, fmt.Errorf("workspace %q: %w", name, err)
	}
	if _, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf("workspace %q not found: %w", name, err)
	}

	raw, err := os.ReadFile(filepath.Join(root, saltFileName))
	if os.IsNotExist(err) {
		if !migration {
			return nil, fmt.Errorf(
				"workspace %q uses the deprecated deterministic key layout and cannot be opened directly; "+
					"migrate it with: aether workspace rekey --workspace %s --old-passphrase <old> --new-passphrase <new>",
				name, name)
		}
		// Legacy layout: reproduce the old name-derived salt.
		w := &Workspace{Name: name, Root: root}
		w.salt = legacySalt(name)
		w.pass = deriveKeyArgon2(passphrase, w.salt)
		if err := w.attachVault(); err != nil {
			return nil, err
		}
		return w, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read workspace key file: %w", err)
	}
	if len(raw) != 2+saltLen+saltTagLen {
		return nil, fmt.Errorf("workspace key file %s is corrupt (unexpected length)", saltFileName)
	}
	if v := binary.BigEndian.Uint16(raw[:2]); v != saltFileVersion {
		return nil, fmt.Errorf("workspace key file version %d unsupported", v)
	}
	salt := raw[2 : 2+saltLen]

	keyless := false
	if passphrase == "" {
		if _, err := os.Stat(filepath.Join(root, keylessMarker)); err != nil {
			return nil, fmt.Errorf("workspace %q is passphrase-protected: an empty passphrase is rejected", name)
		}
		keyless = true
	}

	key := deriveKeyArgon2(passphrase, salt)
	if !hmac.Equal(saltTag(raw[:2+saltLen], key), raw[2+saltLen:]) {
		return nil, fmt.Errorf("workspace %q: wrong passphrase or corrupted %s", name, saltFileName)
	}

	w := &Workspace{Name: name, Root: root, pass: key, salt: salt, Keyless: keyless}
	if err := w.attachVault(); err != nil {
		return nil, err
	}
	if keyless {
		warnKeyless(name)
	}
	return w, nil
}

func warnKeyless(name string) {
	fmt.Fprintf(os.Stderr,
		"WARNING: workspace %q is open in KEYLESS mode (passphrase is empty). "+
			"The encryption key is trivially derivable by anyone with filesystem access. "+
			"Run 'aether workspace rekey' to set a real passphrase.\n", name)
}

// legacySalt reproduces the pre-Stage-1 deterministic salt so the
// migration path can decrypt legacy workspaces.
func legacySalt(name string) []byte {
	s := sha256.Sum256([]byte("aether-salt:" + name))
	return s[:]
}

// deriveKeyArgon2 derives the AES-256 vault key with Argon2id.
func deriveKeyArgon2(passphrase string, salt []byte) []byte {
	return argon2.IDKey([]byte(passphrase), salt, argon2Time, argon2MemoryKiB, argon2Threads, argon2KeyLen)
}

// writeSaltFile persists version||salt||HMAC(key, version||salt)[:14].
func writeSaltFile(path string, salt, key []byte) error {
	raw := make([]byte, 0, 2+saltLen+saltTagLen)
	var ver [2]byte
	binary.BigEndian.PutUint16(ver[:], saltFileVersion)
	raw = append(raw, ver[:]...)
	raw = append(raw, salt...)
	raw = append(raw, saltTag(raw, key)...)
	return os.WriteFile(path, raw, 0o600)
}

// saltTag computes the truncated HMAC tag over version||salt.
func saltTag(versionAndSalt, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(versionAndSalt)
	return mac.Sum(nil)[:saltTagLen]
}

// DeriveKey re-derives the workspace key from a passphrase using the
// workspace's stored salt. Used by Rekey; interactive opens go through
// Open, which also verifies the key file.
func (w *Workspace) DeriveKey(passphrase string) error {
	if w.salt == nil {
		return fmt.Errorf("workspace %q: no key salt loaded", w.Name)
	}
	w.pass = deriveKeyArgon2(passphrase, w.salt)
	return nil
}

// readSaltFile loads the workspace key file. Returns hasSaltFile=false
// when the file does not exist (legacy layout).
func (w *Workspace) readSaltFile() (salt []byte, hasSaltFile bool, err error) {
	raw, err := os.ReadFile(w.saltFilePath())
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if len(raw) != 2+saltLen+saltTagLen {
		return nil, false, fmt.Errorf("workspace key file %s is corrupt (unexpected length)", saltFileName)
	}
	if v := binary.BigEndian.Uint16(raw[:2]); v != saltFileVersion {
		return nil, false, fmt.Errorf("workspace key file version %d unsupported", v)
	}
	// Note: the tag is verified against the derived key by callers that
	// hold a candidate passphrase (openWorkspace); Rekey verifies the
	// vault by decrypting records, which is a strictly stronger check.
	return raw[2 : 2+saltLen], true, nil
}

// List enumerates existing workspace names.
func List() ([]string, error) {
	entries, err := os.ReadDir(Dir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// Exists reports whether a workspace is present.
func Exists(name string) bool {
	if ValidateName(name) != nil {
		return false
	}
	_, err := os.Stat(filepath.Join(Dir(), name))
	return err == nil
}

// Delete securely removes a workspace: files are overwritten with
// zeros before unlinking. The name is validated first so a traversal
// name can never reach the shred/remove path.
func Delete(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	root, err := SafeJoin(Dir(), name)
	if err != nil {
		return fmt.Errorf("workspace %q: %w", name, err)
	}
	if _, err := os.Stat(root); err != nil {
		return fmt.Errorf("workspace %q not found", name)
	}

	var wipeErr error
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if errW := shredFile(path); errW != nil {
			wipeErr = errW
		}
		return nil
	})
	if wipeErr != nil {
		return wipeErr
	}
	return os.RemoveAll(root)
}

func shredFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	size := info.Size()
	if size == 0 {
		return nil
	}

	f, err := os.OpenFile(path, os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	zeros := make([]byte, 4096)
	if _, err := f.WriteAt(zeros, 0); err != nil && size > int64(len(zeros)) {
		return err
	}
	// Full overwrite in chunks.
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}
	for remaining := size; remaining > 0; {
		n := int64(len(zeros))
		if remaining < n {
			n = remaining
		}
		if _, err := f.Write(zeros[:n]); err != nil {
			return err
		}
		remaining -= n
	}
	return f.Sync()
}

// Seal encrypts plaintext with AES-256-GCM. Output: nonce || ciphertext.
func (w *Workspace) Seal(plaintext []byte) ([]byte, error) {
	if len(w.pass) != 32 {
		return nil, fmt.Errorf("workspace key not derived")
	}

	block, err := aes.NewCipher(w.pass)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Open decrypts AES-256-GCM sealed data.
func (w *Workspace) Open(sealed []byte) ([]byte, error) {
	if len(w.pass) != 32 {
		return nil, fmt.Errorf("workspace key not derived")
	}

	block, err := aes.NewCipher(w.pass)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(sealed) < gcm.NonceSize() {
		return nil, fmt.Errorf("sealed data too short")
	}
	nonce, ciphertext := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// SaveRecord encrypts and stores a JSON record under bucket/key.
func (w *Workspace) SaveRecord(bucket, key string, v any) error {
	if err := ValidateRecordKey(key); err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	sealed, err := w.Seal(data)
	if err != nil {
		return err
	}
	return w.vault.PutRecord(bucket, key, sealed)
}

// LoadRecord loads and decrypts a JSON record into out.
func (w *Workspace) LoadRecord(bucket, key string, out any) error {
	if err := ValidateRecordKey(key); err != nil {
		return err
	}
	sealed, err := w.vault.GetRecord(bucket, key)
	if err != nil {
		return err
	}
	data, err := w.Open(sealed)
	if err != nil {
		return fmt.Errorf("decrypt %s/%s: %w", bucket, key, err)
	}
	return json.Unmarshal(data, out)
}

// ListRecords enumerates keys in a bucket.
func (w *Workspace) ListRecords(bucket string) ([]string, error) {
	if err := ValidateRecordKey(bucket); err != nil {
		return nil, err
	}
	return w.vault.ListRecords(bucket)
}

// DeleteRecord removes a record.
func (w *Workspace) DeleteRecord(bucket, key string) error {
	if err := ValidateRecordKey(key); err != nil {
		return err
	}
	return w.vault.DeleteRecord(bucket, key)
}

// LogEvent appends a timestamped JSON event to the workspace journal.
// Stage 2 semantics (forensic F9): the event is appended to the vault
// journal with a monotonic sequence number in a single transaction.
// The previous read-modify-rewrite blob (which silently destroyed all
// history on a corrupt read and raced concurrent writers) is gone.
// A corrupt entry fails closed with an error — history is never
// discarded.
type Event struct {
	Time   time.Time `json:"time"`
	Kind   string    `json:"kind"`
	Detail string    `json:"detail"`
}

func (w *Workspace) LogEvent(kind, detail string) error {
	return w.appendEvent(Event{Time: time.Now().UTC(), Kind: kind, Detail: detail})
}

func (w *Workspace) appendEvent(ev Event) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	sealed, err := w.Seal(data)
	if err != nil {
		return err
	}
	_, err = w.vault.AppendJournal(sealed)
	return err
}

// Events returns the decrypted journal oldest-first.
func (w *Workspace) Events() ([]Event, error) {
	sealedEntries, err := w.vault.ReadJournal()
	if err != nil {
		return nil, err
	}
	var events []Event
	for i, sealed := range sealedEntries {
		data, err := w.Open(sealed)
		if err != nil {
			// Fail closed: never silently drop history.
			return nil, fmt.Errorf("journal entry %d is corrupt: %w", i+1, err)
		}
		var ev Event
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil, fmt.Errorf("journal entry %d is malformed: %w", i+1, err)
		}
		events = append(events, ev)
	}
	return events, nil
}

// SaveArtifact stores a raw artifact (ccache, dumps) unencrypted on
// disk but tracked in the journal. The artifact name is validated to
// prevent path traversal outside the artifacts directory.
func (w *Workspace) SaveArtifact(name string, data []byte) (string, error) {
	if err := ValidateArtifactName(name); err != nil {
		return "", err
	}
	if err := os.MkdirAll(w.Artifacts(), 0o700); err != nil {
		return "", err
	}
	path, err := SafeJoin(w.Artifacts(), name)
	if err != nil {
		return "", fmt.Errorf("artifact %q: %w", name, err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	if err := w.LogEvent("artifact_saved", name); err != nil {
		return path, err
	}
	return path, nil
}
