package workspace

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Debajyoti0-0/aether/internal/paths"
	"golang.org/x/crypto/argon2"
)

// Bucket layout inside a workspace (BoltDB buckets / KV namespaces).
var (
	BucketIdentities = "identities"
	BucketTokens     = "tokens"
	BucketSessions   = "sessions"
	BucketEvidence   = "evidence"
	BucketEvents     = "events"
)

// Workspace is an engagement-scoped data plane. All artifacts, tokens,
// and logs belong to exactly one workspace.
type Workspace struct {
	Name string
	Root string // OS config dir /aether/workspaces/<Name>
	pass []byte // derived key (never persisted)
}

// Dir returns the aether workspace root for the current OS profile
// (migrates the legacy ~/.config layout transparently).
func Dir() string {
	return paths.WorkspacesDir()
}

// Paths of interest inside a workspace.
func (w *Workspace) DBPath() string      { return filepath.Join(w.Root, "db", "vault.aedb") }
func (w *Workspace) Artifacts() string   { return filepath.Join(w.Root, "artifacts") }
func (w *Workspace) Reports() string     { return filepath.Join(w.Root, "reports") }

// Create makes the workspace directory structure.
func Create(name string) (*Workspace, error) {
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
	return &Workspace{Name: name, Root: root}, nil
}

// Open loads an existing workspace and derives its crypto key from the
// passphrase (Argon2id). See DeriveKey for key-derivation policy.
func Open(name, passphrase string) (*Workspace, error) {
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
	w := &Workspace{Name: name, Root: root}
	w.DeriveKey(passphrase)
	return w, nil
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

// DeriveKey derives the AES-256 vault key with Argon2id.
// Salt is derived deterministically from the workspace name.
func (w *Workspace) DeriveKey(passphrase string) {
	salt := sha256.Sum256([]byte("aether-salt:" + w.Name))
	// 64MB memory, 3 iterations, 4 threads — OWASP-recommended baseline.
	w.pass = argon2.IDKey([]byte(passphrase), salt[:], 3, 64*1024, 4, 32)
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
