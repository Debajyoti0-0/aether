// Package web implements Aether's read-only operator dashboard: a
// server-rendered HTML view over the signed audit chain and the identity
// graph, plus a WebGL renderer and a client-side WASM audit verifier.
//
// The package is read-only by construction. Every route is registered for GET
// only (Go 1.22 method routing), so a POST, PUT, PATCH or DELETE can never
// reach a handler. There is no mutation endpoint, no CSRF surface, and no
// control-plane path from the browser into the spine: the CLI remains the sole
// execution path.
package web

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/Debajyoti0-0/aether/internal/store"
)

// GenesisHash is the chain anchor for an empty log. It mirrors
// store.GenesisHash and is duplicated here so the client-side verifier's
// expectations can be asserted without importing the store package.
const GenesisHash = store.GenesisHash

// AuditEntry is the wire shape of one signed audit record. Field names and
// JSON keys MUST match store.Entry exactly, because the WASM verifier and any
// external proof tool parse this. Changing a key here silently invalidates
// every exported proof bundle, so this struct is deliberately a mirror and not
// a redefinition of behaviour.
type AuditEntry struct {
	Seq       int64  `json:"seq"`
	Timestamp string `json:"timestamp"`
	Command   string `json:"command"`
	Result    string `json:"result,omitempty"`
	PrevHash  string `json:"prev_hash"`
	Hash      string `json:"hash"`
	Signature string `json:"signature"`
}

// entryFromStore converts a store.Entry to its wire form. The timestamp is
// re-emitted in exactly the layout store.computeHash hashed, so a client that
// recomputes the hash from this JSON reproduces the server's value. See
// ComputeEntryHash for the format contract.
func entryFromStore(e store.Entry) AuditEntry {
	return AuditEntry{
		Seq:       e.Seq,
		Timestamp: e.Timestamp.UTC().Format(rfc3339NanoLayout),
		Command:   e.Command,
		Result:    e.Result,
		PrevHash:  e.PrevHash,
		Hash:      e.Hash,
		Signature: e.Signature,
	}
}

// rfc3339NanoLayout matches Go's time.RFC3339Nano. It is restated here so the
// wire contract is visible at the point of use rather than only in the store
// package.
const rfc3339NanoLayout = "2006-01-02T15:04:05.999999999Z07:00"

// HashPreimage returns the exact byte string store.computeHash signs over for
// the given field values. The WASM verifier must reproduce this byte for byte;
// the format is therefore frozen here and covered by a cross-implementation
// test against the real store implementation.
//
// Format: "<seq>|<timestamp>|<command>|<result>|<prev_hash>"
func HashPreimage(seq int64, timestamp, command, result, prevHash string) string {
	return fmt.Sprintf("%d|%s|%s|%s|%s", seq, timestamp, command, result, prevHash)
}

// ComputeEntryHash reproduces store.computeHash for a wire-format entry. It
// exists so the dashboard's own "would the client agree with us?" check and the
// tests can compute the expected value without duplicating the store's
// timestamp normalisation rules.
func ComputeEntryHash(e AuditEntry) string {
	return sha256Hex([]byte(HashPreimage(e.Seq, e.Timestamp, e.Command, e.Result, e.PrevHash)))
}

// VerifyChainResult is the server-side mirror of the WASM verifier's result
// shape. The two must agree field for field: the dashboard's green banner is
// only meaningful if an independent implementation reaches the same verdict.
type VerifyChainResult struct {
	Valid        bool     `json:"valid"`
	EntryCount   int      `json:"entry_count"`
	TamperedSeq  []int64  `json:"tampered_seq,omitempty"`
	BrokenAt     *int64   `json:"broken_chain_at,omitempty"`
	ValidCount   int      `json:"valid_count"`
	Error        string   `json:"error,omitempty"`
	PubKeyFinger string   `json:"pubkey_fingerprint"`
	HeadHash     string   `json:"head_hash"`
	VerifiedBy   string   `json:"verified_by"`
	Notes        []string `json:"notes,omitempty"`
}

// verifyPubEd25519 checks the chain with a caller-supplied public key. It is
// separated from log-level verification because the dashboard serves the public
// key to the browser, and a chain verified against a key the server chose is
// not an independent check.
func verifyPubEd25519(entries []AuditEntry, pub ed25519.PublicKey) VerifyChainResult {
	res := VerifyChainResult{
		Valid:        true,
		EntryCount:   len(entries),
		PubKeyFinger: FingerprintPublicKey(pub),
		VerifiedBy:   "go:internal/web (mirrors internal/store verification)",
	}
	if len(entries) == 0 {
		res.HeadHash = GenesisHash
		return res
	}

	prev := GenesisHash
	for _, e := range entries {
		if e.PrevHash != prev {
			if res.BrokenAt == nil {
				b := e.Seq
				res.BrokenAt = &b
			}
			res.Valid = false
			res.Notes = append(res.Notes, fmt.Sprintf(
				"seq %d: prev_hash %s does not match expected %s", e.Seq, short(e.PrevHash), short(prev)))
		}
		prev = e.Hash

		if ComputeEntryHash(e) != e.Hash {
			res.TamperedSeq = append(res.TamperedSeq, e.Seq)
			res.Valid = false
			continue
		}
		sig, err := base64.StdEncoding.DecodeString(e.Signature)
		if err != nil {
			res.TamperedSeq = append(res.TamperedSeq, e.Seq)
			res.Valid = false
			res.Notes = append(res.Notes, fmt.Sprintf("seq %d: signature is not valid base64", e.Seq))
			continue
		}
		if !ed25519.Verify(pub, []byte(e.Hash), sig) {
			res.TamperedSeq = append(res.TamperedSeq, e.Seq)
			res.Valid = false
			res.Notes = append(res.Notes, fmt.Sprintf("seq %d: Ed25519 signature does not verify", e.Seq))
			continue
		}
		res.ValidCount++
	}
	res.HeadHash = entries[len(entries)-1].Hash
	return res
}

// FingerprintPublicKey renders a stable, human-checkable identity for a signing
// key. The dashboard shows this next to the verifier banner so an operator can
// confirm in-browser that the key the browser used is the key they expect.
func FingerprintPublicKey(pub ed25519.PublicKey) string {
	sum := sha256Sum(pub)
	groups := make([]string, 0, 4)
	for i := 0; i < 4; i++ {
		groups = append(groups, hex.EncodeToString(sum[i*4:i*4+4]))
	}
	return strings.Join(groups, "-")
}

// PublicKeyFromSeed derives the Ed25519 public key from a base64 seed, matching
// how store persists and restores the audit key.
func PublicKeyFromSeed(seedB64 string) (ed25519.PublicKey, error) {
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(seedB64))
	if err != nil {
		return nil, fmt.Errorf("decode audit key seed: %w", err)
	}
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("audit key seed is %d bytes, want %d", len(seed), ed25519.SeedSize)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	return priv.Public().(ed25519.PublicKey), nil
}

// short abbreviates a hash for display without implying it is a full hash.
func short(h string) string {
	if len(h) <= 12 {
		return h
	}
	return h[:6] + "…" + h[len(h)-4:]
}

// sortEntries orders a chain oldest-first by sequence. Sorting (rather than
// trusting file order) means a reordered export is detected as a linkage
// failure instead of being silently accepted in the order supplied.
func sortEntries(entries []AuditEntry) {
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Seq < entries[j].Seq })
}
