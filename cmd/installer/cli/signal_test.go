package cli

import (
	"context"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_signalHandler_Signal(t *testing.T) {
	// Create a context with cancel function
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a waitgroup to synchronize the test
	var wg sync.WaitGroup
	wg.Add(1)

	// Track if cleanup function was called
	cleanupCalled := false
	var cleanupSignal os.Signal

	// Mock cleanup function
	cleanup := func(ctx context.Context, sig os.Signal) {
		cleanupCalled = true
		if sig != nil {
			cleanupSignal = sig
		}
		wg.Done()
	}

	// Save original os.Exit and restore after test
	originalOsExit := osExit
	defer func() { osExit = originalOsExit }()

	exitCode := 0
	osExit = func(code int) {
		exitCode = code
		// Instead of exiting, just cancel the context
		cancel()
	}

	// Set up the signal handler
	signalHandler(ctx, cancel, cleanup)

	// Send a signal to trigger the handler
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("Failed to find process: %v", err)
	}

	// Send SIGINT to trigger the handler
	err = p.Signal(syscall.SIGINT)
	if err != nil {
		t.Fatalf("Failed to send signal: %v", err)
	}

	// Wait for cleanup to be called with a timeout
	waitCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
		// Success - cleanup was called
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for cleanup function to be called")
	}

	// Verify cleanup was called with the expected error
	assert.True(t, cleanupCalled, "Cleanup function should have been called")
	assert.Equal(t, syscall.SIGINT, cleanupSignal, "Cleanup should be called with SIGINT")
	assert.Equal(t, 128+int(syscall.SIGINT), exitCode, "Exit code should be 128 + signal number")
}

func Test_signalHandler_ContextDone(t *testing.T) {
	// Create a context with cancel function
	ctx, cancel := context.WithCancel(context.Background())

	// We expect cleanup NOT to be called when context is cancelled
	cleanupCalled := false

	cleanup := func(ctx context.Context, sig os.Signal) {
		cleanupCalled = true
	}

	// Set up the signal handler
	signalHandler(ctx, cancel, cleanup)

	// Cancel the context
	cancel()

	// Give some time for any handlers to run
	time.Sleep(100 * time.Millisecond)

	// Verify cleanup was NOT called
	assert.False(t, cleanupCalled, "Cleanup function should not have been called when context is done")
}
