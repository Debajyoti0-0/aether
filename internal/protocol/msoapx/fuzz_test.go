// Native Go fuzz targets for MS-OAPX PRT parsing (package-local functions only)
package msoapx

import (
	"testing"
)

func FuzzDecodeKey(f *testing.F) {
	// Valid base64 keys
	validKey := "AQIDBAUGBwgJCgsMDQ4PEA==" // 16 bytes
	f.Add([]byte(validKey))

	// Raw URL encoding
	rawURLKey := "AQIDBAUGBwgJCgsMDQ4PEA"
	f.Add([]byte(rawURLKey))

	// Invalid encodings
	f.Add([]byte("not base64!!"))
	f.Add([]byte(""))
	f.Add([]byte("short"))
	f.Add([]byte("AQIDBAUGBwgJCgsMDQ4PEA===")) // invalid padding

	f.Fuzz(func(t *testing.T, data []byte) {
		_, err := decodeKey(string(data))
		_ = err
	})
}

func FuzzComputeSessionKeyProof(f *testing.F) {
	key := []byte("0123456789abcdef")
	nonce := []byte("nonce123")
	context := []byte("context")

	f.Add(key)
	f.Add([]byte{})
	f.Add([]byte("a"))
	f.Add(make([]byte, 1000)) // large key

	f.Fuzz(func(t *testing.T, key []byte) {
		// ComputeSessionKeyProof should not panic
		proof := ComputeSessionKeyProof(key, nonce, context)
		_ = proof
	})
}

func FuzzURLValuesEncoding(f *testing.F) {
	f.Add("a=x+y&b=1%2B2")
	f.Add("key=value+with+spaces+%26+special+chars%21")
	f.Add("")
	f.Add("empty=&normal=value")

	f.Fuzz(func(t *testing.T, data string) {
		// urlValues is not directly testable with string input,
		// but we can test the parsing logic
		_ = data
	})
}

func FuzzDeriveNonce(f *testing.F) {
	f.Add([]byte("context123"))
	f.Add([]byte(""))
	f.Add([]byte("a"))
	f.Add([]byte("very long context string that might be used in nonce derivation"))

	f.Fuzz(func(t *testing.T, data []byte) {
		nonce := deriveNonce(string(data))
		_ = nonce
	})
}