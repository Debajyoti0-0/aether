package workspace

import "testing"

// closeLater registers the workspace vault for release at test cleanup
// so t.TempDir can remove the directory (bbolt holds an flock until
// Close).
func closeLater(t *testing.T, w *Workspace) {
	t.Helper()
	t.Cleanup(func() { _ = w.Close() })
}
