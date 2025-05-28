package release

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

// ValidateECConfig runs some basic checks on the embedded cluster config.
func ValidateECConfig() error {
	cfg := GetEmbeddedClusterConfig()
	if cfg == nil || cfg.Spec.Extensions.Helm == nil {
		return nil
	}

	// for each addon, check to see if the values file parses as yaml
	for _, addon := range cfg.Spec.Extensions.Helm.Charts {
		genericUnmarshal := map[string]interface{}{}
		err := yaml.Unmarshal([]byte(addon.Values), &genericUnmarshal)
		if err != nil {
			logrus.Debugf("failed to parse helm chart values for addon %s as yaml, values were %q: %v", addon.Name, addon.Values, err)
			return fmt.Errorf("failed to parse helm chart values for addon %s as yaml: %w", addon.Name, err)
		}
	}
	return nil
}
