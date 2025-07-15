package types

// PatchAppConfigValuesRequest represents the request when patching the app config values
type PatchAppConfigValuesRequest struct {
	Values AppConfigValues `json:"values"`
}
