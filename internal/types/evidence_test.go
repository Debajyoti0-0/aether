package types

import (
	"testing"
	"time"
)

func TestEvidenceValidateConfidenceDiscipline(t *testing.T) {
	base := EvidenceRecord{
		ID: "e1", ActionID: "a1", Kind: "exec.azure.result",
		EpistemicClass: ClassObserved, Confidence: 1.0,
		CollectedAt: time.Now(), Method: "test",
	}

	if err := base.Validate(); err != nil {
		t.Fatalf("observed/1.0 rejected: %v", err)
	}

	// Observed with confidence != 1.0 is invalid.
	bad := base
	bad.Confidence = 0.9
	if err := bad.Validate(); err == nil {
		t.Error("ClassObserved with 0.9 accepted")
	}

	// Inferred must be strictly between 0 and 1.
	inf := base
	inf.EpistemicClass = ClassInferred
	inf.Confidence = 0.7
	if err := inf.Validate(); err != nil {
		t.Fatalf("inferred/0.7 rejected: %v", err)
	}
	inf.Confidence = 1.0
	if err := inf.Validate(); err == nil {
		t.Error("ClassInferred with 1.0 accepted")
	}

	// Predicted can never claim certainty — this is the T3 invariant.
	pred := base
	pred.EpistemicClass = ClassPredicted
	pred.Confidence = 1.0
	if err := pred.Validate(); err == nil {
		t.Fatal("ClassPredicted with confidence 1.0 accepted — prediction-as-certainty regression")
	}
	pred.Confidence = 0.66
	if err := pred.Validate(); err != nil {
		t.Fatalf("predicted/0.66 rejected: %v", err)
	}

	// Unknown carries zero confidence.
	unk := base
	unk.EpistemicClass = ClassUnknown
	unk.Confidence = 0
	if err := unk.Validate(); err != nil {
		t.Fatalf("unknown/0 rejected: %v", err)
	}
	unk.Confidence = 0.5
	if err := unk.Validate(); err == nil {
		t.Error("ClassUnknown with 0.5 accepted")
	}

	// Missing identity fields.
	noID := base
	noID.ID = ""
	if err := noID.Validate(); err == nil {
		t.Error("record without ID accepted")
	}
}

func TestEvidenceContentHashAndLabel(t *testing.T) {
	e := EvidenceRecord{Payload: []byte("payload")}
	if e.ContentHash() == "" || len(e.ContentHash()) != 64 {
		t.Errorf("content hash = %q", e.ContentHash())
	}
	if ClassObserved.Label() != "[OBSERVED]" || ClassPredicted.Label() != "[PREDICTED]" || ClassUnknown.Label() != "[UNKNOWN]" {
		t.Error("labels wrong")
	}
}
