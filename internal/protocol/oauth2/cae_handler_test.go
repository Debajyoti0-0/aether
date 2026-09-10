package oauth2

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Debajyoti0-0/aether/internal/types"
	"github.com/Debajyoti0-0/aether/test/mock"
)

func challengeHeader(claimsB64 string) http.Header {
	h := http.Header{}
	h.Set("WWW-Authenticate", `Bearer authorization_uri="https://login.windows.net/common/oauth2/authorize", error="insufficient_claims", claims="`+claimsB64+`"`)
	return h
}

func TestClaimsChallengeDetection(t *testing.T) {
	claims := map[string]any{
		"access_token": map[string]any{"acrs": map[string]any{"essential": true}},
	}
	claimsJSON, _ := json.Marshal(claims)
	claimsB64 := base64.StdEncoding.EncodeToString(claimsJSON)

	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Header:     challengeHeader(claimsB64),
		Body:       http.NoBody,
	}

	got, found, err := ClaimsChallenge(resp)
	if err != nil {
		t.Fatalf("claims challenge: %v", err)
	}
	if !found {
		t.Fatal("challenge not detected")
	}
	if _, ok := got["access_token"]; !ok {
		t.Errorf("claims = %v", got)
	}

	// 200 responses never carry a challenge.
	ok200 := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: http.NoBody}
	if _, found, _ := ClaimsChallenge(ok200); found {
		t.Error("200 should not be a challenge")
	}
}

func TestHandleCAEChallenge(t *testing.T) {
	srv := mock.NewEntraIDServer()
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, srv.Client())
	tokens, err := c.HandleCAEChallenge(context.Background(), "dummy", "cid", "rt-1",
		map[string]any{"access_token": map[string]any{"acrs": "essential"}})
	if err != nil {
		t.Fatalf("cae handling: %v", err)
	}
	if tokens.AccessToken != "mock_access_token" {
		t.Errorf("token = %q", tokens.AccessToken)
	}
}

func TestHandleCAEChallengeValidation(t *testing.T) {
	srv := mock.NewEntraIDServer()
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, srv.Client())
	if _, err := c.HandleCAEChallenge(context.Background(), "t", "c", "", map[string]any{}); err == nil {
		t.Error("empty refresh token should fail")
	}
	if _, err := c.HandleCAEChallenge(context.Background(), "t", "c", "rt", nil); err == nil {
		t.Error("empty challenge should fail")
	}
}

func TestRequestWithCAERetryNoChallenge(t *testing.T) {
	srv := mock.NewEntraIDServer()
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, srv.Client())
	tokens := &types.OAuthTokens{AccessToken: "abc", RefreshToken: "rt"}

	calls := 0
	resp, got, err := c.RequestWithCAERetry(context.Background(), "dummy", "cid", tokens, func(bearer string) (*http.Response, error) {
		calls++
		if bearer != "abc" {
			t.Errorf("bearer = %q", bearer)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: http.NoBody}, nil
	})
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if resp.StatusCode != 200 || calls != 1 || got != tokens {
		t.Errorf("resp=%v calls=%d", resp.StatusCode, calls)
	}
}

func TestRequestWithCAERetryWithChallenge(t *testing.T) {
	srv := mock.NewEntraIDServer()
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, srv.Client())
	tokens := &types.OAuthTokens{AccessToken: "stale", RefreshToken: "rt-1"}

	claims := map[string]any{"access_token": map[string]any{"acrs": "essential"}}
	claimsJSON, _ := json.Marshal(claims)
	claimsB64 := base64.StdEncoding.EncodeToString(claimsJSON)

	calls := 0
	resp, refreshed, err := c.RequestWithCAERetry(context.Background(), "dummy", "cid", tokens, func(bearer string) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{
				StatusCode: 401,
				Header:     challengeHeader(claimsB64),
				Body:       http.NoBody,
			}, nil
		}
		if bearer != "mock_access_token" {
			t.Errorf("second bearer = %q", bearer)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: http.NoBody}, nil
	})
	if err != nil {
		t.Fatalf("retry with challenge: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
	if resp.StatusCode != 200 {
		t.Errorf("final status = %d", resp.StatusCode)
	}
	if refreshed == tokens {
		t.Error("tokens should have been refreshed to a new object")
	}
	if !strings.HasPrefix(refreshed.AccessToken, "mock_") {
		t.Errorf("refreshed token = %q", refreshed.AccessToken)
	}
}
