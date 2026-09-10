package exec

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

var amzTimeNow = time.Now

// Sign applies AWS Signature Version 4 to req using body as the payload.
func (s *sigV4) Sign(req *http.Request, body []byte) error {
	now := amzTimeNow().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	req.Header.Set("X-Amz-Date", amzDate)
	if s.SessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", s.SessionToken)
	}
	req.Header.Set("Host", req.URL.Host)

	payloadHash := sha256Hex(body)

	signedHeaders, canonicalHeaders := s.canonicalHeaders(req)
	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI(req),
		canonicalQuery(req),
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	scope := strings.Join([]string{dateStamp, s.Region, s.Service, "aws4_request"}, "/")
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	signingKey := hmacChain(
		[]byte("AWS4"+s.SecretKey),
		[][]byte{[]byte(dateStamp), []byte(s.Region), []byte(s.Service), []byte("aws4_request")},
	)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.AccessKey, scope, signedHeaders, signature))

	return nil
}

func (s *sigV4) canonicalHeaders(req *http.Request) (signedHeaders, headers string) {
	lower := map[string][]string{}
	for k, v := range req.Header {
		lower[strings.ToLower(k)] = v
	}
	lower["host"] = []string{req.URL.Host}

	keys := make([]string, 0, len(lower))
	for k := range lower {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	for _, k := range keys {
		vals := lower[k]
		trimmed := make([]string, len(vals))
		for i, v := range vals {
			trimmed[i] = collapseSpaces(v)
		}
		buf.WriteString(k)
		buf.WriteString(":")
		buf.WriteString(strings.Join(trimmed, ","))
		buf.WriteString("\n")
	}

	return strings.Join(keys, ";"), buf.String()
}

func collapseSpaces(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

func canonicalURI(req *http.Request) string {
	path := req.URL.EscapedPath()
	if path == "" {
		return "/"
	}
	return path
}

func canonicalQuery(req *http.Request) string {
	q := req.URL.Query()
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		vals := q[k]
		sort.Strings(vals)
		for _, v := range vals {
			pairs = append(pairs, uriEscape(k)+"="+uriEscape(v))
		}
	}
	return strings.Join(pairs, "&")
}

func uriEscape(s string) string {
	const hexDigits = "0123456789ABCDEF"
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' {
			out.WriteByte(c)
		} else {
			out.WriteByte('%')
			out.WriteByte(hexDigits[c>>4])
			out.WriteByte(hexDigits[c&0x0f])
		}
	}
	return out.String()
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func hmacChain(key []byte, parts [][]byte) []byte {
	for _, p := range parts {
		key = hmacSHA256(key, p)
	}
	return key
}
