package types

// SetAppConfigValuesRequest represents the request when setting the app config values
type SetAppConfigValuesRequest struct {
	Values map[string]string `json:"values"`
}
