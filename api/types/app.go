package types

import (
	"strings"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

// AppConfig represents the configuration for an app. This is an alias for the
// kotsv1beta1.ConfigSpec type.
type AppConfig = kotsv1beta1.ConfigSpec

// AppConfigValue represents a configuration value for the App with optional metadata
type AppConfigValue struct {
	Default        string `json:"default,omitempty"`
	Value          string `json:"value"`
	Data           string `json:"data,omitempty"`
	ValuePlaintext string `json:"valuePlaintext,omitempty"`
	DataPlaintext  string `json:"dataPlaintext,omitempty"`
	Filename       string `json:"filename,omitempty"`
	RepeatableItem string `json:"repeatableItem,omitempty"`
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
		// Temporary fix for https://app.shortcut.com/replicated/story/129708/template-execution-fails-when-empty-user-config-values-override-generated-defaults
		// TODO: Remove this block of code and add a unit test for this function once a fix has been implemented
		temporaryFixValue := value.Value
		if strings.HasSuffix(key, "_json") && temporaryFixValue == "" {
			temporaryFixValue = "{}"
		}

		configValues[key] = AppConfigValue{
			Default:        value.Default,
			Value:          temporaryFixValue,
			Data:           value.Data,
			ValuePlaintext: value.ValuePlaintext,
			DataPlaintext:  value.DataPlaintext,
			Filename:       value.Filename,
			RepeatableItem: value.RepeatableItem,
		}
	}

	return configValues
}
