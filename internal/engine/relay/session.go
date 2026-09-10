package relay

import (
	"context"
	"fmt"
	"time"

	"github.com/Debajyoti0-0/aether/internal/protocol/oauth2"
	"github.com/Debajyoti0-0/aether/internal/types"
)

// SessionStretcher keeps a captured session alive beyond its normal
// lifetime by continuously refreshing the token with backoff.
type SessionStretcher struct {
	OAuth      *oauth2.Client
	Tenant     string
	ClientID   string
	Scope      string
	MaxRefresh int
	Interval   time.Duration

	refreshCount int
	current      *types.OAuthTokens
}

// NewSessionStretcher builds a stretcher with sensible defaults.
func NewSessionStretcher(oauth *oauth2.Client, tenant, clientID string) *SessionStretcher {
	return &SessionStretcher{
		OAuth:      oauth,
		Tenant:     tenant,
		ClientID:   clientID,
		Scope:      "https://graph.microsoft.com/.default offline_access",
		MaxRefresh: 10,
		Interval:   0, // refresh when tokens near expiry
	}
}

// Stretch initializes the stretcher with a captured refresh token.
func (s *SessionStretcher) Stretch(ctx context.Context, refreshToken string) (*types.OAuthTokens, error) {
	return s.refreshOnce(ctx, refreshToken)
}

// Maintain keeps the session alive until MaxRefresh is reached or ctx
// ends. It sleeps until tokens near expiry before each refresh.
func (s *SessionStretcher) Maintain(ctx context.Context) (*types.OAuthTokens, error) {
	if s.current == nil {
		return nil, fmt.Errorf("call Stretch first")
	}

	for s.refreshCount < s.MaxRefresh {
		wait := time.Duration(s.current.ExpiresIn) * time.Second - 5*time.Minute
		if wait < 0 {
			wait = 0
		}
		if s.Interval > 0 {
			wait = s.Interval
		}

		select {
		case <-ctx.Done():
			return s.current, ctx.Err()
		case <-time.After(wait):
		}

		tokens, err := s.refreshOnce(ctx, s.current.RefreshToken)
		if err != nil {
			return s.current, err
		}
		s.current = tokens
	}
	return s.current, nil
}

func (s *SessionStretcher) refreshOnce(ctx context.Context, refreshToken string) (*types.OAuthTokens, error) {
	if refreshToken == "" {
		return nil, fmt.Errorf("refresh token is empty")
	}

	tokens, err := s.OAuth.RefreshToken(ctx, s.Tenant, s.ClientID, refreshToken, s.Scope)
	if err != nil {
		return nil, fmt.Errorf("refresh: %w", err)
	}
	// Keep the old refresh token if the server did not rotate it.
	if tokens.RefreshToken == "" {
		tokens.RefreshToken = refreshToken
	}
	s.refreshCount++
	s.current = tokens
	return tokens, nil
}

// RefreshCount reports how many successful refreshes have occurred.
func (s *SessionStretcher) RefreshCount() int {
	return s.refreshCount
}
