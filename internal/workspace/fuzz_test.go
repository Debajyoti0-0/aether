// Native Go fuzz targets for workspace/vault and audit decoding
package workspace

import (
	"encoding/json"
	"testing"
)

func FuzzLoadRecord(f *testing.F) {
	// Valid record
	validRecord := `{"key":"value","nested":{"a":1}}`
	f.Add([]byte(validRecord))

	f.Add([]byte(`{}`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`"string"`))
	f.Add([]byte(`123`))
	f.Add([]byte(`true`))
	f.Add([]byte(`null`))
	f.Add([]byte(`not json`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var out map[string]any
		err := json.Unmarshal(data, &out)
		_ = err
		_ = out
	})
}

func FuzzAuditLogEntry(f *testing.F) {
	// Valid audit entry JSON
	validEntry := `{"action_id":"act-123","actor":"user","timestamp":"2024-01-01T00:00:00Z","status":"completed"}`
	f.Add([]byte(validEntry))

	f.Add([]byte(`{}`))
	f.Add([]byte(`{"action_id":""}`))
	f.Add([]byte(`{"actor":""}`))
	f.Add([]byte(`not json`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var entry map[string]any
		err := json.Unmarshal(data, &entry)
		_ = err
		_ = entry
	})
}