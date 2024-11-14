package configutils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/goods"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

// sysctlConfigPath is the path to the sysctl config file that is used to configure
// the embedded cluster. This could have been a constant but we want to be able to
// override it for testing purposes.
var sysctlConfigPath = "/etc/sysctl.d/99-embedded-cluster.conf"

func WriteRuntimeConfig(spec *v1beta1.RuntimeConfigSpec) error {
	if spec == nil {
		return nil
	}

	location := defaults.PathToECConfig()

	err := os.MkdirAll(filepath.Dir(location), 0755)
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

	err = os.WriteFile(location, yml, 0644)
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

// ConfigureSysctl writes the sysctl config file for the embedded cluster and
// reloads the sysctl configuration. This function has a distinct behavior: if
// the sysctl binary does not exist it returns an error but if it fails to lay
// down the sysctl config on disk it simply returns nil.
func ConfigureSysctl(provider *defaults.Provider) error {
	if _, err := exec.LookPath("sysctl"); err != nil {
		return fmt.Errorf("unable to find sysctl binary: %w", err)
	}

	materializer := goods.NewMaterializer(provider)
	if err := materializer.SysctlConfig(sysctlConfigPath); err != nil {
		logrus.Debugf("unable to materialize sysctl config: %v", err)
		return nil
	}

	if _, err := helpers.RunCommand("sysctl", "--system"); err != nil {
		logrus.Debugf("unable to configure sysctl: %v", err)
	}
	return nil
}
