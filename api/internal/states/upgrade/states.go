package upgrade

import "github.com/replicatedhq/embedded-cluster/api/internal/statemachine"

// Basic upgrade states for iteration 1
const (
	StateNew                 statemachine.State = "StateNew"
	StateAppUpgrading        statemachine.State = "StateAppUpgrading"
	StateAppUpgradeSucceeded statemachine.State = "StateAppUpgradeSucceeded"
	StateAppUpgradeFailed    statemachine.State = "StateAppUpgradeFailed"
)
