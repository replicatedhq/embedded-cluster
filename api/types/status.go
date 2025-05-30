package types

import "time"

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
