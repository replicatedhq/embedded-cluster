package helpers

import (
	"errors"
	"fmt"
	"os"

	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	kyaml "sigs.k8s.io/yaml"
)

var (
	ErrNotALicenseFile = errors.New("not a license file")
)

// ParseEndUserConfig parses the end user configuration from the given file.
func ParseEndUserConfig(fpath string) (*embeddedclusterv1beta1.Config, error) {
	if fpath == "" {
		return nil, nil
	}
	data, err := os.ReadFile(fpath)
	if err != nil {
		return nil, fmt.Errorf("failed to read overrides file: %w", err)
	}
	var cfg embeddedclusterv1beta1.Config
	if err := kyaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal overrides file: %w", err)
	}
	return &cfg, nil
}

// ParseLicense parses the license from the given file.
func ParseLicense(fpath string) (*kotsv1beta1.License, error) {
	data, err := os.ReadFile(fpath)
	if err != nil {
		return nil, fmt.Errorf("failed to read license file: %w", err)
	}
	var license kotsv1beta1.License
	if err := kyaml.Unmarshal(data, &license); err != nil {
		return nil, ErrNotALicenseFile
	}
	return &license, nil
}

func ParseConfigValues(fpath string) (*kotsv1beta1.ConfigValues, error) {
	if fpath == "" {
		return nil, nil
	}
	data, err := os.ReadFile(fpath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config values file: %w", err)
	}
	var configValues kotsv1beta1.ConfigValues
	if err := kyaml.Unmarshal(data, &configValues); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config values file: %w", err)
	}
	return &configValues, nil
}

// ParseConfigValuesFromString parses kots ConfigValues from a YAML string
func ParseConfigValuesFromString(yamlContent string) (*kotsv1beta1.ConfigValues, error) {
	var configValues kotsv1beta1.ConfigValues
	if err := kyaml.Unmarshal([]byte(yamlContent), &configValues); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config values YAML: %w", err)
	}
	return &configValues, nil
}
