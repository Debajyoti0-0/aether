package token

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/Debajyoti0-0/aether/internal/types"
)

// Extract scans arbitrary dump text (registry exports, Credential
// Manager dumps, clipboard captures, AAD Broker plugin dumps) for
// PRT-shaped JSON objects and raw base64 PRT cookies, returning
// normalized PRTs.
type Extract struct{}

// NewExtract builds a PRT extractor.
func NewExtract() *Extract { return &Extract{} }

// jsonPRTPattern matches PRT-shaped JSON objects inside larger text.
var jsonPRTPattern = regexp.MustCompile(
	`\{[^{}]*"(?i:cookie|prt_cookie|refresh_token)"\s*:\s*"[^"]+"[^{}]*\}`)

// base64CookiePattern matches long base64url PRT cookie blobs
// (typically 700+ chars; dots separate the 0.AAA… segments).
var base64CookiePattern = regexp.MustCompile(`[A-Za-z0-9_.-]{600,}`)

// ExtractPRTs pulls every PRT-shaped record out of a dump.
func (e *Extract) ExtractPRTs(dump []byte) ([]types.PRT, error) {
	var out []types.PRT
	seen := map[string]bool{}

	// Pass 1: structured JSON objects.
	for _, m := range jsonPRTPattern.FindAll(dump, -1) {
		prt, err := ParsePRT(m)
		if err != nil {
			continue
		}
		if !seen[prt.Cookie] {
			seen[prt.Cookie] = true
			out = append(out, *prt)
		}
	}

	// Pass 2: raw long-base64 blobs (unstructured dumps).
	for _, m := range base64CookiePattern.FindAll(dump, -1) {
		cookie := string(m)
		if seen[cookie] {
			continue
		}
		if !looksLikePRTCookie(cookie) {
			continue
		}
		seen[cookie] = true
		out = append(out, types.PRT{Cookie: cookie})
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("no PRT-shaped records found in dump")
	}
	return out, nil
}

// looksLikePRTCookie applies shape heuristics: real PRT cookies start
// with "0." or carry high character diversity (≥40 distinct chars in
// the first 200) — repeated ASCII base64 text never reaches that.
func looksLikePRTCookie(s string) bool {
	if strings.HasPrefix(s, "0.") {
		return true
	}
	seen := map[byte]bool{}
	limit := min(200, len(s))
	for i := 0; i < limit; i++ {
		seen[s[i]] = true
	}
	return len(seen) >= 40
}

// ExtractFromFile reads a dump file and extracts PRTs.
func (e *Extract) ExtractFromFile(path string) ([]types.PRT, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read dump %s: %w", path, err)
	}
	return e.ExtractPRTs(data)
}

// Marshal renders extracted PRTs as a JSON array file.
func MarshalPRTs(prts []types.PRT) ([]byte, error) {
	data, err := json.MarshalIndent(prts, "", "  ")
	if err != nil {
		return nil, err
	}
	return data, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
