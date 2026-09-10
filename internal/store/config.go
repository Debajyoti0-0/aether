package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Debajyoti0-0/aether/internal/paths"
)

// Config represents Aether's configuration.
type Config struct {
	LogLevel string `json:"log_level"`
	LogFile  string `json:"log_file"`
	Timeout  int    `json:"timeout"`

	BrowserPreset    string `json:"browser_preset"`
	MaxRiskThreshold int    `json:"max_risk_threshold"`

	Azure struct {
		SubscriptionID string `json:"subscription_id"`
	} `json:"azure"`

	AWS struct {
		Region string `json:"region"`
	} `json:"aws"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	cfg := &Config{
		LogLevel:         "info",
		Timeout:          30,
		BrowserPreset:    "chrome",
		MaxRiskThreshold: 50,
	}
	cfg.AWS.Region = "us-east-1"
	return cfg
}

// Validate checks the config for obviously broken values.
func (c *Config) Validate() error {
	if c.MaxRiskThreshold < 0 || c.MaxRiskThreshold > 100 {
		return fmt.Errorf("max_risk_threshold must be 0-100, got %d", c.MaxRiskThreshold)
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got %d", c.Timeout)
	}
	if _, err := os.Stat(c.LogFile); c.LogFile != "" && err == nil && c.LogFile == filepath.Dir(c.LogFile) {
		return fmt.Errorf("log_file is a directory")
	}
	return nil
}

// DefaultLocations are the paths searched when no explicit config is
// given (OS-aware: cwd, user config dir, system dir).
func DefaultLocations() []string {
	return paths.ConfigFileLocations()
}

// LoadConfig loads from an explicit path, a discovered path, or defaults.
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		for _, loc := range DefaultLocations() {
			if _, err := os.Stat(loc); err == nil {
				path = loc
				break
			}
		}
	}

	if path == "" {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// SaveConfig writes the config as JSON to path.
func SaveConfig(path string, cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
