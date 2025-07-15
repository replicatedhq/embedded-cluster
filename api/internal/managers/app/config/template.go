package config

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	kyaml "sigs.k8s.io/yaml"
)

func (m *appConfigManager) initConfigTemplate() error {
	configYAML, err := kyaml.Marshal(m.rawConfig)
	if err != nil {
		return fmt.Errorf("marshal config to yaml: %w", err)
	}

	tmpl := template.New("config").Funcs(sprig.TxtFuncMap())
	parsedTemplate, err := tmpl.Parse(string(configYAML))
	if err != nil {
		return fmt.Errorf("template parse error: %w", err)
	}

	m.configTemplate = parsedTemplate
	return nil
}

func (m *appConfigManager) executeConfigTemplate() (string, error) {
	var buf bytes.Buffer
	err := m.configTemplate.Execute(&buf, nil) // No custom context yet - vanilla Go templates only
	if err != nil {
		return "", fmt.Errorf("template execution error: %w", err)
	}

	return buf.String(), nil
}
