package types

import (
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	kotsv1beta2 "github.com/replicatedhq/kotskinds/apis/kots/v1beta2"
)

// AppConfig represents the configuration for an app. This is an alias for the
// kotsv1beta1.ConfigSpec type.
type AppConfig = kotsv1beta1.ConfigSpec

// AppConfigValue represents a configuration value for te App with optional metadata
type AppConfigValue struct {
	Value    string `json:"value"`
	Filename string `json:"filename,omitempty"`
}

// AppConfigValues represents a map of configuration values for the App.
type AppConfigValues map[string]AppConfigValue

// AppInstall represents the current state of app application installation with detailed chart
// components, following the same schema pattern as types.Infra.
type AppInstall struct {
	Components []AppComponent `json:"components"`
	Status     Status         `json:"status"`
	Logs       string         `json:"logs"`
}

// AppComponent represents an individual chart component within the app
// Following the same schema pattern as types.InfraComponent
type AppComponent struct {
	Name   string `json:"name"`   // Chart name
	Status Status `json:"status"` // Uses existing Status type
}

// InstallableHelmChart represents a Helm chart with pre-processed values ready for installation
type InstallableHelmChart struct {
	Archive []byte
	Values  map[string]any
	CR      *kotsv1beta2.HelmChart
}
