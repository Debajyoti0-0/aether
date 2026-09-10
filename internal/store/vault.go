package store

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Vault is the canonical workspace storage engine (Stage 2, forensic F9):
// a single bbolt file per workspace providing ACID transactions,
// cross-process file locking, crash-safe copy-on-write pages, and
// schema versioning. Records and journal entries are stored as
// caller-sealed ciphertext (per-record AES-256-GCM with the workspace
// master key — bbolt provides integrity, never confidentiality).
//
// Buckets:
//
//	meta            schema_version, journal_seq, audit state
//	records         bucket\x00key -> sealed ciphertext
//	journal         seq (8B BE) -> sealed event JSON (monotonic, append-only)
//	audit           seq -> signed audit Entry JSON (Stage 1 chain format)
//	rollback        seq -> rollback Action JSON (LIFO by seq)
//	rollback_failed seq -> rollback Action JSON retained after failed undo
//
// Cross-process safety: bbolt takes an exclusive flock on the file;
// a second process opening the same vault fails with a timeout error.
var (
	bucketMeta           = []byte("meta")
	bucketRecords        = []byte("records")
	bucketJournal        = []byte("journal")
	bucketAudit          = []byte("audit")
	bucketRollback       = []byte("rollback")
	bucketRollbackFailed = []byte("rollback_failed")
)

// SchemaVersion is the vault schema understood by this build. Open
// refuses vaults written by a NEWER schema and callers run migrations
// for older ones.
const SchemaVersion uint64 = 1

const (
	metaSchemaVersion = "schema_version"
	metaJournalSeq    = "journal_seq"
	metaRollbackSeq   = "rollback_seq"
	metaAuditSeq      = "audit_seq"
	metaAuditPrevHash = "audit_prev_hash"
	metaAuditKey      = "audit_key"
	metaCreatedAt     = "created_at"
)

// lockTimeout bounds how long Open waits for the cross-process file
// lock before reporting the workspace as locked.
const lockTimeout = 2 * time.Second

// Vault is a handle to one workspace's vault.db.
type Vault struct {
	db   *bolt.DB
	path string
}

// OpenVault opens (creating if necessary) the vault at path. It fails
// if another process holds the file lock, or if the stored schema
// version is newer than this build.
func OpenVault(path string) (*Vault, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: lockTimeout})
	if err != nil {
		if err == bolt.ErrTimeout {
			return nil, fmt.Errorf("workspace vault %s is locked by another process", path)
		}
		return nil, fmt.Errorf("open vault %s: %w", path, err)
	}
	v := &Vault{db: db, path: path}
	if err := v.initBuckets(); err != nil {
		db.Close()
		return nil, err
	}
	if err := v.checkSchema(); err != nil {
		db.Close()
		return nil, err
	}
	return v, nil
}

// Close releases the vault (and its file lock).
func (v *Vault) Close() error { return v.db.Close() }

// Path returns the vault file path.
func (v *Vault) Path() string { return v.path }

// Sync flushes the vault to stable storage. The audit path calls this
// after every signed append (audit is the legal artifact); other
// writes rely on bbolt's periodic sync.
func (v *Vault) Sync() error { return v.db.Sync() }

func (v *Vault) initBuckets() error {
	return v.db.Update(func(tx *bolt.Tx) error {
		for _, b := range [][]byte{bucketMeta, bucketRecords, bucketJournal, bucketAudit, bucketRollback, bucketRollbackFailed} {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return err
			}
		}
		m := tx.Bucket(bucketMeta)
		if m.Get([]byte(metaSchemaVersion)) == nil {
			if err := m.Put([]byte(metaSchemaVersion), u64be(SchemaVersion)); err != nil {
				return err
			}
		}
		if m.Get([]byte(metaCreatedAt)) == nil {
			if err := m.Put([]byte(metaCreatedAt), []byte(time.Now().UTC().Format(time.RFC3339))); err != nil {
				return err
			}
		}
		return nil
	})
}

func (v *Vault) checkSchema() error {
	var raw []byte
	err := v.db.View(func(tx *bolt.Tx) error {
		raw = tx.Bucket(bucketMeta).Get([]byte(metaSchemaVersion))
		return nil
	})
	if err != nil {
		return err
	}
	stored := beU64(raw)
	if stored > SchemaVersion {
		return fmt.Errorf("vault schema version %d is newer than this build (%d); upgrade aether to open this workspace", stored, SchemaVersion)
	}
	// stored < SchemaVersion would run migrations here (none needed at v1).
	return nil
}

// ---- records (caller-sealed ciphertext KV) ----

func recordKey(bucket, key string) []byte { return []byte(bucket + "\x00" + key) }

// PutRecord stores a sealed record.
func (v *Vault) PutRecord(bucket, key string, sealed []byte) error {
	return v.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketRecords).Put(recordKey(bucket, key), sealed)
	})
}

// GetRecord returns the sealed record bytes.
func (v *Vault) GetRecord(bucket, key string) ([]byte, error) {
	var out []byte
	err := v.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket(bucketRecords).Get(recordKey(bucket, key))
		if raw == nil {
			return fmt.Errorf("record %s/%s not found", bucket, key)
		}
		out = append([]byte(nil), raw...)
		return nil
	})
	return out, err
}

// ListRecords enumerates keys of one record bucket (the \x00-delimited
// composite key's second component).
func (v *Vault) ListRecords(bucket string) ([]string, error) {
	prefix := []byte(bucket + "\x00")
	var out []string
	err := v.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketRecords).ForEach(func(k, _ []byte) error {
			if bytes.HasPrefix(k, prefix) {
				out = append(out, string(k[len(prefix):]))
			}
			return nil
		})
	})
	return out, err
}

// DeleteRecord removes a record.
func (v *Vault) DeleteRecord(bucket, key string) error {
	return v.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketRecords).Delete(recordKey(bucket, key))
	})
}

// ---- journal (append-only, monotonic seq, caller-sealed) ----

// AppendJournal durably appends one sealed event and returns its
// monotonic sequence number. The sequence lives in the same
// transaction as the entry, so a crash can never produce a gap or a
// duplicate.
func (v *Vault) AppendJournal(sealed []byte) (uint64, error) {
	var seq uint64
	err := v.db.Update(func(tx *bolt.Tx) error {
		m := tx.Bucket(bucketMeta)
		seq = beU64(m.Get([]byte(metaJournalSeq))) + 1
		if err := m.Put([]byte(metaJournalSeq), u64be(seq)); err != nil {
			return err
		}
		return tx.Bucket(bucketJournal).Put(u64be(seq), sealed)
	})
	return seq, err
}

// ReadJournal returns all sealed events in sequence order.
func (v *Vault) ReadJournal() ([][]byte, error) {
	var out [][]byte
	err := v.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketJournal).ForEach(func(_, v []byte) error {
			out = append(out, append([]byte(nil), v...))
			return nil
		})
	})
	return out, err
}

// ReplaceJournal atomically swaps the whole journal for a new set of
// sealed entries (used by rekey: old-key entries cannot be selectively
// rewritten).
func (v *Vault) ReplaceJournal(sealed [][]byte) error {
	return v.db.Update(func(tx *bolt.Tx) error {
		if err := tx.Bucket(bucketJournal).ForEach(func(k, _ []byte) error {
			return tx.Bucket(bucketJournal).Delete(k)
		}); err != nil {
			return err
		}
		m := tx.Bucket(bucketMeta)
		if err := m.Put([]byte(metaJournalSeq), u64be(uint64(len(sealed)))); err != nil {
			return err
		}
		for i, e := range sealed {
			if err := tx.Bucket(bucketJournal).Put(u64be(uint64(i+1)), e); err != nil {
				return err
			}
		}
		return nil
	})
}

// ---- audit (signed chain; JSON format is Stage-1 frozen) ----

// AppendAuditEntry stores one signed Entry JSON under its seq. The
// caller (Log) owns chain semantics; the vault enforces seq monotonicity
// in the same transaction.
func (v *Vault) AppendAuditEntry(entryJSON []byte, seq uint64) error {
	return v.db.Update(func(tx *bolt.Tx) error {
		m := tx.Bucket(bucketMeta)
		if cur := beU64(m.Get([]byte(metaAuditSeq))); seq != cur+1 {
			return fmt.Errorf("audit seq %d out of order (want %d)", seq, cur+1)
		}
		if err := m.Put([]byte(metaAuditSeq), u64be(seq)); err != nil {
			return err
		}
		return tx.Bucket(bucketAudit).Put(u64be(seq), entryJSON)
	})
}

// ReadAuditEntries returns all audit entries oldest-first.
func (v *Vault) ReadAuditEntries() ([][]byte, error) {
	var out [][]byte
	err := v.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(bucketAudit).Cursor()
		for k, val := c.First(); k != nil; k, val = c.Next() {
			out = append(out, append([]byte(nil), val...))
		}
		return nil
	})
	return out, err
}

// AuditMetaGet/Set store audit-chain metadata (prev hash, signing key).
func (v *Vault) AuditMetaGet(name string) ([]byte, error) {
	var out []byte
	err := v.db.View(func(tx *bolt.Tx) error {
		if raw := tx.Bucket(bucketMeta).Get([]byte(name)); raw != nil {
			out = append([]byte(nil), raw...)
		}
		return nil
	})
	return out, err
}

func (v *Vault) AuditMetaSet(name string, val []byte) error {
	return v.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketMeta).Put([]byte(name), val)
	})
}

// ---- rollback (LIFO by seq; failed reversals retained) ----

// RollbackPush appends an action JSON and returns its seq.
func (v *Vault) RollbackPush(actionJSON []byte) (uint64, error) {
	var seq uint64
	err := v.db.Update(func(tx *bolt.Tx) error {
		m := tx.Bucket(bucketMeta)
		seq = beU64(m.Get([]byte(metaRollbackSeq))) + 1
		if err := m.Put([]byte(metaRollbackSeq), u64be(seq)); err != nil {
			return err
		}
		return tx.Bucket(bucketRollback).Put(u64be(seq), actionJSON)
	})
	return seq, err
}

// RollbackPop atomically reads, removes, and returns the newest action
// (single bbolt transaction — the Stage 1 rewrite-before-validate
// corruption window is gone). The popped JSON is retained in
// rollback_failed on failed undo attempts by the caller.
func (v *Vault) RollbackPop() ([]byte, error) {
	var out []byte
	err := v.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketRollback)
		var lastKey []byte
		var lastVal []byte
		c := b.Cursor()
		for k, val := c.Last(); k != nil; k, val = c.Last() {
			lastKey = append([]byte(nil), k...)
			lastVal = append([]byte(nil), val...)
			break
		}
		if lastKey == nil {
			return fmt.Errorf("rollback stack is empty")
		}
		if err := b.Delete(lastKey); err != nil {
			return err
		}
		out = lastVal
		return nil
	})
	return out, err
}

// RollbackPeek returns the newest action without removing it.
func (v *Vault) RollbackPeek() ([]byte, error) {
	var out []byte
	err := v.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketRollback)
		_, val := b.Cursor().Last()
		if val == nil {
			return fmt.Errorf("rollback stack is empty")
		}
		out = append([]byte(nil), val...)
		return nil
	})
	return out, err
}

// RollbackList returns all actions newest-first.
func (v *Vault) RollbackList() ([][]byte, error) {
	var out [][]byte
	err := v.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(bucketRollback).Cursor()
		for k, val := c.Last(); k != nil; k, val = c.Prev() {
			out = append(out, append([]byte(nil), val...))
		}
		return nil
	})
	return out, err
}

// RollbackRetainFailed moves a popped action into the rollback_failed
// bucket so a failed reversal is never silently discarded.
func (v *Vault) RollbackRetainFailed(actionJSON []byte) error {
	return v.db.Update(func(tx *bolt.Tx) error {
		m := tx.Bucket(bucketMeta)
		seq := beU64(m.Get([]byte(metaRollbackSeq))) + 1
		if err := m.Put([]byte(metaRollbackSeq), u64be(seq)); err != nil {
			return err
		}
		return tx.Bucket(bucketRollbackFailed).Put(u64be(seq), actionJSON)
	})
}

// RollbackListFailed returns retained failed-reversal actions
// (newest-first).
func (v *Vault) RollbackListFailed() ([][]byte, error) {
	var out [][]byte
	err := v.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(bucketRollbackFailed).Cursor()
		for k, val := c.Last(); k != nil; k, val = c.Prev() {
			out = append(out, append([]byte(nil), val...))
		}
		return nil
	})
	return out, err
}

// ---- meta helpers ----

// MetaGet returns a metadata value (nil if absent).
func (v *Vault) MetaGet(name string) ([]byte, error) {
	var out []byte
	err := v.db.View(func(tx *bolt.Tx) error {
		if raw := tx.Bucket(bucketMeta).Get([]byte(name)); raw != nil {
			out = append([]byte(nil), raw...)
		}
		return nil
	})
	return out, err
}

// MetaSet writes a metadata value.
func (v *Vault) MetaSet(name string, val []byte) error {
	return v.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketMeta).Put([]byte(name), val)
	})
}

// ---- helpers ----

func u64be(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}

func beU64(b []byte) uint64 {
	if len(b) != 8 {
		return 0
	}
	return binary.BigEndian.Uint64(b)
}

// migrationJSON is a helper for the workspace migration importer.
func migrationJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
