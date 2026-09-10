package validate

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
)

// PDF renders the executive report as a professionally formatted PDF
// document (pure Go — no external binaries).
func (r *ExecutiveReport) PDF() ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(false) // keeps content streams inspectable
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 18)

	// ---- Cover / title ----
	pdf.AddPage()
	pdf.SetFillColor(17, 17, 17)
	pdf.Rect(0, 0, 210, 42, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 22)
	pdf.SetXY(15, 14)
	pdf.Cell(0, 10, "Aether Engagement Report")
	pdf.SetFont("Arial", "", 11)
	pdf.SetXY(15, 27)
	pdf.Cell(0, 8, fmt.Sprintf("Workspace: %s   |   Generated: %s",
		r.Workspace, r.GeneratedAt.Format(time.RFC3339)))

	// ---- Executive summary ----
	pdf.SetTextColor(20, 20, 20)
	pdf.SetXY(15, 52)
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(0, 8, "Executive Summary")
	pdf.Ln(12)
	pdf.SetFont("Arial", "", 10.5)
	pdf.MultiCell(180, 5.5, r.Summary, "", "", false)

	// ---- KPI table ----
	pdf.Ln(4)
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(0, 8, "Key Performance Indicators")
	pdf.Ln(10)

	kpis := [][2]string{
		{"Paths analyzed", fmt.Sprint(r.KPIs.PathsAnalyzed)},
		{"Paths valid", fmt.Sprint(r.KPIs.PathsValid)},
		{"Success rate", fmt.Sprintf("%.1f%%", r.KPIs.SuccessRate)},
		{"Average risk", fmt.Sprintf("%d/100", r.KPIs.AvgRisk)},
		{"Peak risk", fmt.Sprintf("%d/100", r.KPIs.MaxRisk)},
		{"Critical findings", fmt.Sprint(r.KPIs.CriticalFindings)},
		{"Detection exposure", r.KPIs.DetectionExposure},
	}

	// Header row.
	pdf.SetFillColor(17, 17, 17)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(90, 7, "Metric", "1", 0, "", true, 0, "")
	pdf.CellFormat(90, 7, "Value", "1", 1, "", true, 0, "")

	pdf.SetTextColor(20, 20, 20)
	pdf.SetFont("Arial", "", 10)
	fill := false
	for _, kv := range kpis {
		if fill {
			pdf.SetFillColor(240, 240, 240)
		}
		align := ""
		pdf.CellFormat(90, 6.5, kv[0], "1", 0, align, fill, 0, "")
		pdf.CellFormat(90, 6.5, kv[1], "1", 1, align, fill, 0, "")
		fill = !fill
	}

	// ---- Top risks ----
	if len(r.TopRisks) > 0 {
		pdf.Ln(6)
		pdf.SetFont("Arial", "B", 14)
		pdf.Cell(0, 8, "Top Risks")
		pdf.Ln(10)
		pdf.SetFont("Arial", "", 10)
		for _, t := range r.TopRisks {
			pdf.SetTextColor(180, 30, 30)
			pdf.Cell(8, 5.5, "[!]")
			pdf.SetTextColor(20, 20, 20)
			pdf.MultiCell(172, 5.5, t, "", "", false)
		}
	}

	// ---- Remediation ----
	if len(r.Remediations) > 0 {
		pdf.AddPage()
		pdf.SetFont("Arial", "B", 14)
		pdf.Cell(0, 8, "Recommended Remediation")
		pdf.Ln(10)

		for _, rem := range r.Remediations {
			pdf.SetFont("Arial", "B", 11)
			pdf.SetTextColor(20, 20, 20)
			pdf.Cell(0, 6, fmt.Sprintf("[%s] %s", rem.Priority, rem.Title))
			pdf.Ln(6)
			pdf.SetFont("Arial", "I", 9)
			pdf.SetTextColor(90, 90, 90)
			pdf.Cell(0, 5, rem.Area)
			pdf.Ln(6)
			pdf.SetFont("Arial", "", 10)
			pdf.SetTextColor(20, 20, 20)
			pdf.MultiCell(180, 5.5, rem.Detail, "", "", false)
			pdf.Ln(3)
		}
	}

	// ---- MITRE ATT&CK technique mapping ----
	if len(r.MITRETechniques) > 0 {
		pdf.AddPage()
		pdf.SetFont("Arial", "B", 14)
		pdf.Cell(0, 8, "MITRE ATT&CK Techniques Exercised")
		pdf.Ln(10)

		// Simple heatmap grid: colored cells per technique.
		pdf.SetFont("Arial", "", 9)
		x, y := 15.0, float64(pdf.GetY())
		cellW, cellH := 34.0, 12.0
		perRow := 5
		for i, t := range r.MITRETechniques {
			col := i % perRow
			row := i / perRow
			cx := x + float64(col)*cellW
			cy := y + float64(row)*cellH
			pdf.SetFillColor(230, 57, 70)
			pdf.Rect(cx, cy, cellW-2, cellH-2, "F")
			pdf.SetTextColor(255, 255, 255)
			pdf.SetXY(cx, cy+2)
			pdf.Cell(cellW-2, 4, t)
			pdf.SetXY(cx, cy+7)
			pdf.SetFont("Arial", "", 7)
			pdf.Cell(cellW-2, 3, "exercised")
			pdf.SetFont("Arial", "", 9)
		}
	}

	// ---- Footer timestamp on every page ----
	_ = strings.TrimSpace // keep strings linked if templates change

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
