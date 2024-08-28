// Package autopilot provides some tooling to deal with autopilot kubernetes structs.
// So far this is very simple but I can see this growing to be more complex as we
// add more needs regarding parsing and intepretation of autopilot objects.
package autopilot

import (
	"fmt"

	"github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	"github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
)

var msgs = map[v1beta2.PlanStateType]string{
	"":                           "Upgrade not yet scheduled",
	core.PlanSchedulable:         "Upgrade in being prepared",
	core.PlanSchedulableWait:     "Upgrade is being prepared",
	core.PlanCompleted:           "Upgrade has been completed",
	core.PlanWarning:             "Upgrade has failed with warnings",
	core.PlanInconsistentTargets: "Upgrade has failed (malformed request)",
	core.PlanIncompleteTargets:   "Upgrade has failed (malformed request)",
	core.PlanRestricted:          "Upgrade has failed (malformed request)",
	core.PlanMissingSignalNode:   "Upgrade has failed (missing signal node)",
	core.PlanApplyFailed:         "Upgrade apply has failed",
}

// Reason for state returns a descriptive string for the given plan state.
func ReasonForState(plan v1beta2.Plan) string {
	state := plan.Status.State
	if _, ok := msgs[state]; !ok {
		return fmt.Sprintf("Upgrade is in an unknown state (%s)", state)
	}
	return msgs[plan.Status.State]
}

// HasThePlanEnded returns true if the plan execution has ended, with a failure or
// not. This is useful to know if we should wait for the plan to finish or we can
// delete it and create a new one.
func HasThePlanEnded(plan v1beta2.Plan) bool {
	if plan.Status.State == "" {
		return false
	}
	if plan.Status.State == core.PlanSchedulable {
		return false
	}
	if plan.Status.State == core.PlanSchedulableWait {
		return false
	}
	return true
}

// HasPlanSucceeded returns true if the plan has been completed successfully.
func HasPlanSucceeded(plan v1beta2.Plan) bool {
	return plan.Status.State == core.PlanCompleted
}

// HasPlanFailed returns true if the plan has failed.
func HasPlanFailed(plan v1beta2.Plan) bool {
	switch plan.Status.State {
	case core.PlanIncompleteTargets:
		return true
	case core.PlanInconsistentTargets:
		return true
	case core.PlanRestricted:
		return true
	case core.PlanWarning:
		return true
	case core.PlanMissingSignalNode:
		return true
	case core.PlanApplyFailed:
		return true
	default:
		// unknown state
		return HasThePlanEnded(plan) && !HasPlanSucceeded(plan)
	}
}
