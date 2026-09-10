package exec

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// fixedTime pins signing time for deterministic assertions.
var fixedTime = time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)

var realTimeNow = time.Now

type timeTime = time.Time

func TestSigV4SignatureStable(t *testing.T) {
	amzTimeNow = func() (t timeTime) { return fixedTime }
	defer func() { amzTimeNow = realTimeNow }()

	req, _ := http.NewRequest("POST", "https://ssm.us-east-1.amazonaws.com/", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "AmazonSSM.SendCommand")

	s := &sigV4{AccessKey: "AKID", SecretKey: "secret", Region: "us-east-1", Service: "ssm"}
	if err := s.Sign(req, []byte("{}")); err != nil {
		t.Fatalf("sign: %v", err)
	}

	auth := req.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256") {
		t.Errorf("authorization = %q", auth)
	}
	if !strings.Contains(auth, "SignedHeaders=content-type;host;x-amz-date;x-amz-target") {
		t.Errorf("signed headers missing: %q", auth)
	}
	if req.Header.Get("X-Amz-Date") != "20260102T150405Z" {
		t.Errorf("x-amz-date = %q", req.Header.Get("X-Amz-Date"))
	}
}

func TestCanonicalQuerySorted(t *testing.T) {
	req, _ := http.NewRequest("GET", "https://api.example.com/?b=2&a=1&a=0", nil)
	got := canonicalQuery(req)
	if got != "a=0&a=1&b=2" {
		t.Errorf("canonical query = %q", got)
	}
}

func TestURIEscape(t *testing.T) {
	if uriEscape("a b/c") != "a%20b%2Fc" {
		t.Errorf("escape = %q", uriEscape("a b/c"))
	}
	if uriEscape("a-b_c.d~e") != "a-b_c.d~e" {
		t.Errorf("unreserved chars changed")
	}
}
