package types

// PatchAppConfigValuesRequest represents the request when patching the app config values
type PatchAppConfigValuesRequest struct {
	Values AppConfigValues `json:"values"`
}

// TemplateAppConfigRequest represents the request when templating the app config
type TemplateAppConfigRequest struct {
	Values AppConfigValues `json:"values"`
}
