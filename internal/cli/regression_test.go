package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Charter fix (H1): serve cert revoke reads tsCertOperator but never
// registered the flag. This pins the registration for issue AND revoke.
func TestServeCertOperatorFlagRegistered(t *testing.T) {
	if serveCertIssueCmd.Flags().Lookup("operator") == nil {
		t.Error("serve cert issue: --operator not registered")
	}
	if serveCertRevokeCmd.Flags().Lookup("operator") == nil {
		t.Error("serve cert revoke: --operator not registered (charter fix H1)")
	}
}

// Charter fix (L2): runPassphrase was dead state. This pins the flag.
func TestRunCmdPassphraseFlagRegistered(t *testing.T) {
	if runCmd.Flags().Lookup("passphrase") == nil {
		t.Error("run: --passphrase not registered (charter fix L2)")
	}
}

// Charter fix (6): CA resolution — explicit --server-ca wins, teamserver
// root preferred over the cert dir.
func TestResolveServerCAFallback(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "ts")
	opCert := filepath.Join(root, "operators", "alice", "ops.crt")

	if got := resolveServerCA("/explicit/ca.crt", opCert, false); got != "/explicit/ca.crt" {
		t.Errorf("explicit --server-ca must win, got %q", got)
	}
	if got := resolveServerCA("/explicit/ca.crt", opCert, true); got != "/explicit/ca.crt" {
		t.Errorf("insecure must keep explicit CA, got %q", got)
	}
	// Root CA present → preferred.
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "teamserver-ca.crt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := resolveServerCA("", opCert, false); got != filepath.Join(root, "teamserver-ca.crt") {
		t.Errorf("teamserver root CA must be preferred, got %q", got)
	}
	// Root CA missing → cert dir fallback.
	if err := os.Remove(filepath.Join(root, "teamserver-ca.crt")); err != nil {
		t.Fatal(err)
	}
	if got := resolveServerCA("", opCert, false); got != filepath.Join(root, "operators", "alice", "teamserver-ca.crt") {
		t.Errorf("cert-dir fallback expected, got %q", got)
	}
}

// Charter fix (H3): runbook steps carry their own --workspace flag.
func TestWorkspaceFromLine(t *testing.T) {
	if got := workspaceFromLine([]string{"exec", "azure", "--workspace", "demo"}); got != "demo" {
		t.Errorf("space form: got %q", got)
	}
	if got := workspaceFromLine([]string{"exec", "azure", "--workspace=demo"}); got != "demo" {
		t.Errorf("= form: got %q", got)
	}
	if got := workspaceFromLine([]string{"exec", "azure"}); got != "" {
		t.Errorf("absent: got %q", got)
	}
}

// Charter fix (5): the intent whitelist must stay in lockstep with
// buildIntentMutation. Whitelisted extras must be refused here (fail
// closed) — never routed downstream with "unsupported action kind".
func TestIntentWhitelistFailClosed(t *testing.T) {
	for _, m := range mutatingIntents {
		if m[0] != "exec" {
			t.Errorf("intent %s %s: whitelist narrowed to exec commands (charter fix 5)", m[0], m[1])
		}
	}
	for _, extra := range [][2]string{
		{"simulate", "stream"}, {"plugins", "install"},
		{"prt", "import"}, {"pivot", "cloud-to-onprem"},
	} {
		if isMutatingIntent([]string{extra[0], extra[1]}) {
			t.Errorf("extra %s %s must not be routed to the spine (it is refused downstream)", extra[0], extra[1])
		}
		_, _, _, err := buildIntentMutation([]string{extra[0], extra[1]})
		if err == nil || !strings.Contains(err.Error(), "unsupported action kind") {
			t.Errorf("extra %s %s must fail closed here; got %v", extra[0], extra[1], err)
		}
	}
}

// Charter fix (8): provider calls carry a timeout.
func TestProvidersHTTPTimeout(t *testing.T) {
	if providersHTTP == nil || providersHTTP.Timeout <= 0 {
		t.Error("providersHTTP must carry a positive timeout (charter fix 8)")
	}
}

// Charter fix (H4/F-38-1): doctor's probe roundtrip must close the
// workspace before Delete. On Windows this check FAILED every run before
// the fix ("file in use").
func TestDoctorProbeRoundtrip(t *testing.T) {
	c := workspaceRoundtripCheck()
	if !c.ok {
		t.Errorf("doctor probe roundtrip FAILED: %s", c.detail)
	}
}
