//go:build integration

// Stage 5 backfill interoperability matrix (B5-G10): 7 Tier-0 targets
// against the real teamserver/client/storage stack. Target 1 is a full
// mTLS teamserver↔client exchange with a real CA hierarchy. All
// targets are local and deterministic — no external services.
package integration

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Debajyoti0-0/aether/internal/api"
	"github.com/Debajyoti0-0/aether/internal/engine/mutation"
	"github.com/Debajyoti0-0/aether/internal/store"
	"github.com/Debajyoti0-0/aether/internal/workspace"
)

// interopPKI bootstraps a real CA hierarchy per test.
func interopPKI(t *testing.T) (srvCert tls.Certificate, clientCAs *x509.CertPool, caDir string) {
	t.Helper()
	dir := t.TempDir()
	paths, err := api.InitCA(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, pool, err := api.LoadServerTLS(paths.SrvCert, paths.SrvKey, paths.CACert)
	if err != nil {
		t.Fatal(err)
	}
	return srv, pool, dir
}

// interopOperator issues + loads an operator TLS pair from caDir.
func interopOperator(t *testing.T, caDir, name string) tls.Certificate {
	t.Helper()
	certPEM, keyPEM, err := api.IssueOperatorCert(
		filepath.Join(caDir, "teamserver-ca.crt"), filepath.Join(caDir, "teamserver-ca.key"), name, 30)
	if err != nil {
		t.Fatal(err)
	}
	certPath := filepath.Join(caDir, name+".crt")
	keyPath := filepath.Join(caDir, name+".key")
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	pair, _, err := api.LoadOperatorTLS(certPath, keyPath, filepath.Join(caDir, "teamserver-ca.crt"))
	if err != nil {
		t.Fatal(err)
	}
	return pair
}

// sanitizeName maps a subtest name to a safe workspace name.
func sanitizeName(s string) string {
	return strings.NewReplacer("/", "_", "\\", "_").Replace(s)
}

func TestInteropMatrix(t *testing.T) {
	t.Run("target1_full_mtls_exchange", func(t *testing.T) {
		srvCert, clientCAs, caDir := interopPKI(t)
		srv, err := api.NewTeamserver("127.0.0.1:0", srvCert, clientCAs, "", nil,
			func(op *api.Operator, req *api.CommandRequest) (*api.CommandResponse, error) {
				return &api.CommandResponse{Status: "completed", Output: "mtls-ok"}, nil
			})
		if err != nil {
			t.Fatal(err)
		}
		srv.SetMaxInFlight(16)
		go srv.Serve()
		t.Cleanup(func() { _ = srv.Close() })

		client, err := api.Dial(srv.Listener.Addr().String(), interopOperator(t, caDir, "alice"), nil, true)
		if err != nil {
			t.Fatalf("mTLS dial: %v", err)
		}
		t.Cleanup(func() { _ = client.Close() })

		resp, err := client.ExecuteCommand(&api.CommandRequest{WorkspaceID: "W", CommandLine: "interop ping"})
		if err != nil {
			t.Fatalf("command over mTLS: %v", err)
		}
		if resp.Status != "completed" || resp.Output != "mtls-ok" {
			t.Fatalf("resp = %+v", resp)
		}
	})

	t.Run("target2_cert_derived_identity", func(t *testing.T) {
		srvCert, clientCAs, caDir := interopPKI(t)
		gotFingerprint := ""
		srv, err := api.NewTeamserver("127.0.0.1:0", srvCert, clientCAs, "", nil,
			func(op *api.Operator, req *api.CommandRequest) (*api.CommandResponse, error) {
				if op.Name != "bob" {
					t.Errorf("identity = %q, want cert-derived bob", op.Name)
				}
				gotFingerprint = op.CertFingerprint
				return &api.CommandResponse{Status: "completed"}, nil
			})
		if err != nil {
			t.Fatal(err)
		}
		srv.SetMaxInFlight(16)
		go srv.Serve()
		t.Cleanup(func() { _ = srv.Close() })

		client, err := api.Dial(srv.Listener.Addr().String(), interopOperator(t, caDir, "bob"), nil, true)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = client.Close() })
		if _, err := client.ExecuteCommand(&api.CommandRequest{WorkspaceID: "W", CommandLine: "id"}); err != nil {
			t.Fatal(err)
		}
		if len(gotFingerprint) != 64 { // hex SHA-256 of DER
			t.Fatalf("fingerprint = %q", gotFingerprint)
		}
	})

	t.Run("target3_capability_fail_closed", func(t *testing.T) {
		srvCert, clientCAs, caDir := interopPKI(t)
		srv, err := api.NewTeamserver("127.0.0.1:0", srvCert, clientCAs, "", nil,
			func(op *api.Operator, req *api.CommandRequest) (*api.CommandResponse, error) {
				// Operator without a capability set: every specific
				// capability must be denied (fail closed).
				if op.CanExecute("exec.azure") || op.CanExecute("exec.aws") || op.CanExecute("unknown.cap") {
					t.Errorf("capability granted without a cap set: %+v", op.Caps)
					return nil, context.Canceled
				}
				return &api.CommandResponse{Status: "completed"}, nil
			})
		if err != nil {
			t.Fatal(err)
		}
		srv.SetMaxInFlight(16)
		go srv.Serve()
		t.Cleanup(func() { _ = srv.Close() })

		client, err := api.Dial(srv.Listener.Addr().String(), interopOperator(t, caDir, "carol"), nil, true)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = client.Close() })
		if _, err := client.ExecuteCommand(&api.CommandRequest{WorkspaceID: "W", CommandLine: "x"}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("target4_revocation_fail_closed", func(t *testing.T) {
		srvCert, clientCAs, caDir := interopPKI(t)
		rl := api.LoadRevocationList([]byte("mallory\n"))
		served := false
		srv, err := api.NewTeamserver("127.0.0.1:0", srvCert, clientCAs, "", rl,
			func(op *api.Operator, req *api.CommandRequest) (*api.CommandResponse, error) {
				served = true
				return &api.CommandResponse{Status: "completed"}, nil
			})
		if err != nil {
			t.Fatal(err)
		}
		go srv.Serve()
		t.Cleanup(func() { _ = srv.Close() })

		client, err := api.Dial(srv.Listener.Addr().String(), interopOperator(t, caDir, "mallory"), nil, true)
		if err != nil {
			t.Fatalf("TLS dial may succeed; identity gate follows: %v", err)
		}
		t.Cleanup(func() { _ = client.Close() })
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := client.ExecuteCommandCtx(ctx, &api.CommandRequest{WorkspaceID: "W", CommandLine: "x"}); err == nil {
			t.Fatal("revoked operator command served")
		}
		time.Sleep(200 * time.Millisecond)
		if served {
			t.Fatal("runner invoked for revoked operator")
		}
	})

	t.Run("target5_audit_export_verify_roundtrip", func(t *testing.T) {
		dir := t.TempDir()
		jsonl := filepath.Join(dir, "interop.jsonl")
		key := filepath.Join(dir, "interop.key")
		l, err := store.New(jsonl, key)
		if err != nil {
			t.Fatal(err)
		}
		for i := 1; i <= 3; i++ {
			if _, err := l.Append("interop-op", fmt.Sprintf("rec-%d", i)); err != nil {
				t.Fatal(err)
			}
		}
		if err := store.ExportFile(l, jsonl+".export"); err != nil {
			t.Fatal(err)
		}
		vr, err := store.VerifyFile(jsonl+".export", key)
		if err != nil || !vr.ValidAll || vr.Total != 3 {
			t.Fatalf("verify = %+v err=%v", vr, err)
		}
	})

	t.Run("target6_spine_governed_evidence", func(t *testing.T) {
		ws, err := workspace.Create("InteropSpine-"+sanitizeName(t.Name()), "integration-pass")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = ws.Close() })
		res, err := mutation.Run(context.Background(), ws, &fakeExec{
			kind: "exec.azure", target: "interop-vm", opID: "interop-op-1",
			undo: &mutation.UndoSpec{Irreversible: true},
		})
		if err != nil || res.Status != "completed" {
			t.Fatalf("res=%+v err=%v", res, err)
		}
		log, err := ws.AuditLog()
		if err != nil {
			t.Fatal(err)
		}
		entries, err := log.Entries()
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 2 {
			t.Fatalf("entries = %d, want 2 (before+after)", len(entries))
		}
		vr, err := log.Verify()
		if err != nil || !vr.ValidAll || vr.Total != 2 {
			t.Fatalf("verify = %+v err=%v", vr, err)
		}
	})

	t.Run("target7_vault_persistence_reconnect", func(t *testing.T) {
		name := "InteropVault-" + sanitizeName(t.Name())
		ws, err := workspace.Create(name, "integration-pass")
		if err != nil {
			t.Fatal(err)
		}
		type rec struct{ Value string }
		if err := ws.SaveRecord(workspace.BucketTokens, "interop", rec{Value: "persisted"}); err != nil {
			t.Fatal(err)
		}
		if err := ws.Close(); err != nil {
			t.Fatal(err)
		}
		w2, err := workspace.Open(name, "integration-pass")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = w2.Close() })
		var out rec
		if err := w2.LoadRecord(workspace.BucketTokens, "interop", &out); err != nil || out.Value != "persisted" {
			t.Fatalf("record = %+v err=%v", out, err)
		}
	})
}


