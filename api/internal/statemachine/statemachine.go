package statemachine

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/sirupsen/logrus"
)

// State represents the possible states of the install process
type State string

var (
	_ Interface = &stateMachine{}
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
	// RegisterEventHandler registers an async event handler for reporting events in the state machine.
	RegisterEventHandler(targetState State, handler EventHandler)
	// UnregisterEventHandler unregisters an async event handler for reporting events in the state machine.
	UnregisterEventHandler(targetState State)
}

// EventHandler is a function that handles state transition events. Used to report state changes.
type EventHandler func(ctx context.Context, fromState, toState State)

type Lock interface {
	// Release releases the lock.
	Release()
}

// stateMachine manages the state transitions for the install process
type stateMachine struct {
	currentState          State
	validStateTransitions map[State][]State
	lock                  *lock
	mu                    sync.RWMutex
	eventHandlers         map[State][]EventHandler
	logger                logrus.FieldLogger
	handlerTimeout        time.Duration // Timeout for event handlers to complete, default is 5 seconds
}

// StateMachineOption is a configurable state machine option.
type StateMachineOption func(*stateMachine)

// New creates a new state machine starting in the given state with the given valid state
// transitions and options.
func New(currentState State, validStateTransitions map[State][]State, opts ...StateMachineOption) *stateMachine {
	sm := &stateMachine{
		currentState:          currentState,
		validStateTransitions: validStateTransitions,
		logger:                logger.NewDiscardLogger(),
		eventHandlers:         make(map[State][]EventHandler),
		handlerTimeout:        5 * time.Second,
	}

	for _, opt := range opts {
		opt(sm)
	}

	return sm
}

func WithLogger(logger logrus.FieldLogger) StateMachineOption {
	return func(sm *stateMachine) {
		sm.logger = logger
	}
}

func WithHandlerTimeout(handlerTimeout time.Duration) StateMachineOption {
	return func(sm *stateMachine) {
		sm.handlerTimeout = handlerTimeout
	}
}

func (sm *stateMachine) CurrentState() State {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.currentState
}

func (sm *stateMachine) IsFinalState() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return len(sm.validStateTransitions[sm.currentState]) == 0
}

func (sm *stateMachine) AcquireLock() (Lock, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.lock != nil {
		return nil, fmt.Errorf("lock already acquired")
	}

	sm.lock = &lock{
		release: func() {
			sm.mu.Lock()
			defer sm.mu.Unlock()
			sm.lock = nil
		},
	}

	return sm.lock, nil
}

func (sm *stateMachine) IsLockAcquired() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.lock != nil
}

func (sm *stateMachine) ValidateTransition(lock Lock, nextState State) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.lock == nil {
		return fmt.Errorf("lock not acquired")
	} else if sm.lock != lock {
		return fmt.Errorf("lock mismatch")
	}

	if !sm.isValidTransition(sm.currentState, nextState) {
		return fmt.Errorf("invalid transition from %s to %s", sm.currentState, nextState)
	}

	return nil
}

func (sm *stateMachine) Transition(lock Lock, nextState State) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.lock == nil {
		return fmt.Errorf("lock not acquired")
	} else if sm.lock != lock {
		return fmt.Errorf("lock mismatch")
	}

	if !sm.isValidTransition(sm.currentState, nextState) {
		return fmt.Errorf("invalid transition from %s to %s", sm.currentState, nextState)
	}

	fromState := sm.currentState
	sm.currentState = nextState

	// Trigger event handlers asynchronously after successful transition
	sm.triggerHandlers(fromState, nextState)

	return nil
}

func (sm *stateMachine) RegisterEventHandler(targetState State, handler EventHandler) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.eventHandlers[targetState] = append(sm.eventHandlers[targetState], handler)
}

func (sm *stateMachine) UnregisterEventHandler(targetState State) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.eventHandlers, targetState)
}

// triggerHandlers triggers event handlers asynchronously
func (sm *stateMachine) triggerHandlers(from, next State) {
	handlers, exists := sm.eventHandlers[next]
	if !exists || len(handlers) == 0 {
		return
	}
	handlersCopy := make([]EventHandler, len(handlers))
	// Make a copy of the handlers to avoid potential changes to the slice during execution
	copy(handlersCopy, handlers)

	go func(from, to State, handlerList []EventHandler) {
		ctx, cancel := context.WithTimeout(context.Background(), sm.handlerTimeout)
		defer cancel()

		for _, handler := range handlerList {
			func() {
				defer func() {
					if r := recover(); r != nil {
						// Log panic but don't affect the transition
						sm.logger.Errorf("event handler panic from %s to %s: %v\n", from, next, r)
					}
				}()
				handler(ctx, from, to)
			}()
		}
	}(from, next, handlersCopy)
}

func (sm *stateMachine) isValidTransition(currentState State, newState State) bool {
	validTransitions, ok := sm.validStateTransitions[currentState]
	if !ok {
		return false
	}
	return slices.Contains(validTransitions, newState)
}

type lock struct {
	release func()
	mu      sync.Mutex
}

func (l *lock) Release() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.release != nil {
		l.release()
		l.release = nil
	}
}
