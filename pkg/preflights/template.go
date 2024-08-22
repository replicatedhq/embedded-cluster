// Package preflights manages running host preflights on remote hosts.
package preflights

import (
	"bytes"
	"fmt"
	"text/template"
)

type TemplateData struct {
	IsAirgap         bool
	ReplicatedAPIURL string
	ProxyRegistryURL string
}

func renderTemplate(spec string, data TemplateData) (string, error) {
	tmpl, err := template.New("preflight").Parse(spec)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	buf := bytes.NewBuffer(nil)
	err = tmpl.Execute(buf, data)
	if err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}
