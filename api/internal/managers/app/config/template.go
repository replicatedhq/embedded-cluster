package config

import (
	"bytes"
	"fmt"
)

func (m *appConfigManager) processTemplate(configYAML string) (string, error) {
	if configYAML == "" {
		return configYAML, nil
	}

	tmpl, err := m.templateEngine.Parse(configYAML)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, nil) // No custom context - vanilla Go templates only
	if err != nil {
		return "", fmt.Errorf("template execution error: %w", err)
	}

	return buf.String(), nil
}