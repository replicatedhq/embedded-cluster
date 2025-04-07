package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
)

// signalHandler sets up handling for signals to ensure cleanup functions are called.
func signalHandler(ctx context.Context, cancel context.CancelFunc, cleanupFuncs ...func(context.Context, error)) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case sig := <-sigChan:
			logrus.Debugf("Received signal: %v", sig)
			err := fmt.Errorf("command interrupted by signal: %v", sig)

			for _, cleanup := range cleanupFuncs {
				cleanup(ctx, err)
			}

			// Cancel the context after cleanup functions run
			cancel()

			// Exit with non-zero status
			os.Exit(1)
		case <-ctx.Done():
			// Context was canceled elsewhere, do nothing
			return
		}
	}()
}
