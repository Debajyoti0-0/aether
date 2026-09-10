package wstrust

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Debajyoti0-0/aether/internal/transport"
	"github.com/Debajyoti0-0/aether/internal/types"
)

// WSTrust versions / namespaces.
const (
	WSTrust13Namespace = "http://docs.oasis-open.org/ws-sx/ws-trust/200512"
	SOAPEnvelopeNS     = "http://www.w3.org/2003/05/soap-envelope"
	AddressingNS       = "http://www.w3.org/2005/08/addressing"
)

// DefaultEndpoints for Entra ID WS-Trust.
var (
	// UserRealmEndpoint validates the user's authentication authority.
	UserRealmEndpoint = "https://login.microsoftonline.com/common/userrealm/%s?api-version=1.0"
)

// UserRealmResponse is the response from the userrealm discovery endpoint.
type UserRealmResponse struct {
	NameSpaceType       string `json:"namespace_type"`
	FederationBrandName string `json:"federation_brand_name"`
	CloudInstanceName   string `json:"cloud_instance_name"`
	DomainName          string `json:"domain_name"`
}

// Client performs WS-Trust RST/RSTR exchanges against an STS endpoint.
type Client struct {
	Endpoint   string
	HTTPClient *http.Client
}

// NewClient builds a WS-Trust client with the aether uTLS transport.
func NewClient(endpoint, preset string, timeout time.Duration) (*Client, error) {
	hc, err := transport.NewClient(preset, timeout)
	if err != nil {
		return nil, err
	}
	return &Client{Endpoint: endpoint, HTTPClient: hc}, nil
}

// NewClientWithHTTP builds a client around a supplied HTTP client (tests).
func NewClientWithHTTP(endpoint string, hc *http.Client) *Client {
	return &Client{Endpoint: endpoint, HTTPClient: hc}
}

// RequestSecurityToken sends an RST with username/password credentials
// and parses the returned SAML assertion from the RSTR.
func (c *Client) RequestSecurityToken(ctx context.Context, username, password, appliesTo string) (*types.SAMLAssertion, error) {
	if c.Endpoint == "" {
		return nil, fmt.Errorf("ws-trust endpoint is required (discover via userrealm)")
	}
	if appliesTo == "" {
		appliesTo = "urn:federation:MicrosoftOnline"
	}

	rst := BuildRST(username, password, appliesTo, c.Endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(rst))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	req.Header.Set("SOAPAction", `"http://docs.oasis-open.org/ws-sx/ws-trust/200512/RST/Issue"`)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rst request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rst returned http %d: %s", resp.StatusCode, truncate(body, 512))
	}

	return ParseRSTR(body)
}

// DiscoverUserRealm resolves which STS authenticates the user's domain.
func (c *Client) DiscoverUserRealm(ctx context.Context, httpClient *http.Client, user string) (*UserRealmResponse, error) {
	url := fmt.Sprintf(UserRealmEndpoint, user)

	hc := httpClient
	if hc == nil {
		hc = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userrealm request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	realm := &UserRealmResponse{}
	if err := xml.Unmarshal(body, realm); err != nil {
		// userrealm actually returns JSON; XML fallback keeps the
		// function tolerant of both in mock environments.
		return realm, decodeJSON(body, realm)
	}
	return realm, nil
}

func decodeJSON(body []byte, out *UserRealmResponse) error {
	var fields map[string]string
	if err := json.Unmarshal(body, &fields); err != nil {
		return fmt.Errorf("parse userrealm response: %w", err)
	}
	out.NameSpaceType = fields["NameSpaceType"]
	out.FederationBrandName = fields["FederationBrandName"]
	out.CloudInstanceName = fields["CloudInstanceName"]
	out.DomainName = fields["DomainName"]
	if out.NameSpaceType == "" {
		return fmt.Errorf("userrealm response missing NameSpaceType: %s", truncate(body, 256))
	}
	return nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}

func randomID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("_%x", b)
}

// normalizeNamespace helps tolerate slightly different RSTR shapes.
func normalizeNamespace(s string) string {
	return strings.TrimSpace(s)
}
