package config

import (
	"fmt"

	apitemplate "github.com/replicatedhq/embedded-cluster/api/pkg/template"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (m *appConfigManager) executeConfigTemplate(configValues types.AppConfigValues, config apitemplate.InstallationConfig) (string, error) {
	result, err := m.templateEngine.Execute(configValues, config)
	if err != nil {
		return "", fmt.Errorf("template execution error: %w", err)
	}

	return result, nil
}
