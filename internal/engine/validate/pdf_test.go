package validate

import (
	"strings"
	"testing"
)

func TestPDFGeneration(t *testing.T) {
	r := Executive("ClientX", execTestResults(), []int{30, 80}, "prt_exchange", "imds_hijack", "sp_hijack")

	pdfBytes, err := r.PDF()
	if err != nil {
		t.Fatalf("pdf: %v", err)
	}
	if len(pdfBytes) < 1000 {
		t.Errorf("pdf too small: %d bytes", len(pdfBytes))
	}
	// PDF magic.
	if !strings.HasPrefix(string(pdfBytes[:5]), "%PDF-") {
		t.Error("output is not a PDF")
	}
	// Contains the report title.
	if !strings.Contains(string(pdfBytes), "Aether Engagement Report") {
		t.Error("pdf missing title")
	}
}

func TestPDFMinimal(t *testing.T) {
	r := Executive("Lab", nil, nil)
	pdfBytes, err := r.PDF()
	if err != nil {
		t.Fatalf("pdf: %v", err)
	}
	if !strings.HasPrefix(string(pdfBytes[:5]), "%PDF-") {
		t.Error("output is not a PDF")
	}
}
