package statemachine

import (
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	// StateNew is the initial state of the install process
	StateNew State = "New"
	// StateInstallationConfigured is the state of the install process when the installation is configured
	StateInstallationConfigured State = "InstallationConfigured"
	// StatePreflightsRunning is the state of the install process when the preflights are running
	StatePreflightsRunning State = "PreflightsRunning"
	// StatePreflightsSucceeded is the state of the install process when the preflights have succeeded
	StatePreflightsSucceeded State = "PreflightsSucceeded"
	// StatePreflightsFailed is the state of the install process when the preflights have failed
	StatePreflightsFailed State = "PreflightsFailed"
	// StatePreflightsFailedBypassed is the state of the install process when the preflights have failed bypassed
	StatePreflightsFailedBypassed State = "PreflightsFailedBypassed"
	// StateInfrastructureInstalling is the state of the install process when the infrastructure is being installed
	StateInfrastructureInstalling State = "InfrastructureInstalling"
	// StateSucceeded is the final state of the install process when the install has succeeded
	StateSucceeded State = "Succeeded"
	// StateFailed is the final state of the install process when the install has failed
	StateFailed State = "Failed"
)

var validStateTransitions = map[State][]State{
	StateNew:                      {StateInstallationConfigured},
	StateInstallationConfigured:   {StatePreflightsRunning},
	StatePreflightsRunning:        {StatePreflightsSucceeded, StatePreflightsFailed},
	StatePreflightsSucceeded:      {StateInfrastructureInstalling, StatePreflightsRunning, StateInstallationConfigured},
	StatePreflightsFailed:         {StatePreflightsFailedBypassed, StatePreflightsRunning, StateInstallationConfigured},
	StatePreflightsFailedBypassed: {StateInfrastructureInstalling, StatePreflightsRunning, StateInstallationConfigured},
	StateInfrastructureInstalling: {StateSucceeded, StateFailed},
	StateSucceeded:                {},
	StateFailed:                   {},
}

func TestLockAcquisitionAndRelease(t *testing.T) {
	sm := New(StateNew, validStateTransitions)
	assert.False(t, sm.IsLockAcquired())

	// Test valid lock acquisition
	lock, err := sm.AcquireLock()
	assert.NoError(t, err)
	assert.NotNil(t, lock)
	assert.True(t, sm.IsLockAcquired())

	// Test transition with lock
	err = sm.Transition(lock, StateInstallationConfigured)
	assert.NoError(t, err)
	assert.Equal(t, StateInstallationConfigured, sm.CurrentState())

	// Release lock
	lock.Release()
	assert.False(t, sm.IsLockAcquired())

	// Test double lock acquisition
	lock, err = sm.AcquireLock()
	assert.NoError(t, err)
	assert.NotNil(t, lock)
	assert.True(t, sm.IsLockAcquired())

	err = sm.Transition(lock, StatePreflightsRunning)
	assert.NoError(t, err)

	// Release lock
	lock.Release()
	assert.Equal(t, StatePreflightsRunning, sm.CurrentState())
	assert.False(t, sm.IsLockAcquired())
}

func TestDoubleLockAcquisition(t *testing.T) {
	sm := New(StateNew, validStateTransitions)
	assert.False(t, sm.IsLockAcquired())

	lock1, err := sm.AcquireLock()
	assert.NoError(t, err)
	assert.True(t, sm.IsLockAcquired())

	// Try to acquire second lock while first is held
	lock2, err := sm.AcquireLock()
	assert.Error(t, err, "second lock acquisition should fail while first is held")
	assert.Nil(t, lock2)
	assert.Contains(t, err.Error(), "lock already acquired")
	assert.True(t, sm.IsLockAcquired())

	// Release first lock
	lock1.Release()
	assert.False(t, sm.IsLockAcquired())

	// Now second lock should work
	lock2, err = sm.AcquireLock()
	assert.NoError(t, err)
	assert.NotNil(t, lock2)
	assert.True(t, sm.IsLockAcquired())

	// Release second lock
	lock2.Release()
	assert.False(t, sm.IsLockAcquired())
}

func TestLockReleaseAfterTransition(t *testing.T) {
	sm := New(StateNew, validStateTransitions)
	assert.False(t, sm.IsLockAcquired())

	lock, err := sm.AcquireLock()
	assert.NoError(t, err)
	assert.True(t, sm.IsLockAcquired())

	err = sm.Transition(lock, StateInstallationConfigured)
	assert.NoError(t, err)
	assert.True(t, sm.IsLockAcquired())

	// Release lock after transition
	lock.Release()
	assert.False(t, sm.IsLockAcquired())

	// State should remain changed
	assert.Equal(t, StateInstallationConfigured, sm.CurrentState())
}

func TestDoubleLockRelease(t *testing.T) {
	sm := New(StateNew, validStateTransitions)
	assert.False(t, sm.IsLockAcquired())

	lock, err := sm.AcquireLock()
	assert.NoError(t, err)
	assert.True(t, sm.IsLockAcquired())

	// Release lock
	lock.Release()
	assert.False(t, sm.IsLockAcquired())

	// Acquire another lock
	lock2, err := sm.AcquireLock()
	assert.NoError(t, err)
	assert.NotNil(t, lock2)
	assert.True(t, sm.IsLockAcquired())

	// Second release should not actually do anything
	lock.Release()
	assert.True(t, sm.IsLockAcquired())

	// Should not be able to acquire lock after as the other lock is still held
	nilLock, err := sm.AcquireLock()
	assert.Error(t, err, "should not be able to acquire lock after as the other lock is still held")
	assert.Nil(t, nilLock)
	assert.True(t, sm.IsLockAcquired())

	// Release the second lock
	lock2.Release()
	assert.False(t, sm.IsLockAcquired())

	// Should be able to acquire lock after the other lock is released
	lock3, err := sm.AcquireLock()
	assert.NoError(t, err)
	assert.NotNil(t, lock3)
	assert.True(t, sm.IsLockAcquired())

	lock3.Release()
	assert.False(t, sm.IsLockAcquired())
}

func TestRaceConditionMultipleGoroutines(t *testing.T) {
	sm := New(StateNew, validStateTransitions)
	assert.False(t, sm.IsLockAcquired())

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	// Start multiple goroutines trying to acquire lock simultaneously
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			lock, err := sm.AcquireLock()
			if err == nil && lock != nil {
				err = sm.Transition(lock, StateInstallationConfigured)
				if err == nil {
					mu.Lock()
					successCount++
					mu.Unlock()

					// Release the lock
					lock.Release()
				} else {
					lock.Release()
				}
			}
		}()
	}

	wg.Wait()

	// Only one transition should succeed
	assert.Equal(t, 1, successCount, "only one transition should succeed")
	assert.Equal(t, StateInstallationConfigured, sm.CurrentState())
	// There should be no lock acquired at the end
	assert.False(t, sm.IsLockAcquired())
}

func TestRaceConditionReadWrite(t *testing.T) {
	sm := New(StateNew, validStateTransitions)
	assert.False(t, sm.IsLockAcquired())

	var wg sync.WaitGroup

	// Start a goroutine that continuously reads the current state
	readDone := make(chan bool)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			_ = sm.CurrentState()
			_ = sm.IsFinalState()
		}
		readDone <- true
	}()

	// Start a goroutine that performs transitions
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Wait for reads to start
		<-readDone

		lock, err := sm.AcquireLock()
		if err == nil && lock != nil {
			err = sm.Transition(lock, StateInstallationConfigured)
			if err == nil {
				lock.Release()
			} else {
				lock.Release()
			}
		}

		lock, err = sm.AcquireLock()
		if err == nil && lock != nil {
			err = sm.Transition(lock, StatePreflightsRunning)
			if err == nil {
				lock.Release()
			} else {
				lock.Release()
			}
		}
	}()

	wg.Wait()

	// Final state should be consistent
	finalState := sm.CurrentState()
	assert.True(t, finalState == StateInstallationConfigured || finalState == StatePreflightsRunning,
		"final state should be one of the expected states")
	// There should be no lock acquired at the end
	assert.False(t, sm.IsLockAcquired())
}

func TestIsFinalState(t *testing.T) {
	finalStates := []State{
		StateSucceeded,
		StateFailed,
	}

	for state := range validStateTransitions {
		var isFinal bool
		if slices.Contains(finalStates, state) {
			isFinal = true
		}

		sm := New(state, validStateTransitions)

		if isFinal {
			assert.True(t, sm.IsFinalState(), "expected state %s to be final", state)
		} else {
			assert.False(t, sm.IsFinalState(), "expected state %s to not be final", state)
		}
	}
}

func TestFinalStateTransitionBlocking(t *testing.T) {
	finalStates := []State{StateSucceeded, StateFailed}

	for _, finalState := range finalStates {
		t.Run(string(finalState), func(t *testing.T) {
			sm := New(finalState, validStateTransitions)

			// Try to transition from final state
			lock, err := sm.AcquireLock()
			if err != nil {
				t.Fatalf("failed to acquire lock: %v", err)
			}

			err = sm.Transition(lock, StateNew)
			assert.Error(t, err, "should not be able to transition from final state %s", finalState)
			assert.Contains(t, err.Error(), "invalid transition")

			// Release the lock
			lock.Release()

			// State should remain unchanged
			assert.Equal(t, finalState, sm.CurrentState())
		})
	}
}

func TestMultiStateTransitionWithLock(t *testing.T) {
	sm := New(StateNew, validStateTransitions)
	assert.False(t, sm.IsLockAcquired())

	// Acquire lock and transition through multiple states
	lock, err := sm.AcquireLock()
	assert.NoError(t, err)
	assert.NotNil(t, lock)
	assert.True(t, sm.IsLockAcquired())

	// Transition 1: New -> StateInstallationConfigured
	err = sm.Transition(lock, StateInstallationConfigured)
	assert.NoError(t, err)
	assert.Equal(t, StateInstallationConfigured, sm.CurrentState())

	// Transition 2: StateInstallationConfigured -> StatePreflightsRunning
	err = sm.Transition(lock, StatePreflightsRunning)
	assert.NoError(t, err)
	assert.Equal(t, StatePreflightsRunning, sm.CurrentState())

	// Transition 3: StatePreflightsRunning -> StatePreflightsSucceeded
	err = sm.Transition(lock, StatePreflightsSucceeded)
	assert.NoError(t, err)
	assert.Equal(t, StatePreflightsSucceeded, sm.CurrentState())

	// Transition 4: StatePreflightsSucceeded -> StateInfrastructureInstalling
	err = sm.Transition(lock, StateInfrastructureInstalling)
	assert.NoError(t, err)
	assert.Equal(t, StateInfrastructureInstalling, sm.CurrentState())

	assert.True(t, sm.IsLockAcquired())
	// Release the lock
	lock.Release()
	assert.False(t, sm.IsLockAcquired())

	// State should be the final state in the transition chain
	assert.Equal(t, StateInfrastructureInstalling, sm.CurrentState(), "state should be the final transitioned state after lock release")
}

func TestInvalidTransition(t *testing.T) {
	sm := New(StateNew, validStateTransitions)
	assert.False(t, sm.IsLockAcquired())

	lock, err := sm.AcquireLock()
	assert.NoError(t, err)
	assert.True(t, sm.IsLockAcquired())

	// Try invalid transition
	err = sm.Transition(lock, StateSucceeded)
	assert.Error(t, err, "should not be able to transition directly from New to Succeeded")
	assert.Contains(t, err.Error(), "invalid transition")

	// State should remain unchanged
	assert.Equal(t, StateNew, sm.CurrentState())

	assert.True(t, sm.IsLockAcquired())
	lock.Release()
	assert.False(t, sm.IsLockAcquired())
}

func TestTransitionWithoutLock(t *testing.T) {
	sm := New(StateNew, validStateTransitions)

	assert.False(t, sm.IsLockAcquired())
	err := sm.Transition(nil, StateInstallationConfigured)
	assert.Error(t, err, "transition should be invalid")
	assert.Contains(t, err.Error(), "lock not acquired")
}

func TestValidateTransitionWithoutLock(t *testing.T) {
	sm := New(StateNew, validStateTransitions)

	assert.False(t, sm.IsLockAcquired())
	err := sm.ValidateTransition(nil, StateInstallationConfigured)
	assert.Error(t, err, "transition should be invalid")
	assert.Contains(t, err.Error(), "lock not acquired")
}

func TestTransitionWithNilLock(t *testing.T) {
	sm := New(StateNew, validStateTransitions)

	lock, err := sm.AcquireLock()
	assert.NoError(t, err)

	err = sm.Transition(nil, StateInstallationConfigured)
	assert.Error(t, err, "transition should be invalid")
	assert.Contains(t, err.Error(), "lock mismatch")

	lock.Release()
}

func TestValidateTransitionWithNilLock(t *testing.T) {
	sm := New(StateNew, validStateTransitions)

	lock, err := sm.AcquireLock()
	assert.NoError(t, err)

	err = sm.ValidateTransition(nil, StateInstallationConfigured)
	assert.Error(t, err, "transition should be invalid")
	assert.Contains(t, err.Error(), "lock mismatch")

	lock.Release()
}

func TestTransitionWithWrongLock(t *testing.T) {
	sm := New(StateNew, validStateTransitions)

	lock, err := sm.AcquireLock()
	assert.NoError(t, err)
	lock.Release()

	lock2, err := sm.AcquireLock()
	assert.NoError(t, err)

	err = sm.Transition(lock, StateInstallationConfigured)
	assert.Error(t, err, "transition should be invalid")
	assert.Contains(t, err.Error(), "lock mismatch")

	lock2.Release()
}

func TestValidateTransitionWithWrongLock(t *testing.T) {
	sm := New(StateNew, validStateTransitions)

	lock, err := sm.AcquireLock()
	assert.NoError(t, err)
	lock.Release()

	lock2, err := sm.AcquireLock()
	assert.NoError(t, err)

	err = sm.ValidateTransition(lock, StateInstallationConfigured)
	assert.Error(t, err, "transition should be invalid")
	assert.Contains(t, err.Error(), "lock mismatch")

	lock2.Release()
}

func TestValidateTransitionWithNonExistentState(t *testing.T) {
	sm := New(StateNew, validStateTransitions)
	lock, err := sm.AcquireLock()
	assert.NoError(t, err)

	// Test with a state that doesn't exist in the transition map
	nonExistentState := State("NonExistentState")
	err = sm.ValidateTransition(lock, nonExistentState)
	assert.Error(t, err, "transition to non-existent state should be invalid")
	assert.Contains(t, err.Error(), "invalid transition")
	assert.Contains(t, err.Error(), string(StateNew))
	assert.Contains(t, err.Error(), string(nonExistentState))

	lock.Release()
}

func TestValidateTransitionStateConsistency(t *testing.T) {
	sm := New(StateNew, validStateTransitions)
	lock, err := sm.AcquireLock()
	assert.NoError(t, err)

	// Validate a transition
	err = sm.ValidateTransition(lock, StateInstallationConfigured)
	assert.NoError(t, err, "transition should be valid")

	// State should remain unchanged after validation
	assert.Equal(t, StateNew, sm.CurrentState(), "state should not change after validation")

	// Actually perform the transition
	err = sm.Transition(lock, StateInstallationConfigured)
	assert.NoError(t, err, "transition should succeed")
	assert.Equal(t, StateInstallationConfigured, sm.CurrentState(), "state should change after transition")

	lock.Release()
}

func TestValidateTransitionEdgeCases(t *testing.T) {
	// Test with empty transition map
	emptyTransitions := make(map[State][]State)
	sm := New(StateNew, emptyTransitions)
	lock, err := sm.AcquireLock()
	assert.NoError(t, err)

	// Any transition should be invalid with empty transition map
	err = sm.ValidateTransition(lock, StateInstallationConfigured)
	assert.Error(t, err, "transition should be invalid with empty transition map")
	assert.Contains(t, err.Error(), "invalid transition")

	lock.Release()

	// Test with state that has no valid transitions (final state)
	finalStateTransitions := map[State][]State{
		StateSucceeded: {},
		StateFailed:    {},
	}
	sm = New(StateSucceeded, finalStateTransitions)
	lock, err = sm.AcquireLock()
	assert.NoError(t, err)

	// Any transition from final state should be invalid
	err = sm.ValidateTransition(lock, StateNew)
	assert.Error(t, err, "transition from final state should be invalid")
	assert.Contains(t, err.Error(), "invalid transition")

	lock.Release()
}
