package statemachine

import (
	"fmt"
	"slices"
	"sync"
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
}

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
}

// New creates a new state machine starting in the given state with the given valid state
// transitions.
func New(currentState State, validStateTransitions map[State][]State) *stateMachine {
	return &stateMachine{
		currentState:          currentState,
		validStateTransitions: validStateTransitions,
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

	sm.currentState = nextState

	return nil
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
