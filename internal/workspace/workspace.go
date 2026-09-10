package workspace

import (
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
	"time"

	"github.com/Debajyoti0-0/aether/internal/paths"
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
// and logs belong to exactly one workspace.
type Workspace struct {
	Name    string
	Root    string // OS config dir /aether/workspaces/<Name>
	Keyless bool   // opened with the explicit keyless marker
	pass    []byte // derived key (never persisted)
	salt    []byte // per-workspace random salt
}

// Dir returns the aether workspace root for the current OS profile
// (migrates the legacy ~/.config layout transparently).
func Dir() string {
	return paths.WorkspacesDir()
}

// Paths of interest inside a workspace.
func (w *Workspace) Artifacts() string { return filepath.Join(w.Root, "artifacts") }
func (w *Workspace) Reports() string   { return filepath.Join(w.Root, "reports") }

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
		filepath.Join(root, "db"),
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

	return &Workspace{Name: name, Root: root, pass: key, salt: salt, Keyless: passphrase == ""}, nil
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
	return os.WriteFile(w.recordPath(bucket, key), sealed, 0o600)
}

// LoadRecord loads and decrypts a JSON record into out.
func (w *Workspace) LoadRecord(bucket, key string, out any) error {
	if err := ValidateRecordKey(key); err != nil {
		return err
	}
	sealed, err := os.ReadFile(w.recordPath(bucket, key))
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
	dir := w.recordPath(bucket, "")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// DeleteRecord removes a record.
func (w *Workspace) DeleteRecord(bucket, key string) error {
	if err := ValidateRecordKey(key); err != nil {
		return err
	}
	err := os.Remove(w.recordPath(bucket, key))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (w *Workspace) recordPath(bucket, key string) string {
	if err := ValidateRecordKey(bucket); err != nil {
		// Buckets are package constants; an invalid bucket name is a
		// programming error. Fail closed rather than write outside the
		// workspace.
		return filepath.Join(w.Root, "db", "INVALID_BUCKET")
	}
	dir := filepath.Join(w.Root, "db", bucket)
	// Ensure the bucket dir exists lazily.
	_ = os.MkdirAll(dir, 0o700)
	if key == "" {
		return dir
	}
	if err := ValidateRecordKey(key); err != nil {
		return filepath.Join(dir, "INVALID_KEY")
	}
	return filepath.Join(dir, key)
}

// LogEvent appends a timestamped JSON event to the workspace journal
// (encrypted per record).
type Event struct {
	Time    time.Time `json:"time"`
	Kind    string    `json:"kind"`
	Detail  string    `json:"detail"`
}

func (w *Workspace) LogEvent(kind, detail string) error {
	events, _ := w.loadEvents()
	events = append(events, Event{Time: time.Now().UTC(), Kind: kind, Detail: detail})
	return w.storeEvents(events)
}

func (w *Workspace) loadEvents() ([]Event, error) {
	sealed, err := os.ReadFile(w.recordPath(BucketEvents, "journal"))
	if err != nil {
		return nil, nil
	}
	data, err := w.Open(sealed)
	if err != nil {
		return nil, err
	}
	var events []Event
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, err
	}
	return events, nil
}

func (w *Workspace) storeEvents(events []Event) error {
	data, err := json.Marshal(events)
	if err != nil {
		return err
	}
	sealed, err := w.Seal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(w.recordPath(BucketEvents, "journal"), sealed, 0o600)
}

// Events returns the decrypted journal.
func (w *Workspace) Events() ([]Event, error) {
	return w.loadEvents()
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
