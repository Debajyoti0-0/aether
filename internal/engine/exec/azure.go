package exec

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Debajyoti0-0/aether/internal/types"
)

// AzureAPIBase is the Azure management endpoint.
const AzureAPIBase = "https://management.azure.com"

// AzureExecutor executes commands on Azure VMs via the RunCommand API,
// speaking raw ARM REST (no SDK) so tokens are used directly.
type AzureExecutor struct {
	SubscriptionID string
	Token          string // bearer token for management.azure.com
	HTTPClient     *http.Client
	APIBase        string
}

// NewAzureExecutor builds an executor.
func NewAzureExecutor(subscriptionID, token string, hc *http.Client) *AzureExecutor {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &AzureExecutor{SubscriptionID: subscriptionID, Token: token, HTTPClient: hc, APIBase: AzureAPIBase}
}

// RunCommandInput is the ARM RunCommand request body.
type RunCommandInput struct {
	CommandID string   `json:"commandId"`
	Script    []string `json:"script"`
	Timeout   int      `json:"timeoutInSeconds,omitempty"`
}

// runCommandResult is the ARM instance-view result.
type runCommandResult struct {
	Status  string `json:"status"`
	Output  string `json:"output"`
	Error   string `json:"error"`
	ExitCode int   `json:"exitCode"`
}

// ExecuteOnAzureVM runs a shell command on a Linux VM (RunShellScript).
func (e *AzureExecutor) ExecuteOnAzureVM(ctx context.Context, resourceGroup, vmID, command string) (*types.CommandResult, error) {
	if e.SubscriptionID == "" {
		return nil, fmt.Errorf("subscription id is required")
	}
	if e.Token == "" {
		return nil, fmt.Errorf("access token is required")
	}

	url := fmt.Sprintf("%s/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachines/%s/runCommand?api-version=2024-03-01",
		strings.TrimRight(e.APIBase, "/"), e.SubscriptionID, resourceGroup, vmID)

	payload := RunCommandInput{CommandID: "RunShellScript", Script: []string{command}, Timeout: 1800}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+e.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("runcommand request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}

	// Synchronous API returns 200 with results; 202 means async polling.
	if resp.StatusCode == http.StatusAccepted {
		return e.pollAsync(ctx, url, resp.Header.Get("Location"), respBody)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("runcommand http %d: %s", resp.StatusCode, truncateBytes(respBody, 512))
	}

	return parseRunCommandResult(respBody)
}

func (e *AzureExecutor) pollAsync(ctx context.Context, pollURL, location string, body []byte) (*types.CommandResult, error) {
	target := location
	if target == "" {
		target = pollURL
	}

	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+e.Token)

		resp, err := e.HTTPClient.Do(req)
		if err != nil {
			continue
		}
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return parseRunCommandResult(b)
		}
	}
	return nil, fmt.Errorf("runcommand polling timed out")
}

func parseRunCommandResult(body []byte) (*types.CommandResult, error) {
	var wrapper struct {
		Value []struct {
			Code          string           `json:"code"`
			Level         string           `json:"level"`
			Message       string           `json:"message"`
			InstanceView  *runCommandResult `json:"instanceView,omitempty"`
		} `json:"value"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, fmt.Errorf("parse runcommand response: %w", err)
	}

	result := &types.CommandResult{}
	for _, v := range wrapper.Value {
		if v.InstanceView != nil {
			result.ExitCode = v.InstanceView.ExitCode
			if v.InstanceView.Error != "" {
				result.Output += v.InstanceView.Error
			} else {
				result.Output += v.InstanceView.Output
			}
		} else if v.Message != "" {
			result.Output += v.Message
		}
		if v.Code == "ProvisioningState/succeeded" || v.Code == "ComponentStatus/StdOut/succeeded" {
			result.Status = "succeeded"
		}
		if v.Code == "ComponentStatus/StdErr/failed" || strings.EqualFold(v.Level, "error") {
			result.Status = "failed"
		}
	}
	if result.Status == "" {
		result.Status = "completed"
	}
	return result, nil
}

func truncateBytes(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
