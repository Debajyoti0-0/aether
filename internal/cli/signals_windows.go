//go:build windows

package cli

import (
	"context"
	"os"
	"os/signal"
)

// interruptSignals returns the interrupt signal set for Windows.
// syscall.SIGTERM is accepted by signal.Notify on Windows but only
// console CTRL events (SIGINT/CTRL_BREAK) actually fire.
func interruptSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}

// signalContext wraps signal.NotifyContext with platform signals.
func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), interruptSignals()...)
}
