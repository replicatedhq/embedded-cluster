// Package openebs manages the OpenEBS storage provisioner helm chart
// installation or upgrade in the cluster.
package openebs

import (
	"fmt"

	"github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v2"
)

const (
	releaseName = "openebs"
)

// Overwritten by -ldflags in Makefile
var (
	ChartURL  = "https://url"
	ChartName = "name"
	Version   = "v0.0.0"
)

var helmValues = map[string]interface{}{
	"ndmOperator": map[string]interface{}{
		"enabled": false,
	},
	"ndm": map[string]interface{}{
		"enabled": false,
	},
	"localprovisioner": map[string]interface{}{
		"hostpathClass": map[string]interface{}{
			"isDefaultClass": true,
		},
	},
}

// OpenEBS manages the installation of the OpenEBS helm chart.
type OpenEBS struct{}

// Version returns the version of the OpenEBS chart.
func (o *OpenEBS) Version() (map[string]string, error) {
	return map[string]string{"OpenEBS": "v" + Version}, nil
}

// HostPreflights returns the host preflight objects found inside the OpenEBS
// Helm Chart, this is empty as there is no host preflight on there.
func (o *OpenEBS) HostPreflights() (*v1beta2.HostPreflightSpec, error) {
	return nil, nil
}

// GenerateHelmConfig generates the helm config for the OpenEBS chart.
func (o *OpenEBS) GenerateHelmConfig() ([]v1beta1.Chart, []v1beta1.Repository, error) {
	chartConfig := v1beta1.Chart{
		Name:      releaseName,
		ChartName: ChartName,
		Version:   Version,
		TargetNS:  "openebs",
	}

	repositoryConfig := v1beta1.Repository{
		Name: "openebs",
		URL:  ChartURL,
	}

	valuesStringData, err := yaml.Marshal(helmValues)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal helm values: %w", err)
	}
	chartConfig.Values = string(valuesStringData)

	return []v1beta1.Chart{chartConfig}, []v1beta1.Repository{repositoryConfig}, nil
}

// New creates a new OpenEBS addon.
func New() (*OpenEBS, error) {
	return &OpenEBS{}, nil
}
