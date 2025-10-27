package configutils

import (
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"sigs.k8s.io/yaml"
)

type gvk struct {
	ApiVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
}

// ValidateKotsConfigValues checks if the file exists and has the 'kots.io/v1beta1 ConfigValues' GVK
func ValidateKotsConfigValues(filename string) error {
	contents, err := helpers.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("config values file not found")
		}
		return fmt.Errorf("unable to read config values file: %w", err)
	}

	var kind gvk

	err = yaml.Unmarshal(contents, &kind)
	if err != nil {
		return fmt.Errorf("unable to unmarshal config values file: %w", err)
	}
	if kind.ApiVersion != "kots.io/v1beta1" || kind.Kind != "ConfigValues" {
		return fmt.Errorf("config values file is not a valid kots config values file")
	}

	return nil
}
