package manager

import "context"

// Component defines the lifecycle of managed components.
type Component interface {
	// Init initializes the component and prepares it for execution.
	Init(context.Context) error

	// Start starts the component.
	Start(context.Context) error

	// Stop stops this component, potentially cleaning up any temporary
	// resources attached to it.
	Stop() error
}

// Ready is the interface for a component that can be checked for readiness.
type Ready interface {
	// Ready performs a ready check and indicates that a component is ready to run.
	Ready() error
}
