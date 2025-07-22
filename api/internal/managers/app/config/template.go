package config

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (m *appConfigManager) executeConfigTemplate(configValues types.AppConfigValues) (string, error) {
	result, err := m.templateEngine.Execute(configValues)
	if err != nil {
		return "", fmt.Errorf("template execution error: %w", err)
	}

	return result, nil
}
