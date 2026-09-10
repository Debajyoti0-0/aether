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
