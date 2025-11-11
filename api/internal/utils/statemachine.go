package utils

import (
	"fmt"
	"slices"

	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
)

// CheckStateMismatch checks if the status state and the statemachine state are in sync. If they
// are not in sync, a StateMismatchError (409 Conflict) if the statemachine state is a
// transitionable state, otherwise an unrecoverable error is returned.
func CheckStateMismatch(stateMap map[apitypes.State]statemachine.State, transitionableStates []statemachine.State, statusState apitypes.State, statemachineState statemachine.State) error {
	expectedState, ok := stateMap[statusState]
	if !ok {
		return fmt.Errorf("unexpected status state: %s", statusState)
	}

	if expectedState != statemachineState {
		// if the statemachine state is a transitionable state, this is expected and we return a StateMismatchError
		if slices.Contains(transitionableStates, statemachineState) {
			return NewStateMismatchError(statemachineState, statusState)
		}
		return fmt.Errorf("unexpected state mismatch: statemachine state %s, status state %s", statemachineState, statusState)
	}

	return nil
}

// NewStateMismatchError returns a new StateMismatchError (409 Conflict) for the given statemachine
// state and status state.
func NewStateMismatchError(statemachineState statemachine.State, statusState apitypes.State) *apitypes.APIError {
	return apitypes.NewConflictError(fmt.Errorf("state mismatch: statemachine state %s, status state %s", statemachineState, statusState))
}
