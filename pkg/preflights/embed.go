package preflights

import (
	"context"
	_ "embed"

	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/loader"
)

//go:embed host-preflight.yaml
var clusterHostPreflightYAML string

func GetClusterHostPreflights(ctx context.Context) ([]v1beta2.HostPreflight, error) {
	kinds, err := loader.LoadSpecs(ctx, loader.LoadOptions{
		RawSpec: clusterHostPreflightYAML,
	})
	if err != nil {
		return nil, err
	}
	return kinds.HostPreflightsV1Beta2, nil
}
