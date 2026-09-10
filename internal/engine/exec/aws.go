package exec

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Debajyoti0-0/aether/internal/types"
)

// AWS SSM constants.
const (
	SSMAPIBase = "https://ssm.%s.amazonaws.com"
)

// AWSExecutor executes commands on EC2 instances via SSM RunCommand,
// speaking SigV4-signed REST directly (no SDK dependency).
type AWSExecutor struct {
	Region       string
	AccessKey    string
	SecretKey    string
	SessionToken string
	HTTPClient   *http.Client
}

// NewAWSExecutor builds an AWS SSM executor.
func NewAWSExecutor(region, accessKey, secretKey, sessionToken string, hc *http.Client) *AWSExecutor {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &AWSExecutor{Region: region, AccessKey: accessKey, SecretKey: secretKey, SessionToken: sessionToken, HTTPClient: hc}
}

// ssmCommand is the SSM SendCommand request body.
type ssmCommand struct {
	DocumentName string            `json:"DocumentName"`
	InstanceIDs  []string          `json:"InstanceIds"`
	Parameters   map[string][]string `json:"Parameters"`
	Comment      string            `json:"Comment,omitempty"`
	TimeoutSecs  int               `json:"TimeoutSeconds,omitempty"`
}

// ssmCommandResponse is the SendCommand response shape.
type ssmCommandResponse struct {
	Command struct {
		CommandID string `json:"CommandId"`
		Status    string `json:"Status"`
	} `json:"Command"`
}

// ExecuteOnEC2 runs a shell command on an EC2 instance via SSM.
func (e *AWSExecutor) ExecuteOnEC2(ctx context.Context, instanceID, command string) (*types.CommandResult, error) {
	if e.Region == "" {
		return nil, fmt.Errorf("region is required")
	}
	if e.AccessKey == "" || e.SecretKey == "" {
		return nil, fmt.Errorf("aws credentials are required")
	}

	cmd := ssmCommand{
		DocumentName: "AWS-RunShellScript",
		InstanceIDs:  []string{instanceID},
		Parameters:   map[string][]string{"commands": {command}},
		Comment:      "aether",
		TimeoutSecs:  3600,
	}
	body, _ := json.Marshal(cmd)

	endpoint := fmt.Sprintf(SSMAPIBase, e.Region)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "AmazonSSM.SendCommand")

	signer := &sigV4{
		AccessKey:    e.AccessKey,
		SecretKey:    e.SecretKey,
		SessionToken: e.SessionToken,
		Region:       e.Region,
		Service:      "ssm",
	}
	if err := signer.Sign(req, body); err != nil {
		return nil, fmt.Errorf("sign request: %w", err)
	}

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ssm sendcommand: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ssm http %d: %s", resp.StatusCode, truncateBytes(respBody, 512))
	}

	var out ssmCommandResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("parse ssm response: %w", err)
	}

	return &types.CommandResult{
		Status:   out.Command.Status,
		Output:   fmt.Sprintf("command dispatched: %s (poll GetCommandInvocation for output)", out.Command.CommandID),
		ExitCode: -1,
	}, nil
}

// GetInvocation fetches the output of a dispatched SSM command.
func (e *AWSExecutor) GetInvocation(ctx context.Context, commandID, instanceID string) (*types.CommandResult, error) {
	body, _ := json.Marshal(map[string]string{
		"CommandId":  commandID,
		"InstanceId": instanceID,
	})

	endpoint := fmt.Sprintf(SSMAPIBase, e.Region)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "AmazonSSM.GetCommandInvocation")

	signer := &sigV4{AccessKey: e.AccessKey, SecretKey: e.SecretKey, SessionToken: e.SessionToken, Region: e.Region, Service: "ssm"}
	if err := signer.Sign(req, body); err != nil {
		return nil, err
	}

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ssm http %d: %s", resp.StatusCode, truncateBytes(respBody, 512))
	}

	var out struct {
		Status           string `json:"Status"`
		Output           string `json:"Output"`
		StandardError    string `json:"StandardErrorContent"`
		ResponseCode     int    `json:"ResponseCode"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}

	result := &types.CommandResult{Status: out.Status, ExitCode: out.ResponseCode, Output: out.Output}
	if out.StandardError != "" {
		result.Output += "\n" + out.StandardError
	}
	return result, nil
}

// sigV4 implements the minimal AWS Signature Version 4 signing process
// (single-chunk payloads, POST with JSON body).
type sigV4 struct {
	AccessKey    string
	SecretKey    string
	SessionToken string
	Region       string
	Service      string
}
