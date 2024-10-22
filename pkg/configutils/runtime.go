package configutils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"sigs.k8s.io/yaml"
)

func WriteRuntimeConfig(spec *v1beta1.RuntimeConfigSpec) error {
	if spec == nil {
		return nil
	}

	location := defaults.PathToECConfig()

	err := os.MkdirAll(filepath.Dir(location), 0700)
	if err != nil {
		return fmt.Errorf("unable to create runtime config directory: %w", err)
	}

	// check if the file already exists, if it does delete it
	err = helpers.RemoveAll(location)
	if err != nil {
		return fmt.Errorf("unable to remove existing runtime config: %w", err)
	}

	yml, err := yaml.Marshal(spec)
	if err != nil {
		return fmt.Errorf("unable to marshal runtime config: %w", err)
	}

	err = os.WriteFile(location, yml, 0600)
	if err != nil {
		return fmt.Errorf("unable to write runtime config: %w", err)
	}

	return nil
}

func ReadRuntimeConfig() (*v1beta1.RuntimeConfigSpec, error) {
	location := defaults.PathToECConfig()

	data, err := os.ReadFile(location)
	if err != nil {
		return nil, fmt.Errorf("unable to read runtime config: %w", err)
	}

	var spec v1beta1.RuntimeConfigSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("unable to unmarshal runtime config: %w", err)
	}

	return &spec, nil
}
