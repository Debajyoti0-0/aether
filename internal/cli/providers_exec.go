package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/Debajyoti0-0/aether/pkg/plugins/sdk"
)

// execOktaUsers enumerates Okta users with the raw REST shape used by
// the okta plugin (kept here so the CLI controls output formatting).
func execOktaUsers(token, domain string, limit int) error {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if domain == "" || token == "" {
		return fmt.Errorf("--domain and --token are required")
	}

	url := strings.TrimRight(domain, "/") + fmt.Sprintf("/api/v1/users?limit=%d", limit)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "SSWS "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("okta request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("okta http %d: %s", resp.StatusCode, string(body[:min(len(body), 256)]))
	}

	var users []struct {
		ID     string `json:"id"`
		Login  string `json:"login"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &users); err != nil {
		return err
	}

	for _, u := range users {
		fmt.Printf("%-24s %-10s %s\n", u.Login, u.Status, u.ID)
	}
	fmt.Fprintf(defaultStderr(), "\n%d user(s)\n", len(users))
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// defaultStderr indirection keeps output testable.
var defaultStderr = func() io.Writer { return os.Stderr }

// sdkResultAlias keeps the sdk import referenced by shared helpers.
var _ = sdk.Result{}
