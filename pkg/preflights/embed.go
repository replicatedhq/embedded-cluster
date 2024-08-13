package preflights

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/loader"
)

var (
	//go:embed host-preflight.yaml
	clusterHostPreflightYAML string
)

func GetClusterHostPreflights(ctx context.Context, data TemplateData) ([]v1beta2.HostPreflight, error) {
	spec, err := renderTemplate(clusterHostPreflightYAML, data)
	if err != nil {
		return nil, fmt.Errorf("render host preflight template: %w", err)
	}
	kinds, err := loader.LoadSpecs(ctx, loader.LoadOptions{
		RawSpecs: []string{
			spec,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("load host preflight specs: %w", err)
	}
	return kinds.HostPreflightsV1Beta2, nil
}
