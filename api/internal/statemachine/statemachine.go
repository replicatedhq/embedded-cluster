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

	// Trigger event handlers after successful transition
	sm.triggerHandlers(fromState, nextState)

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

// triggerHandlers triggers event handlers
func (sm *stateMachine) triggerHandlers(from, next State) {
	handlers, exists := sm.eventHandlers[next]
	if !exists || len(handlers) == 0 {
		return
	}
	for _, handler := range handlers {
		err := handler.TriggerHandler(context.Background(), from, next)
		if err != nil {
			sm.logger.WithFields(logrus.Fields{"fromState": from, "toState": next}).Errorf("event handler error: %v", err)
		}
	}
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
