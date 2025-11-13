package types

import (
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

// AppConfig represents the configuration for an app. This is an alias for the
// kotsv1beta1.ConfigSpec type.
type AppConfig = kotsv1beta1.ConfigSpec

// AppConfigValue represents a configuration value for the App with optional metadata
type AppConfigValue struct {
	Default        string `json:"default,omitempty" validate:"optional"`
	Value          string `json:"value"`
	Data           string `json:"data,omitempty" validate:"optional"`
	DataPlaintext  string `json:"dataPlaintext,omitempty" validate:"optional"`
	Filename       string `json:"filename,omitempty" validate:"optional"`
	RepeatableItem string `json:"repeatableItem,omitempty" validate:"optional"`
}

// AppConfigValues represents a map of configuration values for the App.
type AppConfigValues map[string]AppConfigValue

// AppInstall represents the current state of app installation
type AppInstall struct {
	Status Status `json:"status"`
	Logs   string `json:"logs"`
}

// ConvertToAppConfigValues converts kots ConfigValues to AppConfigValues format
func ConvertToAppConfigValues(kotsConfigValues *kotsv1beta1.ConfigValues) AppConfigValues {
	if kotsConfigValues == nil {
		return nil
	}

	configValues := make(AppConfigValues)
	for key, value := range kotsConfigValues.Spec.Values {
		// password types will have ValuePlaintext set instead of Value. Let's not break compatibility for now
		finalValue := value.Value
		if value.Value == "" && value.ValuePlaintext != "" {
			finalValue = value.ValuePlaintext
		}
		configValues[key] = AppConfigValue{
			Default:        value.Default,
			Value:          finalValue,
			Data:           value.Data,
			DataPlaintext:  value.DataPlaintext,
			Filename:       value.Filename,
			RepeatableItem: value.RepeatableItem,
		}
	}

	return configValues
}
