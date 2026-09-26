package web

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"time"
)

// ProofBundle is a self-contained, independently verifiable proof that one audit
// entry is authentic and correctly linked to its predecessor.
//
// It is deliberately more than a copy of the entry. A recipient who has only
// the bundle must be able to check, without trusting Aether and without network
// access, both that the entry's own signature is valid and that the entry sits
// at the claimed position in the chain. That requires the predecessor's hash,
// so the bundle carries it.
type ProofBundle struct {
	Format      string          `json:"format"`
	FormatVer   int             `json:"format_version"`
	Generated   time.Time       `json:"generated"`
	Entry       AuditEntry      `json:"entry"`
	Predecessor *PredecessorRef `json:"predecessor"`
	PublicKey   KeyRef          `json:"public_key"`
	Claim       ProofClaim      `json:"claim"`
	HowToVerify []string        `json:"how_to_verify"`
}

// PredecessorRef identifies the entry this one links to.
type PredecessorRef struct {
	Seq       int64  `json:"seq"`
	Hash      string `json:"hash"`
	IsGenesis bool   `json:"is_genesis"`
	Known     bool   `json:"known_to_serializer"`
}

// KeyRef carries the verification key in every encoding a verifier might need.
type KeyRef struct {
	Algorithm   string `json:"algorithm"`
	Base64      string `json:"base64"`
	Hex         string `json:"hex"`
	Fingerprint string `json:"fingerprint"`
}

// ProofClaim states precisely what is being asserted, so a verifier can tell a
// bundle that proves one entry from one that proves a whole chain. This bundle
// proves a single entry plus its linkage; it does not by itself prove that no
// later entry was appended or removed, because that requires the full chain.
type ProofClaim struct {
	Statement      string `json:"statement"`
	HashPreimage   string `json:"hash_preimage"`
	RecomputedHash string `json:"recomputed_hash"`
	HashMatches    bool   `json:"hash_matches"`
	SignatureValid bool   `json:"signature_valid"`
	LinkageValid   bool   `json:"linkage_valid"`
	Scope          string `json:"scope"`
	ScopeLimit     string `json:"scope_limit"`
}

// ErrNoSuchEntry is returned for a sequence number absent from the chain.
var ErrNoSuchEntry = fmt.Errorf("no such audit entry")

// ProofBundle builds the bundle for one entry.
func (s *Server) ProofBundle(seq int64) (*ProofBundle, error) {
	entries := s.src.Entries()
	var found *AuditEntry
	for i := range entries {
		if entries[i].Seq == seq {
			found = &entries[i]
			break
		}
	}
	if found == nil {
		return nil, fmt.Errorf("%w: seq %d", ErrNoSuchEntry, seq)
	}
	pub := s.src.PublicKey()
	if pub == nil {
		return nil, fmt.Errorf("no signing public key is available, so no proof can be produced")
	}

	b := &ProofBundle{
		Format:    "aether.audit.proof",
		FormatVer: 1,
		Generated: time.Now().UTC(),
		Entry:     *found,
		PublicKey: KeyRef{Algorithm: "Ed25519", Base64: base64Std(pub), Hex: hexOf(pub), Fingerprint: FingerprintPublicKey(pub)},
		HowToVerify: []string{
			`1. Decode public_key.base64 (standard base64) to 32 bytes.`,
			`2. Recompute: sha256( claim.hash_preimage ) rendered as lowercase hex. Compare with entry.hash. They must match, or the entry was altered.`,
			`3. Decode entry.signature (standard base64) to 64 bytes. Verify it as an Ed25519 signature over the ASCII bytes of entry.hash using the public key.`,
			`4. Confirm entry.prev_hash equals predecessor.hash, and that predecessor.seq is entry.seq - 1.`,
			`5. With the full chain, additionally confirm that predecessor.hash is itself the hash of the predecessor entry.`,
			`Go equivalent: aether export verify-evidence --proof <this file>`,
		},
	}

	pre := PredecessorRef{Hash: found.PrevHash}
	if found.PrevHash == GenesisHash {
		pre.IsGenesis = true
		pre.Seq = 0
	} else {
		pre.Seq = found.Seq - 1
		if p, ok := s.entryAt(found.Seq - 1); ok {
			pre.Known = true
			pre.Hash = p.Hash
		} else {
			// The predecessor entry is not in the chain we were given. The
			// linkage can still be stated but not confirmed, and the bundle
			// says so rather than implying a full proof.
			pre.Hash = found.PrevHash
		}
	}
	b.Predecessor = &pre

	preimage := HashPreimage(found.Seq, found.Timestamp, found.Command, found.Result, found.PrevHash)
	recomputed := sha256Hex([]byte(preimage))
	sigOK := false
	if sig, err := b64.DecodeString(found.Signature); err == nil {
		sigOK = ed25519.Verify(pub, []byte(found.Hash), sig)
	}
	linkOK := pre.IsGenesis || (pre.Known && pre.Hash == found.PrevHash)

	b.Claim = ProofClaim{
		Statement:      fmt.Sprintf("audit entry seq %d is authentic under key %s and links to its predecessor", found.Seq, FingerprintPublicKey(pub)),
		HashPreimage:   preimage,
		RecomputedHash: recomputed,
		HashMatches:    recomputed == found.Hash,
		SignatureValid: sigOK,
		LinkageValid:   linkOK,
		Scope:          "single entry: its own hash, its own Ed25519 signature, and its linkage to the immediately preceding entry",
		ScopeLimit:     "this does NOT prove that later entries were not appended or removed, and does NOT prove that the signing key is the engagement's key. Confirming either requires the full exported chain and an out-of-band key fingerprint check.",
	}
	return b, nil
}

// hexOf renders bytes as lowercase hex.
func hexOf(b []byte) string { return hex.EncodeToString(b) }
