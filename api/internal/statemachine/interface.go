package statemachine

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
	// ValidateTransition checks if transitions from the current state to the new states, in order,
	// are valid.
	ValidateTransition(lock Lock, nextStates ...State) error
	// Transition attempts to transition to a new state and returns an error if the transition is
	// invalid.
	Transition(lock Lock, nextState State) error
	// AcquireLock acquires a lock on the state machine.
	AcquireLock() (Lock, error)
	// IsLockAcquired checks if a lock already exists on the state machine.
	IsLockAcquired() bool
	// RegisterEventHandler registers a blocking event handler for reporting events in the state
	// machine.
	RegisterEventHandler(targetState State, handler EventHandlerFunc, options ...EventHandlerOption)
	// UnregisterEventHandler unregisters a blocking event handler for reporting events in the
	// state machine.
	UnregisterEventHandler(targetState State)
}

type Lock interface {
	// Release releases the lock.
	Release()
}
