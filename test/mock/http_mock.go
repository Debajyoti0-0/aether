package mock

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
)

// NewEntraIDServer simulates Entra ID token endpoints for tests.
// The returned server must be closed by the caller.
func NewEntraIDServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/common/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch r.Form.Get("grant_type") {
		case "srv_challenge":
			writeJSON(w, http.StatusOK, map[string]string{"Nonce": "mock-nonce-123"})
		default:
			writeJSON(w, http.StatusOK, map[string]any{
				"access_token":  "mock_access_token",
				"refresh_token": "mock_refresh_token",
				"expires_in":    3600,
				"token_type":    "Bearer",
			})
		}
	})

	mux.HandleFunc("/common/oauth2/v2.0/token", handleToken)
	mux.HandleFunc("/dummy/oauth2/v2.0/token", handleToken)
	mux.HandleFunc("/dummy/oauth2/v2.0/devicecode", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"device_code":      "mock_device_code",
			"user_code":        "MOCKCODE",
			"verification_uri": "https://microsoft.com/devicelogin",
			"expires_in":       900,
			"interval":         1,
		})
	})

	return httptest.NewServer(mux)
}

func handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	grant := r.Form.Get("grant_type")
	switch grant {
	case "urn:ietf:params:oauth:grant-type:device_code":
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":             "authorization_pending",
			"error_description": "waiting for user",
		})
	case "":
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid_request",
		})
	default:
		writeJSON(w, http.StatusOK, map[string]any{
			"access_token":  "mock_access_token",
			"refresh_token": "mock_refresh_token",
			"id_token":      "mock_id_token",
			"expires_in":    3600,
			"token_type":    "Bearer",
			"scope":         r.Form.Get("scope"),
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
