package helpers

import (
	"fmt"
	"os"

	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	kyaml "sigs.k8s.io/yaml"
)

type ErrNotALicenseFile struct {
	Err error
}

func (e ErrNotALicenseFile) Error() string {
	return e.Err.Error()
}

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

// ParseLicense parses the license from the given file and returns a LicenseWrapper
// that provides version-agnostic access to both v1beta1 and v1beta2 licenses.
func ParseLicense(fpath string) (licensewrapper.LicenseWrapper, error) {
	data, err := os.ReadFile(fpath)
	if err != nil {
		return licensewrapper.LicenseWrapper{}, fmt.Errorf("unable to read license file: %w", err)
	}
	return ParseLicenseFromBytes(data)
}

// ParseLicenseFromBytes parses license data from bytes and returns a LicenseWrapper
// that provides version-agnostic access to both v1beta1 and v1beta2 licenses.
func ParseLicenseFromBytes(data []byte) (licensewrapper.LicenseWrapper, error) {
	wrapper, err := licensewrapper.LoadLicenseFromBytes(data)
	if err != nil {
		return licensewrapper.LicenseWrapper{}, fmt.Errorf("failed to load license: %w", err)
	}
	return wrapper, nil
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
