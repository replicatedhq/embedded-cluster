package statemachine

import (
	"context"
	"time"
)

// State represents the possible states of the install process
type State string

var (
	_ Interface    = &stateMachine{}
	_ EventHandler = &eventHandler{}
)

// Interface is the interface for the state machine
type Interface interface {
	// CurrentState returns the current state
	CurrentState() State
	// IsFinalState checks if the current state is a final state
	IsFinalState() bool
	// ValidateTransition checks if a transition from the current state to a new state is valid
	ValidateTransition(lock Lock, newState State) error
	// Transition attempts to transition to a new state and returns an error if the transition is
	// invalid.
	Transition(lock Lock, nextState State) error
	// AcquireLock acquires a lock on the state machine.
	AcquireLock() (Lock, error)
	// IsLockAcquired checks if a lock already exists on the state machine.
	IsLockAcquired() bool
	// Enable event handlers for state transitions
	WithEventHandlers
}

// WithEventHandlers is an interface that allows registering and unregistering event handlers for state transitions.
type WithEventHandlers interface {
	// RegisterEventHandler registers a sync event handler for reporting events in the state machine.
	RegisterEventHandler(targetState State, handler EventHandlerFunc, options ...EventHandlerOption)
	// UnregisterEventHandler unregisters a sync event handler for reporting events in the state machine.
	UnregisterEventHandler(targetState State)
}

type Lock interface {
	// Release releases the lock.
	Release()
}

type EventHandler interface {
	// TriggerHandler triggers the event handler for a state transition.
	TriggerHandler(ctx context.Context, fromState, toState State) error
}

// EventHandlerFunc is a function that handles state transition events. Used to report state changes.
type EventHandlerFunc func(ctx context.Context, fromState, toState State)

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
