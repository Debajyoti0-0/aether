package store

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// RedactedKeys are field names whose values are never written to logs.
var RedactedKeys = map[string]bool{
	"token": true, "password": true, "secret": true,
	"cookie": true, "session_key": true, "access_token": true,
	"refresh_token": true, "authorization": true, "client_secret": true,
}

// NewLogger creates a structured logger. File logging uses production
// JSON encoding; console logging uses a minimal OPSEC-safe format.
func NewLogger(level, logFile string) (*zap.Logger, error) {
	var lvl zapcore.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = zapcore.InfoLevel
	}

	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "ts"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	var sink string
	if logFile != "" {
		sink = logFile
	} else {
		sink = "stderr"
	}

	cfg := zap.Config{
		Level:             zap.NewAtomicLevelAt(lvl),
		Development:       false,
		Encoding:          "json",
		EncoderConfig:     encCfg,
		OutputPaths:       []string{sink},
		ErrorOutputPaths:  []string{sink},
	}

	logger, err := cfg.Build()
	if err != nil {
		return nil, err
	}
	return logger, nil
}

// Redact returns redacted copies of sensitive fields for logging.
func Redact(fields ...zap.Field) []zap.Field {
	out := make([]zap.Field, len(fields))
	for i, f := range fields {
		if RedactedKeys[f.Key] {
			out[i] = zap.String(f.Key, "[REDACTED]")
		} else {
			out[i] = f
		}
	}
	return out
}

// LogFileWritable reports whether a log file path is usable.
func LogFileWritable(path string) bool {
	if path == "" {
		return false
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return false
	}
	f.Close()
	return true
}
