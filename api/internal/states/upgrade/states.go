package upgrade

import "github.com/replicatedhq/embedded-cluster/api/internal/statemachine"

// Upgrade states
const (
	StateNew              statemachine.State = "StateNew"
	StateAppUpgrading     statemachine.State = "StateAppUpgrading"
	StateAppUpgradeFailed statemachine.State = "StateAppUpgradeFailed"
	StateSucceeded        statemachine.State = "StateSucceeded"
)
