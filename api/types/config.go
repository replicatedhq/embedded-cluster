package types

import (
	"strings"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

// ConvertToAppConfigValues converts kots ConfigValues to AppConfigValues format
func ConvertToAppConfigValues(kotsConfigValues *kotsv1beta1.ConfigValues) AppConfigValues {
	if kotsConfigValues == nil {
		return nil
	}
	configValues := make(AppConfigValues)
	for key, value := range kotsConfigValues.Spec.Values {
		configValues[key] = ConvertConfigValue(key, value)
	}

	return configValues
}

// ConvertConfigValue converts a single kots ConfigValue to AppConfigValue format
func ConvertConfigValue(key string, value kotsv1beta1.ConfigValue) AppConfigValue {
	// Apply JSON fix for empty values that templates expect to parse as JSON
	effectiveValue := value.Value
	if strings.HasSuffix(key, "_json") && effectiveValue == "" {
		effectiveValue = "{}"
	}

	return AppConfigValue{
		Default:        value.Default,
		Value:          effectiveValue,
		Data:           value.Data,
		ValuePlaintext: value.ValuePlaintext,
		DataPlaintext:  value.DataPlaintext,
		Filename:       value.Filename,
		RepeatableItem: value.RepeatableItem,
	}
}
