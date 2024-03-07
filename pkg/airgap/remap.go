package airgap

import (
	"fmt"
	"path/filepath"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
)

// RemapHelm removes all helm repositories from the cluster config, and changes the upstreams of all helm charts
// to paths on the host within the charts directory
func RemapHelm(cfg *v1beta1.ClusterConfig) {
	// there's no upstream to reach, so we can zero out the repositories
	cfg.Spec.Extensions.Helm.Repositories = nil

	// replace each chart's name with the path on the host it should be found at
	// see https://docs.k0sproject.io/v1.29.2+k0s.0/helm-charts/#example
	for idx := range cfg.Spec.Extensions.Helm.Charts {
		cfg.Spec.Extensions.Helm.Charts[idx].ChartName = helmChartHostPath(cfg.Spec.Extensions.Helm.Charts[idx])
	}
}

func helmChartHostPath(chart v1beta1.Chart) string {
	return filepath.Join(defaults.EmbeddedClusterChartsSubDir(), fmt.Sprintf("%s-%s.tgz", chart.Name, chart.Version))
}
