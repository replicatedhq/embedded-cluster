package types

import (
	"errors"
	"fmt"
	"time"
)

type Status struct {
	State       State     `json:"state"`
	Description string    `json:"description"`
	LastUpdated time.Time `json:"lastUpdated"`
}

type State string

const (
	StatePending   State = "Pending"
	StateRunning   State = "Running"
	StateSucceeded State = "Succeeded"
	StateFailed    State = "Failed"
)

func NewStatus() *Status {
	return &Status{
		State: StatePending,
	}
}

func ValidateStatus(status *Status) error {
	var ve *APIError

	if status == nil {
		return NewBadRequestError(errors.New("a status is required"))
	}

	switch status.State {
	case StatePending, StateRunning, StateSucceeded, StateFailed:
		// valid states
	default:
		ve = AppendFieldError(ve, "state", fmt.Errorf("invalid state: %s", status.State))
	}

	if status.Description == "" {
		ve = AppendFieldError(ve, "description", errors.New("description is required"))
	}

	return ve.ErrorOrNil()
}
