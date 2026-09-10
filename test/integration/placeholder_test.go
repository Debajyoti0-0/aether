//go:build integration

// Package integration holds cross-component integration tests.
// They are excluded from the default unit-test run and executed in CI via:
//
//	go test -tags=integration ./test/integration/...
//
// Stage 1 ships this placeholder so the CI job exists and fails loudly as
// soon as integration tests land (Stage 2). It deliberately asserts
// nothing beyond the build tag resolving.
package integration

import "testing"

func TestIntegrationTierActive(t *testing.T) {
	// Placeholder: the integration tier becomes populated in Stage 2
	// (Storage & Spine). Keeping this file ensures the CI job exercises
	// the correct build tag from day one.
}
