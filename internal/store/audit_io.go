package store

import "os"

func osReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func osWriteFile(path string, data []byte) error { return os.WriteFile(path, data, 0o600) }
