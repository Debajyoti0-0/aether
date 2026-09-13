// Native Go fuzz targets for token parsing (PRT, OAuth, JWT)
package token

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/Debajyoti0-0/aether/internal/types"
)

func FuzzParsePRT(f *testing.F) {
	validPRT := `{"cookie":"0.AAAA","device_id":"d1","tenant_id":"t1","user_id":"u1","session_key":"` + base64.StdEncoding.EncodeToString([]byte("0123456789abcdef")) + `"}`
	f.Add([]byte(validPRT))

	// Missing fields
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"cookie":"x"}`))
	f.Add([]byte(`{"tenant_id":"t"}`))
	f.Add([]byte(`{"cookie":"x","tenant_id":"t"}`))
	f.Add([]byte(`{"cookie":"x","tenant_id":"t","session_key":"invalid"}`))
	f.Add([]byte(`{"cookie":"x","tenant_id":"t","session_key":""}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		_, err := ParsePRT(data)
		_ = err
	})
}

func FuzzParseOAuthTokens(f *testing.F) {
	validTokens := `{"access_token":"ya29.xxx","refresh_token":"1//xxx","token_type":"Bearer","expires_in":3600,"scope":"https://www.googleapis.com/auth/userinfo.profile"}`
	f.Add([]byte(validTokens))

	// Minimal
	f.Add([]byte(`{"access_token":"x"}`))
	f.Add([]byte(`{}`))

	// Invalid JSON
	f.Add([]byte(`{not json}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var tokens types.OAuthTokens
		err := json.Unmarshal(data, &tokens)
		_ = err
		_ = tokens
	})
}

func FuzzValidatePRT(f *testing.F) {
	f.Add([]byte("AQIDBAUGBwgJCgsMDQ4PEA=="))
	f.Add([]byte(""))
	f.Add([]byte("invalid!!"))
	f.Add([]byte("short"))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Test validation with various session key encodings
		prt := &types.PRT{
			Cookie:     "cookie",
			TenantID:   "t",
			SessionKey: string(data),
		}
		// Just verify no panic
		_ = prt.IsZero()
		_, _ = base64.StdEncoding.DecodeString(prt.SessionKey)
	})
}