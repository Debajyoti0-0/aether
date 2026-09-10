//go:build !windows

package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// interruptSignals returns the interrupt signal set for unix-likes:
// SIGINT (ctrl-c) and SIGTERM (systemd/kill).
func interruptSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}

// signalContext wraps signal.NotifyContext with platform signals.
func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), interruptSignals()...)
}
