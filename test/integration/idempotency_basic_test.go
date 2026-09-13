//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"testing"

	"github.com/Debajyoti0-0/aether/internal/workspace"
)

// Force use of workspace import
var _ workspace.Workspace

func TestIdempotencyBasic(t *testing.T) {
	t.Parallel()
	ws := newWorkspace(t, "idempotency-basic")

	// Test basic put/get
	err := ws.IdempotencyPut("test-key", []byte(`{"status":"completed","result":"test"}`))
	if err != nil {
		t.Fatalf("put failed: %v", err)
	}

	data, err := ws.IdempotencyGet("test-key")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	var record map[string]interface{}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if record["status"] != "completed" {
		t.Fatalf("expected completed, got %v", record["status"])
	}

	// Test PutIfAbsent with new key
	existing, err := ws.IdempotencyPutIfAbsent("new-key", []byte(`{"status":"pending"}`))
	if err != nil {
		t.Fatalf("putifabsent failed: %v", err)
	}
	if existing != nil {
		t.Fatalf("expected nil for new key, got %v", existing)
	}

	// Test PutIfAbsent with existing key
	existing2, err := ws.IdempotencyPutIfAbsent("test-key", []byte(`{"status":"pending"}`))
	if err != nil {
		t.Fatalf("putifabsent failed: %v", err)
	}
	if existing2 == nil {
		t.Fatal("expected existing record for existing key")
	}

	// Verify the existing record is returned
	var record2 map[string]interface{}
	if err := json.Unmarshal(existing2, &record2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if record2["status"] != "completed" {
		t.Fatalf("expected completed, got %v", record2["status"])
	}
}