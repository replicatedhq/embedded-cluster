// Package openebs manages the OpenEBS storage provisioner helm chart
// installation or upgrade in the cluster.
package embeddedclusteroperator

import (
	"fmt"

	"github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/replicatedhq/helmvm/pkg/addons/adminconsole"
	"github.com/replicatedhq/helmvm/pkg/defaults"
	"github.com/replicatedhq/helmvm/pkg/metrics"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v2"
)

const (
	releaseName = "embedded-cluster-operator"
)

// Overwritten by -ldflags in Makefile
var (
	ChartURL  = "https://url"
	ChartName = "name"
	Version   = "v0.0.0"
)

var helmValues = map[string]interface{}{
	"kotsVersion":               adminconsole.Version,
	"embeddedClusterVersion":    defaults.Version,
	"embeddedClusterK0sVersion": defaults.K0sVersion,
	"embeddedBinaryName":        defaults.BinaryName(),
	"embeddedClusterID":         metrics.ClusterID().String(),
}

type EmbeddedClusterOperator struct{}

func (e *EmbeddedClusterOperator) Version() (map[string]string, error) {
	return map[string]string{"EmbeddedClusterOperator": "v" + Version}, nil
}

// HostPreflight returns the host preflight objects found inside the EmbeddedClusterOperator
// Helm Chart, this is empty as there is no host preflight on there.
func (e *EmbeddedClusterOperator) HostPreflights() (*v1beta2.HostPreflightSpec, error) {
	return nil, nil
}

func (e *EmbeddedClusterOperator) GenerateHelmConfig() ([]v1beta1.Chart, []v1beta1.Repository, error) {
	chartConfig := v1beta1.Chart{
		Name:      releaseName,
		ChartName: fmt.Sprintf("%s/%s", ChartURL, ChartName),
		Version:   Version,
		TargetNS:  "kube-system",
	}

	valuesStringData, err := yaml.Marshal(helmValues)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal helm values: %w", err)
	}
	chartConfig.Values = string(valuesStringData)

	return []v1beta1.Chart{chartConfig}, nil, nil
}

func New() (*EmbeddedClusterOperator, error) {
	return &EmbeddedClusterOperator{}, nil
}
