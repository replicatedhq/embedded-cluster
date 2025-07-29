package preflights

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"

	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights/types"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/loader"
)

//go:embed host-preflight.yaml
var clusterHostPreflightYAML string

func GetClusterHostPreflights(ctx context.Context, data types.HostPreflightTemplateData) ([]v1beta2.HostPreflight, error) {
	spec, err := renderHostPreflightTemplate(clusterHostPreflightYAML, data)
	if err != nil {
		return nil, fmt.Errorf("render host preflight template: %w", err)
	}
	kinds, err := loader.LoadSpecs(ctx, loader.LoadOptions{
		RawSpecs: []string{
			spec,
		},
		Strict: true,
	})
	if err != nil {
		return nil, fmt.Errorf("load host preflight specs: %w", err)
	}
	return kinds.HostPreflightsV1Beta2, nil
}

func renderHostPreflightTemplate(spec string, data types.HostPreflightTemplateData) (string, error) {
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
