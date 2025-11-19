package statemachine

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"
)

var (
	_ EventHandler = &eventHandler{}
)

// EventHandler is an interface for handling state transition events in the state machine.
type EventHandler interface {
	// TriggerHandler triggers the event handler for a state transition.
	TriggerHandler(ctx context.Context, fromState, toState State, eventData interface{}) error
}

// EventHandlerFunc is a function that handles state transition events. Used to report state changes.
type EventHandlerFunc func(ctx context.Context, fromState, toState State, eventData interface{})

// EventHandlerOption is a configurable state machine option.
type EventHandlerOption func(*eventHandler)

// WithHandlerTimeout sets the timeout for the event handler to complete.
func WithHandlerTimeout(timeout time.Duration) EventHandlerOption {
	return func(eh *eventHandler) {
		eh.timeout = timeout
	}
}

// NewEventHandler creates a new event handler with the provided function and options.
func NewEventHandler(handler EventHandlerFunc, options ...EventHandlerOption) EventHandler {
	eh := &eventHandler{
		handler: handler,
		timeout: 5 * time.Second, // Default timeout
	}

	for _, option := range options {
		option(eh)
	}

	return eh
}

// eventHandler is a struct that implements the EventHandler interface. It contains a handler function that is called when a state transition occurs, and it supports a timeout for the handler to complete.
type eventHandler struct {
	handler EventHandlerFunc
	timeout time.Duration // Timeout for the handler to complete, default is 5 seconds
}

// TriggerHandler triggers the event handler for a state transition. The trigger is blocking and will wait for the handler to complete or timeout.
func (eh *eventHandler) TriggerHandler(ctx context.Context, fromState, toState State, eventData interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, eh.timeout)
	defer cancel()
	done := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Capture panic but don't affect the transition
				//nolint:staticcheck // ST1005 not sure why we need a newline here
				err := fmt.Errorf("event handler panic from %s to %s: %v: %s\n", fromState, toState, r, debug.Stack())
				done <- err
			}
			close(done)
		}()
		eh.handler(ctx, fromState, toState, eventData)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		err := fmt.Errorf("event handler for transition from %s to %s timed out after %s", fromState, toState, eh.timeout)
		return err
	}
}
