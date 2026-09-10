package relay

import (
	"context"
	"fmt"
	"time"

	"github.com/Debajyoti0-0/aether/internal/protocol/oauth2"
	"github.com/Debajyoti0-0/aether/internal/types"
)

// DeviceCodeRelay runs the OAuth2 device code flow for a chosen client,
// so the operator can phish/relay the code from another channel and
// poll for the resulting tokens.
type DeviceCodeRelay struct {
	OAuth  *oauth2.Client
	Tenant string
}

// Start initiates the device code flow.
func (d *DeviceCodeRelay) Start(ctx context.Context, clientID, scope string) (*oauth2.DeviceCode, error) {
	if d.Tenant == "" {
		return nil, fmt.Errorf("tenant is required")
	}
	if scope == "" {
		scope = "https://graph.microsoft.com/.default offline_access"
	}
	return d.OAuth.StartDeviceCode(ctx, d.Tenant, clientID, scope)
}

// Poll waits for the user to approve the code, polling at the interval
// the server advertises until the code expires or the context ends.
func (d *DeviceCodeRelay) Poll(ctx context.Context, clientID string, dc *oauth2.DeviceCode) (*types.OAuthTokens, error) {
	interval := time.Duration(dc.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)
	if dc.ExpiresIn <= 0 {
		deadline = time.Now().Add(15 * time.Minute)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Until(deadline)):
			return nil, fmt.Errorf("device code expired")
		case <-ticker.C:
			tokens, err := d.OAuth.PollDeviceCode(ctx, d.Tenant, clientID, dc)
			if err == oauth2.ErrAuthPending {
				continue
			}
			if err != nil {
				return nil, err
			}
			return tokens, nil
		}
	}
}
