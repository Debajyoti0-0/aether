package spine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Debajyoti0-0/aether/internal/workspace"
)

func newWorkspaceForStateTest(dir string) (*workspace.Workspace, error) {
	return workspace.Create("StateTestWS", "test-pass")
}

func testContext() context.Context { return context.Background() }

// stateTestMutation is a minimal spine.Mutation double.
type stateTestMutation struct {
	fail bool
}

func (m *stateTestMutation) Kind() string   { return "exec.test" }
func (m *stateTestMutation) Target() string { return "state-target" }
func (m *stateTestMutation) BeforeState() ([]byte, error) {
	return nil, ErrStateUnavailable
}
func (m *stateTestMutation) Execute(ctx context.Context) (string, error) {
	if m.fail {
		return "", errors.New("injected failure")
	}
	return "op-1", nil
}
func (m *stateTestMutation) AfterState() ([]byte, error) {
	if m.fail {
		return nil, ErrStateUnavailable
	}
	return []byte(`{"ok":true}`), nil
}
func (m *stateTestMutation) UndoRecipe() *UndoSpec { return nil }

// TestSpineFailureStateSeq: an execution failure terminates in `failed`
// via the legal executing → failed edge.
func TestSpineFailureStateSeq(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AETHER_CONFIG_DIR", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("AppData", dir)
	t.Setenv("HOME", dir)

	w, err := workspace.Create("StateFailWS", "test-pass")
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	s := New(w)
	res, err := s.Run(testContext(), &Action{
		Kind: "exec.test", Target: "t-2", Actor: "test",
		Mutation:     &stateTestMutation{fail: true},
		ApprovalMode: ApprovalAuto,
	})
	if err == nil {
		t.Fatal("expected failure")
	}
	got := stateSeqString(res.StateSeq)
	if !strings.HasSuffix(got, "executing>failed") {
		t.Errorf("state seq = %s, want suffix executing>failed", got)
	}
}
