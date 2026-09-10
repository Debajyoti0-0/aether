package workspace

import "os"

// osReadFile/osWriteFile are indirections for testability.
var (
	osReadFile  = os.ReadFile
	osWriteFile = func(path string, data []byte) error {
		return os.WriteFile(path, data, 0o600)
	}
)
