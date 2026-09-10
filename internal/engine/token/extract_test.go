package token

import (
	"encoding/base64"
	"strings"
	"testing"
)

const structuredDump = `Some log noise 2026-09-09
{"cookie":"0.AAAAA_b64blob","device_id":"dev-1","tenant_id":"t-1","user_id":"u-1","session_key":"MDEyMzQ1Njc4OWFiY2RlZg=="}
more noise {"unrelated": true}
{"cookie":"0.BBBBB_otherblob","tenant_id":"t-2"}`

func TestExtractStructuredJSON(t *testing.T) {
	prts, err := NewExtract().ExtractPRTs([]byte(structuredDump))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(prts) != 2 {
		t.Fatalf("prts = %d, want 2", len(prts))
	}
	if prts[0].DeviceID != "dev-1" || prts[0].TenantID != "t-1" {
		t.Errorf("prt0 = %+v", prts[0])
	}
	if prts[1].TenantID != "t-2" {
		t.Errorf("prt1 = %+v", prts[1])
	}
}

func TestExtractRawBase64Blob(t *testing.T) {
	// 0.-prefixed 600+ char base64url blob.
	blob := "0." + strings.Repeat("AbCdEf-_09", 65)
	prts, err := NewExtract().ExtractPRTs([]byte("noise\n"+blob+"\nnoise"))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(prts) != 1 || prts[0].Cookie != blob {
		t.Errorf("prts = %+v", prts)
	}
}

func TestExtractRejectsPlainBase64(t *testing.T) {
	// Base64 of plain lowercase ASCII text — no mixed classes.
	plain := strings.Repeat(base64.StdEncoding.EncodeToString([]byte("hello world ")), 40)
	if _, err := NewExtract().ExtractPRTs([]byte(plain)); err == nil {
		t.Error("plain-text base64 should not match")
	}
}

func TestExtractEmpty(t *testing.T) {
	if _, err := NewExtract().ExtractPRTs([]byte("nothing here at all")); err == nil {
		t.Error("expected error for no PRTs")
	}
}

func TestMarshalPRTs(t *testing.T) {
	prts, err := NewExtract().ExtractPRTs([]byte(structuredDump))
	if err != nil {
		t.Fatal(err)
	}
	data, err := MarshalPRTs(prts)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"cookie"`) {
		t.Errorf("marshaled = %s", data)
	}
}

func TestLooksLikePRTCookie(t *testing.T) {
	if !looksLikePRTCookie("0.AAAA"+strings.Repeat("x", 600)) {
		t.Error("0. prefix should match")
	}
	if looksLikePRTCookie(strings.Repeat("a", 700)) {
		t.Error("single-class blob should not match")
	}
}
