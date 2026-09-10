package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendAndVerifyChain(t *testing.T) {
	dir := t.TempDir()
	l, err := New(filepath.Join(dir, "audit.jsonl"), filepath.Join(dir, "audit.key"))
	if err != nil {
		t.Fatal(err)
	}

	for i, cmd := range []string{"prt convert", "cap evaluate", "exec azure"} {
		if _, err := l.Append(cmd, "ok"); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	res, err := l.Verify()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !res.ValidAll {
		t.Errorf("chain invalid: %+v", res)
	}
	if res.Total != 3 || res.Valid != 3 {
		t.Errorf("res = %+v", res)
	}
}

func TestVerifyDetectsTampering(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")
	keyPath := filepath.Join(dir, "audit.key")

	l, _ := New(logPath, keyPath)
	l.Append("safe command", "ok")
	l.Append("another command", "ok")

	// Tamper with the first entry's command in-place.
	data, _ := osReadFile(logPath)
	tampered := strings.Replace(string(data), "safe command", "EVIL command", 1)
	osWriteFile(logPath, []byte(tampered))

	checker, err := New(logPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	res, err := checker.Verify()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if res.ValidAll {
		t.Fatal("tampered chain reported valid")
	}
	if len(res.Tampered) != 1 || res.Tampered[0] != 1 {
		t.Errorf("tampered = %v", res.Tampered)
	}
}

func TestVerifyDetectsChainBreak(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")
	keyPath := filepath.Join(dir, "audit.key")

	l, _ := New(logPath, keyPath)
	l.Append("a", "ok")
	l.Append("b", "ok")

	// Delete the first entry → the second's PrevHash won't link.
	data, _ := osReadFile(logPath)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	osWriteFile(logPath, []byte(lines[1]+"\n"))

	checker, _ := New(logPath, keyPath)
	res, err := checker.Verify()
	if err != nil {
		t.Fatal(err)
	}
	if res.ValidAll || res.BrokenChainAt != 2 {
		t.Errorf("res = %+v", res)
	}
}

func TestKeyPersistence(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")
	keyPath := filepath.Join(dir, "audit.key")

	l1, _ := New(logPath, keyPath)
	l1.Append("persistent", "ok")

	// Reopen with the same key file — verification must pass.
	l2, err := New(logPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	res, err := l2.Verify()
	if err != nil || !res.ValidAll {
		t.Errorf("reopened verify = %+v err=%v", res, err)
	}
}

func TestExportJSONL(t *testing.T) {
	dir := t.TempDir()
	l, _ := New(filepath.Join(dir, "audit.jsonl"), filepath.Join(dir, "audit.key"))
	l.Append("cmd-1", "ok")
	l.Append("cmd-2", "ok")

	target := filepath.Join(dir, "export.jsonl")
	if err := ExportFile(l, target); err != nil {
		t.Fatalf("export: %v", err)
	}

	// The export verifies against the same key.
	res, err := VerifyFile(target, filepath.Join(dir, "audit.key"))
	if err != nil {
		t.Fatal(err)
	}
	if !res.ValidAll || res.Total != 2 {
		t.Errorf("export verify = %+v", res)
	}
}

func TestAppendCtx(t *testing.T) {
	dir := t.TempDir()
	l, _ := New(filepath.Join(dir, "audit.jsonl"), filepath.Join(dir, "audit.key"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := AppendCtx(ctx, l, "cmd", ""); err == nil {
		t.Error("cancelled context should fail")
	}

	if _, err := AppendCtx(context.Background(), l, "cmd", "ok"); err != nil {
		t.Errorf("append: %v", err)
	}
}

func TestAppendEmptyCommand(t *testing.T) {
	dir := t.TempDir()
	l, _ := New(filepath.Join(dir, "audit.jsonl"), filepath.Join(dir, "audit.key"))
	if _, err := l.Append("", ""); err == nil {
		t.Error("empty command should fail")
	}
}
