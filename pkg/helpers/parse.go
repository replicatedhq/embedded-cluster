package helpers

import (
	"fmt"
	"os"

	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	kyaml "sigs.k8s.io/yaml"
)

// ParseEndUserConfig parses the end user configuration from the given file.
func ParseEndUserConfig(fpath string) (*embeddedclusterv1beta1.Config, error) {
	if fpath == "" {
		return nil, nil
	}
	data, err := os.ReadFile(fpath)
	if err != nil {
		return nil, fmt.Errorf("unable to read overrides file: %w", err)
	}
	var cfg embeddedclusterv1beta1.Config
	if err := kyaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal overrides file: %w", err)
	}
	return &cfg, nil
}

// ParseLicense parses the license from the given file.
func ParseLicense(fpath string) (*kotsv1beta1.License, error) {
	data, err := os.ReadFile(fpath)
	if err != nil {
		return nil, fmt.Errorf("unable to read license file: %w", err)
	}
	var license kotsv1beta1.License
	if err := kyaml.Unmarshal(data, &license); err != nil {
		return nil, fmt.Errorf("unable to unmarshal license file: %w", err)
	}
	return &license, nil
}
