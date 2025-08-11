package types

// LinuxInfraSetupRequest represents a request to set up infrastructure
type LinuxInfraSetupRequest struct {
	IgnoreHostPreflights bool `json:"ignoreHostPreflights"`
}

// PatchAppConfigValuesRequest represents the request when patching the app config values
type PatchAppConfigValuesRequest struct {
	Values AppConfigValues `json:"values"`
}

// TemplateAppConfigRequest represents the request when templating the app config
type TemplateAppConfigRequest struct {
	Values AppConfigValues `json:"values"`
}

// InstallAppRequest represents the request when installing the app
type InstallAppRequest struct {
	IgnoreAppPreflights bool `json:"ignoreAppPreflights"`
}
