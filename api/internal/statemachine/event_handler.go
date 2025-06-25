package statemachine

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"
)

// eventHandler is a struct that holds the event handler function and its timeout.
type eventHandler struct {
	handler EventHandlerFunc
	timeout time.Duration // Timeout for the handler to complete, default is 5 seconds
}

// TriggerHandler triggers the event handler for a state transition. The trigger is blocking and will wait for the handler to complete or timeout.
func (eh *eventHandler) TriggerHandler(ctx context.Context, fromState, toState State) error {
	ctx, cancel := context.WithTimeout(ctx, eh.timeout)
	defer cancel()
	done := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Capture panic but don't affect the transition
				err := fmt.Errorf("event handler panic from %s to %s: %v: %s\n", fromState, toState, r, debug.Stack())
				done <- err
			}
		}()
		eh.handler(ctx, fromState, toState)
		close(done)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		err := fmt.Errorf("event handler for transition from %s to %s timed out after %s", fromState, toState, eh.timeout)
		return err
	}
}
