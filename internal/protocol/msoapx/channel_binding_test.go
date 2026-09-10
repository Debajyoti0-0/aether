package msoapx

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
	"testing"
)

func TestChannelBindingHeader(t *testing.T) {
	finished := []byte("0123456789abcdef0123456789abcdef")
	cb := &ChannelBinding{FinishedMessage: finished}

	want := "tls-unique:" + base64.StdEncoding.EncodeToString(func() []byte {
		h := sha256.Sum256(finished)
		return h[:]
	}())
	if got := cb.GenerateHeader(); got != want {
		t.Errorf("header = %q, want %q", got, want)
	}
}

func TestLoadChannelBindingBase64(t *testing.T) {
	cb, err := LoadChannelBinding(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef")))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if string(cb.FinishedMessage) != "0123456789abcdef" {
		t.Errorf("binding = %q", cb.FinishedMessage)
	}
}

func TestLoadChannelBindingHex(t *testing.T) {
	cb, err := LoadChannelBinding("30313233343536373839616263646566")
	if err != nil {
		t.Fatalf("load hex: %v", err)
	}
	if string(cb.FinishedMessage) != "0123456789abcdef" {
		t.Errorf("binding = %q", cb.FinishedMessage)
	}
}

func TestLoadChannelBindingEmpty(t *testing.T) {
	if _, err := LoadChannelBinding(""); err == nil {
		t.Error("empty binding should fail")
	}
	if _, err := LoadChannelBinding("!!!notvalid!!!"); err == nil {
		t.Error("invalid binding should fail")
	}
}

func TestInjectBinding(t *testing.T) {
	req, _ := http.NewRequest("POST", "https://login.microsoftonline.com/token", nil)
	cb := &ChannelBinding{FinishedMessage: []byte("0123456789abcdef0123456789abcdef")}

	InjectBinding(req, cb)
	if !strings.HasPrefix(req.Header.Get("x-client-bound"), "tls-unique:") {
		t.Error("x-client-bound header missing")
	}
	if !strings.HasPrefix(req.Header.Get("x-ms-client-binding"), "tls-unique:") {
		t.Error("x-ms-client-binding header missing")
	}

	// nil safety
	InjectBinding(req, nil)
	InjectBinding(nil, cb)
}
