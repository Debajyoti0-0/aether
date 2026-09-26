package cli

import "testing"

// Regression (Stage 44-R D-001): the doctor workspace roundtrip must close
// the workspace handle before Delete. On Windows an open vault.db file
// cannot be unlinked, so an unclosed handle made this check fail on every
// run. Run with -count=1; on Windows it must pass without unlink errors.
func TestWorkspaceRoundtripCheckClosesBeforeDelete(t *testing.T) {
	c := workspaceRoundtripCheck()
	if !c.ok {
		t.Fatalf("workspace roundtrip check failed: %s", c.detail)
	}
}
