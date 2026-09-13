// Native Go fuzz targets for IMDS response parsing
package exec

import (
	"encoding/json"
	"testing"
)

func FuzzParseIMDSIdentityToken(f *testing.F) {
	validToken := `{"access_token":"eyJ0eXAi...","client_id":"xxxx","expires_in":"3600","expires_on":"1234567890","resource":"https://management.azure.com/","token_type":"Bearer"}`
	f.Add([]byte(validToken))

	// Minimal
	f.Add([]byte(`{"access_token":"x"}`))
	f.Add([]byte(`{}`))

	// Invalid JSON
	f.Add([]byte(`{not json}`))

	// Extra fields
	f.Add([]byte(`{"access_token":"x","extra_field":"value"}`))

	// Missing expires_in
	f.Add([]byte(`{"access_token":"x","token_type":"Bearer"}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var raw IMDSIdentityToken
		err := json.Unmarshal(data, &raw)
		_ = err
		_ = raw
	})
}

func FuzzParseInstanceMetadata(f *testing.F) {
	validMetadata := `{"compute":{"vmId":"xxx","name":"myvm","resourceGroupName":"myrg","subscriptionId":"xxx","location":"eastus"}}`
	f.Add([]byte(validMetadata))

	f.Add([]byte(`{}`))
	f.Add([]byte(`{"compute":{}}`))
	f.Add([]byte(`{"compute":{"vmId":"x","name":"y"}}`))
	f.Add([]byte(`not json`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var doc map[string]any
		err := json.Unmarshal(data, &doc)
		_ = err
		_ = doc
	})
}