package types

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// EpistemicClass separates what Aether knows from what it guesses
// (Stage 2 evidence seed; full provenance model is Stage 4).
// The classes must never be collapsed into a boolean.
type EpistemicClass string

const (
	ClassObserved  EpistemicClass = "observed"  // directly retrieved from the target
	ClassInferred  EpistemicClass = "inferred"  // derived from observations
	ClassPredicted EpistemicClass = "predicted" // model output — never certain
	ClassUnknown   EpistemicClass = "unknown"
)

// EvidenceRecord is the minimum evidence contract written by the
// Action spine (StageEvidence).
type EvidenceRecord struct {
	ID             string         `json:"id"`
	ActionID       string         `json:"action_id"`
	Kind           string         `json:"kind"`
	Target         string         `json:"target"`
	EpistemicClass EpistemicClass `json:"epistemic_class"`
	Confidence     float64        `json:"confidence"` // 0.0–1.0; 1.0 only for observed
	CollectedAt    time.Time      `json:"collected_at"`
	ExpiresAt      *time.Time     `json:"expires_at,omitempty"`
	Method         string         `json:"method"`
	Payload        []byte         `json:"payload,omitempty"`
}

// Validate enforces the confidence discipline:
//
//	ClassObserved  → confidence must be exactly 1.0
//	ClassInferred  → confidence must be in (0.0, 1.0)
//	ClassPredicted → confidence must be < 1.0 (predictions are never certain)
//	ClassUnknown   → confidence must be 0.0
func (e EvidenceRecord) Validate() error {
	if e.ID == "" || e.ActionID == "" || e.Kind == "" {
		return fmt.Errorf("evidence record requires id, action_id, and kind")
	}
	switch e.EpistemicClass {
	case ClassObserved:
		if e.Confidence != 1.0 {
			return fmt.Errorf("evidence %s: ClassObserved requires confidence 1.0, got %v", e.ID, e.Confidence)
		}
	case ClassInferred:
		if e.Confidence <= 0 || e.Confidence >= 1.0 {
			return fmt.Errorf("evidence %s: ClassInferred requires 0 < confidence < 1, got %v", e.ID, e.Confidence)
		}
	case ClassPredicted:
		if e.Confidence >= 1.0 {
			// Predictions must never be represented as certainty. This
			// is a programming error and fails loudly at the point of
			// use (records are rejected, reports refuse to render).
			return fmt.Errorf("evidence %s: ClassPredicted cannot carry confidence 1.0", e.ID)
		}
		if e.Confidence < 0 {
			return fmt.Errorf("evidence %s: negative confidence", e.ID)
		}
	case ClassUnknown:
		if e.Confidence != 0 {
			return fmt.Errorf("evidence %s: ClassUnknown requires confidence 0, got %v", e.ID, e.Confidence)
		}
	default:
		return fmt.Errorf("evidence %s: unknown epistemic class %q", e.ID, e.EpistemicClass)
	}
	if e.CollectedAt.IsZero() {
		return fmt.Errorf("evidence %s: collected_at is required", e.ID)
	}
	return nil
}

// ContentHash returns the SHA-256 of the payload for integrity checks.
func (e EvidenceRecord) ContentHash() string {
	h := sha256.Sum256(e.Payload)
	return hex.EncodeToString(h[:])
}

// Label renders the epistemic class as a report prefix.
func (c EpistemicClass) Label() string {
	switch c {
	case ClassObserved:
		return "[OBSERVED]"
	case ClassInferred:
		return "[INFERRED]"
	case ClassPredicted:
		return "[PREDICTED]"
	default:
		return "[UNKNOWN]"
	}
}
