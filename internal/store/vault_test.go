package store

import (
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	bolt "go.etcd.io/bbolt"
)

// boltTx aliases the bbolt transaction type for the abort test.
type boltTx = bolt.Tx

func openTestVault(t *testing.T) *Vault {
	t.Helper()
	v, err := OpenVault(filepath.Join(t.TempDir(), "vault.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	return v
}

func TestVaultRecordsRoundTrip(t *testing.T) {
	v := openTestVault(t)
	if err := v.PutRecord("tokens", "graph", []byte("sealed-bytes")); err != nil {
		t.Fatal(err)
	}
	got, err := v.GetRecord("tokens", "graph")
	if err != nil || string(got) != "sealed-bytes" {
		t.Fatalf("get = %q err=%v", got, err)
	}
	keys, _ := v.ListRecords("tokens")
	if len(keys) != 1 || keys[0] != "graph" {
		t.Errorf("keys = %v", keys)
	}
	if err := v.DeleteRecord("tokens", "graph"); err != nil {
		t.Fatal(err)
	}
	if _, err := v.GetRecord("tokens", "graph"); err == nil {
		t.Fatal("deleted record still readable")
	}
}

func TestVaultRecordIsolation(t *testing.T) {
	v := openTestVault(t)
	if err := v.PutRecord("tokens", "shared-name", []byte("a")); err != nil {
		t.Fatal(err)
	}
	if err := v.PutRecord("evidence", "shared-name", []byte("b")); err != nil {
		t.Fatal(err)
	}
	a, _ := v.GetRecord("tokens", "shared-name")
	b, _ := v.GetRecord("evidence", "shared-name")
	if string(a) != "a" || string(b) != "b" {
		t.Fatalf("bucket isolation broken: %q %q", a, b)
	}
	keysT, _ := v.ListRecords("tokens")
	keysE, _ := v.ListRecords("evidence")
	if len(keysT) != 1 || len(keysE) != 1 {
		t.Fatalf("list leak: %v %v", keysT, keysE)
	}
}

func TestVaultJournalMonotonic(t *testing.T) {
	v := openTestVault(t)
	for i := 0; i < 5; i++ {
		seq, err := v.AppendJournal([]byte{byte(i)})
		if err != nil {
			t.Fatal(err)
		}
		if seq != uint64(i+1) {
			t.Fatalf("seq = %d, want %d", seq, i+1)
		}
	}
	entries, err := v.ReadJournal()
	if err != nil || len(entries) != 5 {
		t.Fatalf("journal = %d entries err=%v", len(entries), err)
	}
}

// Crash-consistency proxy: a failed Update must leave no partial state.
func TestVaultFailedUpdateLeavesNoPartialWrites(t *testing.T) {
	v := openTestVault(t)
	err := v.db.Update(func(tx *boltTx) error {
		if err := tx.Bucket(bucketJournal).Put(u64be(999), []byte("ghost")); err != nil {
			return err
		}
		return errors.New("abort the transaction")
	})
	if err == nil {
		t.Fatal("expected abort error")
	}
	entries, _ := v.ReadJournal()
	if len(entries) != 0 {
		t.Fatalf("aborted transaction left %d entries — no atomicity", len(entries))
	}
}

func TestVaultAuditChainAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.db")

	v1, err := OpenVault(path)
	if err != nil {
		t.Fatal(err)
	}
	l1, err := NewVaultLog(v1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l1.Append("cmd-1", "r1"); err != nil {
		t.Fatal(err)
	}
	if _, err := l1.Append("cmd-2", "r2"); err != nil {
		t.Fatal(err)
	}
	if err := v1.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopen: the chain resumes and verifies.
	v2, err := OpenVault(path)
	if err != nil {
		t.Fatal(err)
	}
	defer v2.Close()
	l2, err := NewVaultLog(v2)
	if err != nil {
		t.Fatal(err)
	}
	vr, err := l2.Verify()
	if err != nil {
		t.Fatal(err)
	}
	if !vr.ValidAll || vr.Total != 2 || vr.Valid != 2 {
		t.Fatalf("verify = %+v", vr)
	}
	// Appending continues the chain.
	if _, err := l2.Append("cmd-3", "r3"); err != nil {
		t.Fatal(err)
	}
	if vr, _ := l2.Verify(); !vr.ValidAll || vr.Total != 3 {
		t.Fatalf("verify after append = %+v", vr)
	}
}

func TestVaultSchemaVersionRefusesNewer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.db")
	v, err := OpenVault(path)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a vault written by a future build.
	if err := v.MetaSet(metaSchemaVersion, u64be(SchemaVersion+1)); err != nil {
		t.Fatal(err)
	}
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenVault(path); err == nil {
		t.Fatal("newer schema version opened without error")
	} else if !strings.Contains(err.Error(), "newer than this build") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVaultRollbackStackSemantics(t *testing.T) {
	v := openTestVault(t)
	for _, id := range []string{"a", "b", "c"} {
		if _, err := v.RollbackPush([]byte(`{"id":"` + id + `"}`)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := v.RollbackPop(); err != nil {
		t.Fatal(err)
	}
	top, err := v.RollbackPeek()
	if err != nil || !strings.Contains(string(top), `"b"`) {
		t.Fatalf("peek = %s err=%v", top, err)
	}
	if err := v.RollbackRetainFailed([]byte(`{"id":"retained"}`)); err != nil {
		t.Fatal(err)
	}
	failed, _ := v.RollbackListFailed()
	if len(failed) != 1 {
		t.Fatalf("failed = %d", len(failed))
	}
}

// Cross-process safety: bbolt's flock means a second OpenVault on the
// same file must fail while the first handle is live.
func TestVaultLockExclusive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.db")
	v, err := OpenVault(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	if _, err := OpenVault(path); err == nil {
		t.Fatal("second concurrent OpenVault succeeded; expected lock error")
	}
}

// T1 acceptance: records are sealed before they reach the vault, so the
// raw file must not contain known plaintext. (The workspace layer seals
// every record/journal entry with the master key; the vault stores
// opaque ciphertext.)
func TestVaultNoPlaintextAtRest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.db")
	v, err := OpenVault(path)
	if err != nil {
		t.Fatal(err)
	}

	// Seal like the workspace layer does (AES-256-GCM, random nonce).
	key := make([]byte, 32)
	if _, err := crand.Read(key); err != nil {
		t.Fatal(err)
	}
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	if _, err := crand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	secret := []byte("TOPSECRET-PR-COOKIE-VALUE")
	sealed := gcm.Seal(nonce, nonce, secret, nil)

	if err := v.PutRecord("tokens", "x", sealed); err != nil {
		t.Fatal(err)
	}
	if _, err := v.AppendJournal(sealed); err != nil {
		t.Fatal(err)
	}
	_ = v.Close()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), string(secret)) {
		t.Fatal("plaintext found in vault.db — records must be sealed by the caller")
	}
}
