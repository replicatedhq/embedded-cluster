package statemachine

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func TestEventHandlerRegistrationAndTriggering(t *testing.T) {
	sm := New(StateNew, validStateTransitions)

	// Create a mock handler
	mockHandler := &MockEventHandler{}
	mockHandler.On("Handle", mock.Anything, StateNew, StateInstallationConfigured).Return()

	// Register event handler
	handler := func(ctx context.Context, from, to State) {
		mockHandler.Handle(ctx, from, to)
	}

	sm.RegisterEventHandler(StateInstallationConfigured, handler)

	// Perform transition
	lock, err := sm.AcquireLock()
	assert.NoError(t, err)

	err = sm.Transition(lock, StateInstallationConfigured)
	assert.NoError(t, err)

	lock.Release()

	// Use assert.Eventually to verify handler was called with correct parameters
	assert.Eventually(t, func() bool {
		return mockHandler.AssertCalled(t, "Handle", mock.Anything, StateNew, StateInstallationConfigured)
	}, time.Second, time.Millisecond*50)

	mockHandler.AssertExpectations(t)
}

func TestEventHandlerMultipleHandlers(t *testing.T) {
	sm := New(StateNew, validStateTransitions)

	// Create mock handlers
	mockHandler1 := &MockEventHandler{}
	mockHandler1.On("Handle", mock.Anything, StateNew, StateInstallationConfigured).Return()

	mockHandler2 := &MockEventHandler{}
	mockHandler2.On("Handle", mock.Anything, StateNew, StateInstallationConfigured).Return()

	// Register multiple handlers for the same state
	handler1 := func(ctx context.Context, from, to State) {
		mockHandler1.Handle(ctx, from, to)
	}

	handler2 := func(ctx context.Context, from, to State) {
		mockHandler2.Handle(ctx, from, to)
	}

	sm.RegisterEventHandler(StateInstallationConfigured, handler1)
	sm.RegisterEventHandler(StateInstallationConfigured, handler2)

	// Perform transition
	lock, err := sm.AcquireLock()
	assert.NoError(t, err)

	err = sm.Transition(lock, StateInstallationConfigured)
	assert.NoError(t, err)

	lock.Release()

	// Use assert.Eventually to verify handler was called with correct parameters
	assert.Eventually(t, func() bool {
		return mockHandler1.AssertCalled(t, "Handle", mock.Anything, StateNew, StateInstallationConfigured)
	}, time.Second, time.Millisecond*50, "mockHandler1 was not called")

	assert.Eventually(t, func() bool {
		return mockHandler2.AssertCalled(t, "Handle", mock.Anything, StateNew, StateInstallationConfigured)
	}, time.Second, time.Millisecond*50, "mockHandler2 was not called")

	mockHandler1.AssertExpectations(t)
	mockHandler2.AssertExpectations(t)
}

func TestEventHandlerUnregistration(t *testing.T) {
	sm := New(StateNew, validStateTransitions)

	mockHandler := &MockEventHandler{}

	handler := func(ctx context.Context, from, to State) {
		mockHandler.Handle(ctx, from, to)
	}

	sm.RegisterEventHandler(StateInstallationConfigured, handler)

	// Unregister handlers
	sm.UnregisterEventHandler(StateInstallationConfigured)

	// Perform transition
	lock, err := sm.AcquireLock()
	assert.NoError(t, err)

	err = sm.Transition(lock, StateInstallationConfigured)
	assert.NoError(t, err)

	lock.Release()

	// Use assert.Eventually to wait for the state to change
	assert.Eventually(t, func() bool {
		return sm.currentState == StateInstallationConfigured
	}, time.Second, time.Millisecond*50, "failed to transition to StateInstallationConfigured")
	// Verify that the handler was not called
	mockHandler.AssertNotCalled(t, "Handle", mock.Anything, StateNew, StateInstallationConfigured)
	mockHandler.AssertExpectations(t)
}

func TestEventHandlerPanicRecovery(t *testing.T) {
	sm := New(StateNew, validStateTransitions)

	mockPanicHandler := &MockEventHandler{}
	mockPanicHandler.On("Handle", mock.Anything, StateNew, StateInstallationConfigured).Return()

	// Register panicking handler
	panicHandler := func(ctx context.Context, from, to State) {
		mockPanicHandler.Handle(ctx, from, to)
		panic("test panic")
	}

	mockNormalHandler := &MockEventHandler{}
	mockNormalHandler.On("Handle", mock.Anything, StateNew, StateInstallationConfigured).Return()

	// Register normal handler
	normalHandler := func(ctx context.Context, from, to State) {
		mockNormalHandler.Handle(ctx, from, to)
	}

	sm.RegisterEventHandler(StateInstallationConfigured, panicHandler)
	sm.RegisterEventHandler(StateInstallationConfigured, normalHandler)

	// Perform transition
	lock, err := sm.AcquireLock()
	assert.NoError(t, err)

	err = sm.Transition(lock, StateInstallationConfigured)
	assert.NoError(t, err)

	lock.Release()

	// Use assert.Eventually to verify handler was called with correct parameters
	assert.Eventually(t, func() bool {
		return mockPanicHandler.AssertCalled(t, "Handle", mock.Anything, StateNew, StateInstallationConfigured)
	}, time.Second, time.Millisecond*50, "mockPanicHandler was not called")

	assert.Eventually(t, func() bool {
		return mockNormalHandler.AssertCalled(t, "Handle", mock.Anything, StateNew, StateInstallationConfigured)
	}, time.Second, time.Millisecond*50, "mockNormalHandler was not called")

	mockPanicHandler.AssertExpectations(t)
	mockNormalHandler.AssertExpectations(t)
	// Verify state machine is still in correct state
	assert.Equal(t, StateInstallationConfigured, sm.CurrentState())
}

func TestEventHandlerContextTimeout(t *testing.T) {
	sm := New(StateNew, validStateTransitions)

	mockHandler := &MockEventHandler{}
	mockHandler.On("Handle", mock.Anything, StateNew, StateInstallationConfigured).Return()

	handler := func(ctx context.Context, from, to State) {
		mockHandler.Handle(ctx, from, to)
	}

	sm.RegisterEventHandler(StateInstallationConfigured, handler, WithHandlerTimeout(time.Millisecond))

	// Perform transition
	lock, err := sm.AcquireLock()
	assert.NoError(t, err)

	err = sm.Transition(lock, StateInstallationConfigured)
	assert.NoError(t, err)

	lock.Release()

	// Verify handler was called and context was cancelled
	assert.Eventually(t, func() bool {
		return mockHandler.AssertCalled(t, "Handle", mock.Anything, StateNew, StateInstallationConfigured)
	}, time.Second, time.Millisecond*50, "mockHandler was not called")

	mockHandler.AssertExpectations(t)
	// State machine correctly transitioned
	assert.Equal(t, StateInstallationConfigured, sm.CurrentState())
}

func TestEventHandlerDifferentStates(t *testing.T) {
	tests := []struct {
		name              string
		registerState     State
		transitionToState State
		shouldTrigger     bool
	}{
		{
			name:              "handler for target state should trigger",
			registerState:     StateInstallationConfigured,
			transitionToState: StateInstallationConfigured,
			shouldTrigger:     true,
		},
		{
			name:              "handler for different state should not trigger",
			registerState:     StatePreflightsRunning,
			transitionToState: StateInstallationConfigured,
			shouldTrigger:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := New(StateNew, validStateTransitions)

			mockHandler := &MockEventHandler{}
			if tt.shouldTrigger {
				mockHandler.On("Handle", mock.Anything, StateNew, tt.transitionToState).Return()
			}

			handler := func(ctx context.Context, from, to State) {
				mockHandler.Handle(ctx, from, to)
			}

			sm.RegisterEventHandler(tt.registerState, handler)

			// Perform transition
			lock, err := sm.AcquireLock()
			assert.NoError(t, err)

			err = sm.Transition(lock, tt.transitionToState)
			assert.NoError(t, err)

			lock.Release()

			if tt.shouldTrigger {
				assert.Eventually(t, func() bool {
					return mockHandler.AssertCalled(t, "Handle", mock.Anything, StateNew, tt.transitionToState)
				}, time.Second, time.Millisecond*50, "mockHandler was not called")
				mockHandler.AssertExpectations(t)
			} else {
				// Use assert.Eventually to wait for the state to change, then verify no calls
				assert.Eventually(t, func() bool {
					return sm.CurrentState() == tt.transitionToState
				}, time.Second, time.Millisecond*50, "failed to transition to target state")
				mockHandler.AssertNotCalled(t, "Handle", mock.Anything, StateNew, tt.transitionToState)
				mockHandler.AssertExpectations(t)
			}
		})
	}
}

func TestEventHandlerConcurrentRegistration(t *testing.T) {
	sm := New(StateNew, validStateTransitions)

	numHandlers := 10
	mockHandlers := make([]*MockEventHandler, numHandlers)
	var wg sync.WaitGroup
	wg.Add(numHandlers)

	// Initialize mock handlers
	for i := 0; i < numHandlers; i++ {
		mockHandlers[i] = &MockEventHandler{}
		mockHandlers[i].On("Handle", mock.Anything, StateNew, StateInstallationConfigured).Return()
	}

	// Register handlers concurrently
	for i := 0; i < numHandlers; i++ {
		i := i // capture loop variable
		go func() {
			defer wg.Done()
			handler := func(ctx context.Context, from, to State) {
				mockHandlers[i].Handle(ctx, from, to)
			}
			sm.RegisterEventHandler(StateInstallationConfigured, handler)
		}()
	}

	wg.Wait()

	// Perform transition
	lock, err := sm.AcquireLock()
	assert.NoError(t, err)

	err = sm.Transition(lock, StateInstallationConfigured)
	assert.NoError(t, err)

	lock.Release()

	// Verify all handlers were called using assert.Eventually
	for i := 0; i < numHandlers; i++ {
		i := i // capture loop variable
		assert.Eventually(t, func() bool {
			return mockHandlers[i].AssertCalled(t, "Handle", mock.Anything, StateNew, StateInstallationConfigured)
		}, time.Second, time.Millisecond*50, "mockHandler %d was not called", i)
		mockHandlers[i].AssertExpectations(t)
	}
}

func TestValidateTransitionMultipleStates(t *testing.T) {
	tests := []struct {
		name          string
		initialState  State
		nextStates    []State
		expectedError string
		description   string
	}{
		{
			name:          "valid multi-state transition",
			initialState:  StateNew,
			nextStates:    []State{StateInstallationConfigured, StatePreflightsRunning},
			expectedError: "",
			description:   "should validate a valid chain of transitions",
		},
		{
			name:          "valid single state transition",
			initialState:  StateNew,
			nextStates:    []State{StateInstallationConfigured},
			expectedError: "",
			description:   "should validate a single valid transition",
		},
		{
			name:          "invalid first transition",
			initialState:  StateNew,
			nextStates:    []State{StateSucceeded, StateInstallationConfigured},
			expectedError: "invalid transition from New to Succeeded",
			description:   "should fail on first invalid transition",
		},
		{
			name:          "invalid middle transition",
			initialState:  StateNew,
			nextStates:    []State{StateInstallationConfigured, StateSucceeded},
			expectedError: "invalid transition from InstallationConfigured to Succeeded",
			description:   "should fail on middle invalid transition",
		},
		{
			name:          "empty states list",
			initialState:  StateNew,
			nextStates:    []State{},
			expectedError: "",
			description:   "should succeed with empty states list",
		},
		{
			name:          "complex valid chain",
			initialState:  StateNew,
			nextStates:    []State{StateInstallationConfigured, StatePreflightsRunning, StatePreflightsSucceeded, StateInfrastructureInstalling},
			expectedError: "",
			description:   "should validate a complex chain of valid transitions",
		},
		{
			name:          "transition from final state",
			initialState:  StateSucceeded,
			nextStates:    []State{StateNew},
			expectedError: "invalid transition from Succeeded to New",
			description:   "should fail when trying to transition from final state",
		},
		{
			name:          "non-existent state in chain",
			initialState:  StateNew,
			nextStates:    []State{StateInstallationConfigured, State("NonExistentState")},
			expectedError: "invalid transition from InstallationConfigured to NonExistentState",
			description:   "should fail when encountering non-existent state in chain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := New(tt.initialState, validStateTransitions)
			lock, err := sm.AcquireLock()
			assert.NoError(t, err)

			// Validate the transition chain
			err = sm.ValidateTransition(lock, tt.nextStates...)

			if tt.expectedError != "" {
				assert.Error(t, err, tt.description)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err, tt.description)
			}

			// Verify that the state machine state remains unchanged after validation
			assert.Equal(t, tt.initialState, sm.CurrentState(), "state should remain unchanged after validation")

			lock.Release()
		})
	}
}

func TestTransitionMultipleStates(t *testing.T) {
	tests := []struct {
		name               string
		initialState       State
		nextStates         []State
		expectedFinalState State
		expectedError      string
		description        string
	}{
		{
			name:               "valid multi-state transition",
			initialState:       StateNew,
			nextStates:         []State{StateInstallationConfigured, StatePreflightsRunning},
			expectedFinalState: StatePreflightsRunning,
			expectedError:      "",
			description:        "should transition through multiple valid states",
		},
		{
			name:               "valid single state transition",
			initialState:       StateNew,
			nextStates:         []State{StateInstallationConfigured},
			expectedFinalState: StateInstallationConfigured,
			expectedError:      "",
			description:        "should transition to single valid state",
		},
		{
			name:               "invalid first transition",
			initialState:       StateNew,
			nextStates:         []State{StateSucceeded, StateInstallationConfigured},
			expectedFinalState: StateNew,
			expectedError:      "invalid transition from New to Succeeded",
			description:        "should fail on first invalid transition and not change state",
		},
		{
			name:               "invalid middle transition",
			initialState:       StateNew,
			nextStates:         []State{StateInstallationConfigured, StateSucceeded},
			expectedFinalState: StateNew,
			expectedError:      "invalid transition from InstallationConfigured to Succeeded",
			description:        "should fail on middle invalid transition and not change state",
		},
		{
			name:               "empty states list should fail",
			initialState:       StateNew,
			nextStates:         []State{},
			expectedFinalState: StateNew,
			expectedError:      "no states to transition to",
			description:        "should fail with empty states list",
		},
		{
			name:               "complex valid chain",
			initialState:       StateNew,
			nextStates:         []State{StateInstallationConfigured, StatePreflightsRunning, StatePreflightsSucceeded, StateInfrastructureInstalling},
			expectedFinalState: StateInfrastructureInstalling,
			expectedError:      "",
			description:        "should transition through complex chain of valid states",
		},
		{
			name:               "transition from final state",
			initialState:       StateSucceeded,
			nextStates:         []State{StateNew},
			expectedFinalState: StateSucceeded,
			expectedError:      "invalid transition from Succeeded to New",
			description:        "should fail when trying to transition from final state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := New(tt.initialState, validStateTransitions)
			lock, err := sm.AcquireLock()
			assert.NoError(t, err)

			// Perform the transition chain
			err = sm.Transition(lock, tt.nextStates...)

			if tt.expectedError != "" {
				assert.Error(t, err, tt.description)
				assert.Contains(t, err.Error(), tt.expectedError)
				// State should remain unchanged on error
				assert.Equal(t, tt.initialState, sm.CurrentState(), "state should remain unchanged on transition error")
			} else {
				assert.NoError(t, err, tt.description)
				// State should be updated to final state in chain
				assert.Equal(t, tt.expectedFinalState, sm.CurrentState(), "state should be updated to final state in chain")
			}

			lock.Release()
		})
	}
}

// MockEventHandler is a mock for event handler testing
type MockEventHandler struct {
	mock.Mock
}

func (m *MockEventHandler) Handle(ctx context.Context, from, to State) {
	m.Called(ctx, from, to)
}
