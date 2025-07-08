package statemachine

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/sirupsen/logrus"
)

// stateMachine manages the state transitions for the install process
type stateMachine struct {
	currentState          State
	validStateTransitions map[State][]State
	lock                  *lock
	mu                    sync.RWMutex
	eventHandlers         map[State][]EventHandler
	logger                logrus.FieldLogger
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

func (sm *stateMachine) ValidateTransition(lock Lock, nextStates ...State) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.lock == nil {
		return fmt.Errorf("lock not acquired")
	} else if sm.lock != lock {
		return fmt.Errorf("lock mismatch")
	}

	currentState := sm.currentState
	for _, nextState := range nextStates {
		if !sm.isValidTransition(currentState, nextState) {
			return fmt.Errorf("invalid transition from %s to %s", currentState, nextState)
		}
		currentState = nextState
	}

	return nil
}

func (sm *stateMachine) Transition(lock Lock, nextStates ...State) (finalError error) {
	sm.mu.Lock()
	defer func() {
		if finalError != nil {
			sm.mu.Unlock()
		}
	}()

	if len(nextStates) == 0 {
		return fmt.Errorf("no states to transition to")
	}

	if sm.lock == nil {
		return fmt.Errorf("lock not acquired")
	} else if sm.lock != lock {
		return fmt.Errorf("lock mismatch")
	}

	currentState := sm.currentState
	for _, nextState := range nextStates {
		if !sm.isValidTransition(currentState, nextState) {
			return fmt.Errorf("invalid transition from %s to %s", currentState, nextState)
		}
		currentState = nextState
	}

	safeHandlers := make(map[State][]EventHandler)
	for _, nextState := range nextStates {
		// Trigger event handlers after successful transition
		handlers, exists := sm.eventHandlers[nextState]
		if !exists || len(handlers) == 0 {
			continue
		}

		sh := make([]EventHandler, len(handlers))
		copy(sh, handlers) // Copy to avoid holding the lock while calling handlers
		safeHandlers[nextState] = sh
	}

	fromState := sm.currentState
	sm.currentState = nextStates[len(nextStates)-1]

	// We can release the lock here since the transition is successful and there will be no further operations to the state machine internal state
	sm.mu.Unlock()

	for nextState, handlers := range safeHandlers {
		for _, handler := range handlers {
			err := handler.TriggerHandler(context.Background(), fromState, nextState)
			if err != nil {
				sm.logger.
					WithError(err).
					WithFields(logrus.Fields{"fromState": fromState, "toState": nextState}).
					Error("event handler error")
			}
		}
		fromState = nextState
	}

	return nil
}

func (sm *stateMachine) RegisterEventHandler(targetState State, handler EventHandlerFunc, options ...EventHandlerOption) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.eventHandlers[targetState] = append(sm.eventHandlers[targetState], NewEventHandler(handler, options...))
}

func (sm *stateMachine) UnregisterEventHandler(targetState State) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.eventHandlers, targetState)
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
