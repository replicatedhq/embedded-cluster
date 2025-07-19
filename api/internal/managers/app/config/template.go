package config

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/types"
	kyaml "sigs.k8s.io/yaml"
)

func (m *appConfigManager) initConfigTemplate() error {
	configYAML, err := kyaml.Marshal(m.rawConfig)
	if err != nil {
		return fmt.Errorf("marshal config to yaml: %w", err)
	}

	if err := m.templateEngine.Parse(string(configYAML)); err != nil {
		return fmt.Errorf("template parse error: %w", err)
	}

	return nil
}

func (m *appConfigManager) executeConfigTemplate(configValues types.AppConfigValues) (string, error) {
	result, err := m.templateEngine.Execute(configValues)
	if err != nil {
		return "", fmt.Errorf("template execution error: %w", err)
	}

	return result, nil
}
