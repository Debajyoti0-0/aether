package store

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

var base64Std = base64.StdEncoding

// Entry is one tamper-evident audit record.
type Entry struct {
	Seq       int64     `json:"seq"`
	Timestamp time.Time `json:"timestamp"`
	Command   string    `json:"command"`
	Result    string    `json:"result,omitempty"`
	// PrevHash links to the previous entry (genesis = all zeros).
	PrevHash  string    `json:"prev_hash"`
	// Hash covers seq+timestamp+command+result+prevHash.
	Hash      string    `json:"hash"`
	// Signature is the Ed25519 signature over Hash.
	Signature string    `json:"signature"`
}

// Log is an append-only signed audit trail stored as JSONL.
type Log struct {
	mu       sync.Mutex
	path     string
	key      ed25519.PrivateKey
	prevHash string
	seq      int64
}

// GenesisHash is the chain anchor for an empty log.
const GenesisHash = "0000000000000000000000000000000000000000000000000000000000000000"

// New opens (or creates) an audit log at path. The key is loaded from
// keyPath if present, otherwise generated and persisted.
func New(path, keyPath string) (*Log, error) {
	if err := os.MkdirAll(dir(path), 0o700); err != nil {
		return nil, err
	}

	key, err := loadOrCreateKey(keyPath)
	if err != nil {
		return nil, err
	}

	l := &Log{path: path, key: key, prevHash: GenesisHash}

	// Resume sequence + chain from any existing entries.
	entries, err := l.readAll()
	if err != nil {
		return nil, err
	}
	if len(entries) > 0 {
		last := entries[len(entries)-1]
		l.seq = last.Seq
		l.prevHash = last.Hash
	}
	return l, nil
}

// Append records a command/result and extends the signature chain.
func (l *Log) Append(command, result string) (*Entry, error) {
	if command == "" {
		return nil, fmt.Errorf("command is required")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.seq++
	e := Entry{
		Seq:      l.seq,
		Timestamp: time.Now().UTC(),
		Command:  command,
		Result:   result,
		PrevHash: l.prevHash,
	}
	e.Hash = computeHash(e)

	sig := ed25519.Sign(l.key, []byte(e.Hash))
	e.Signature = base64Encode(sig)

	data, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return nil, err
	}

	l.prevHash = e.Hash
	return &e, nil
}

// VerifyResult reports the integrity of the whole chain.
type VerifyResult struct {
	Total     int      `json:"total"`
	Valid     int      `json:"valid"`
	Tampered  []int64  `json:"tampered,omitempty"`
	BrokenChainAt int64 `json:"broken_chain_at,omitempty"`
	ValidAll  bool     `json:"valid_all"`
}

// Verify walks the chain forward, recomputing hashes and checking
// signatures and linkage.
func (l *Log) Verify() (*VerifyResult, error) {
	entries, err := l.readAll()
	if err != nil {
		return nil, err
	}

	res := &VerifyResult{Total: len(entries), ValidAll: true}
	prevHash := GenesisHash

	for _, e := range entries {
		// Chain linkage.
		if e.PrevHash != prevHash {
			res.BrokenChainAt = e.Seq
			res.ValidAll = false
		}
		prevHash = e.Hash

		// Hash integrity.
		if computeHash(e) != e.Hash {
			res.Tampered = append(res.Tampered, e.Seq)
			res.ValidAll = false
			continue
		}

		// Signature.
		sig, err := base64Decode(e.Signature)
		if err != nil || !ed25519.Verify(l.key.Public().(ed25519.PublicKey), []byte(e.Hash), sig) {
			res.Tampered = append(res.Tampered, e.Seq)
			res.ValidAll = false
			continue
		}
		res.Valid++
	}
	return res, nil
}

// Entries returns all entries oldest-first.
func (l *Log) Entries() ([]Entry, error) {
	return l.readAll()
}

// ExportJSONL renders the trail as signed JSONL for submission.
func (l *Log) ExportJSONL() (string, error) {
	entries, err := l.readAll()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, e := range entries {
		data, err := json.Marshal(e)
		if err != nil {
			return "", err
		}
		b.Write(data)
		b.WriteByte('\n')
	}
	return b.String(), nil
}

// computeHash covers every integrity-relevant field.
func computeHash(e Entry) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d|%s|%s|%s|%s", e.Seq, e.Timestamp.UTC().Format(time.RFC3339Nano), e.Command, e.Result, e.PrevHash)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func loadOrCreateKey(keyPath string) (ed25519.PrivateKey, error) {
	if data, err := os.ReadFile(keyPath); err == nil {
		seed, err := base64Decode(strings.TrimSpace(string(data)))
		if err != nil {
			return nil, fmt.Errorf("decode audit key: %w", err)
		}
		return ed25519.NewKeyFromSeed(seed), nil
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(keyPath, []byte(base64Encode(priv.Seed())+"\n"), 0o600); err != nil {
		return nil, err
	}
	return priv, nil
}

func (l *Log) readAll() ([]Entry, error) {
	data, err := os.ReadFile(l.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []Entry
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, fmt.Errorf("corrupt audit entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func base64Encode(b []byte) string {
	return base64Std.EncodeToString(b)
}

func base64Decode(s string) ([]byte, error) {
	return base64Std.DecodeString(s)
}

// VerifyFile verifies an exported JSONL trail against a key.
func VerifyFile(jsonlPath, keyPath string) (*VerifyResult, error) {
	key, err := loadOrCreateKey(keyPath)
	if err != nil {
		return nil, err
	}
	l := &Log{path: jsonlPath, key: key}
	return l.Verify()
}

// ExportFile writes the current trail to a target path.
func ExportFile(l *Log, target string) error {
	data, err := l.ExportJSONL()
	if err != nil {
		return err
	}
	return os.WriteFile(target, []byte(data), 0o600)
}

// AppendCtx is a context-aware convenience wrapper.
func AppendCtx(ctx context.Context, l *Log, command, result string) (*Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return l.Append(command, result)
}

func dir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}
