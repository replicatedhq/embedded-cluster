package types

const (
	HealthStatusOK    = "ok"
	HealthStatusError = "error"
)

type Health struct {
	Status string `json:"status"`
}
