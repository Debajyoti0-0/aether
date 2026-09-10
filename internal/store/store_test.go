package store

import (
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "aether.json")

	cfg := DefaultConfig()
	cfg.LogLevel = "debug"
	cfg.Azure.SubscriptionID = "sub-1"

	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.LogLevel != "debug" {
		t.Errorf("log level = %q", loaded.LogLevel)
	}
	if loaded.Azure.SubscriptionID != "sub-1" {
		t.Errorf("sub id = %q", loaded.Azure.SubscriptionID)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("load default: %v", err)
	}
	if cfg.MaxRiskThreshold != 50 {
		t.Errorf("risk threshold = %d", cfg.MaxRiskThreshold)
	}
}

func TestConfigValidation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxRiskThreshold = 150
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for risk threshold")
	}
}

func TestBoltStore(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	type token struct {
		Access string `json:"access"`
	}

	if err := s.PutJSON(BucketTokens, "t1", token{Access: "abc"}); err != nil {
		t.Fatalf("put: %v", err)
	}

	var out token
	if err := s.GetJSON(BucketTokens, "t1", &out); err != nil {
		t.Fatalf("get: %v", err)
	}
	if out.Access != "abc" {
		t.Errorf("access = %q", out.Access)
	}

	list, err := s.List(BucketTokens)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("list len = %d", len(list))
	}

	if err := s.Delete(BucketTokens, "t1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := s.GetJSON(BucketTokens, "t1", &out); err == nil {
		t.Error("expected missing key error")
	}
}

func TestRedact(t *testing.T) {
	fields := Redact(
		zap.String("token", "supersecret"),
		zap.String("user", "admin"),
	)
	if fields[0].String != "[REDACTED]" {
		t.Errorf("token field = %q, want [REDACTED]", fields[0].String)
	}
	if fields[1].String != "admin" {
		t.Errorf("user field = %q", fields[1].String)
	}
}
