package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
)

// osExit is a variable to make testing easier
var osExit = os.Exit

// signalHandler sets up handling for signals to ensure cleanup functions are called.
func signalHandler(ctx context.Context, cancel context.CancelFunc, cleanupFuncs ...func(context.Context, os.Signal)) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case sig := <-sigChan:
			logrus.Debugf("Received signal: %v", sig)

			for _, cleanup := range cleanupFuncs {
				cleanup(ctx, sig)
			}

			// Cancel the context after cleanup functions run
			cancel()

			// Exit with code 128 + signal number (Unix convention)
			signum := 0
			if sigVal, ok := sig.(syscall.Signal); ok {
				signum = int(sigVal)
			}
			osExit(128 + signum)
		case <-ctx.Done():
			// Context was canceled elsewhere, do nothing
			return
		}
	}()
}
