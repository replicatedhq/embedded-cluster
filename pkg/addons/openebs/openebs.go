// Package openebs manages the OpenEBS storage provisioner helm chart
// installation or upgrade in the cluster.
package openebs

import (
	"context"
	"fmt"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

const (
	releaseName = "openebs"
	namespace   = "openebs"
)

// Overwritten by -ldflags in Makefile
var (
	ChartURL     = "https://url"
	ChartName    = "name"
	Version      = "v0.0.0"
	UtilsVersion = ""
)

var helmValues = map[string]interface{}{
	"ndmOperator": map[string]interface{}{
		"enabled": false,
	},
	"ndm": map[string]interface{}{
		"enabled": false,
	},
	"localprovisioner": map[string]interface{}{
		"deviceClass": map[string]interface{}{
			"enabled": false,
		},
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

func (a *OpenEBS) Name() string {
	return "OpenEBS"
}

// HostPreflights returns the host preflight objects found inside the OpenEBS
// Helm Chart, this is empty as there is no host preflight on there.
func (o *OpenEBS) HostPreflights() (*v1beta2.HostPreflightSpec, error) {
	return nil, nil
}

// GetProtectedFields returns the protected fields for the embedded charts.
// placeholder for now.
func (o *OpenEBS) GetProtectedFields() map[string][]string {
	protectedFields := []string{}
	return map[string][]string{releaseName: protectedFields}
}

// GenerateHelmConfig generates the helm config for the OpenEBS chart.
func (o *OpenEBS) GenerateHelmConfig(onlyDefaults bool) ([]v1beta1.Chart, []v1beta1.Repository, error) {
	helmValues["helper"] = map[string]interface{}{
		"imageTag": UtilsVersion,
	}

	chartConfig := v1beta1.Chart{
		Name:      releaseName,
		ChartName: ChartName,
		Version:   Version,
		TargetNS:  namespace,
		Order:     1,
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

func (o *OpenEBS) GetAdditionalImages() []string {
	return []string{fmt.Sprintf("openebs/linux-utils:%s", UtilsVersion)}
}

// Outro is executed after the cluster deployment.
func (o *OpenEBS) Outro(ctx context.Context, cli client.Client) error {
	loading := spinner.Start()
	loading.Infof("Waiting for Storage to be ready")
	if err := kubeutils.WaitForDeployment(ctx, cli, namespace, "openebs-localpv-provisioner"); err != nil {
		loading.Close()
		return err
	}
	loading.Closef("Storage is ready!")
	return nil
}

// New creates a new OpenEBS addon.
func New() (*OpenEBS, error) {
	return &OpenEBS{}, nil
}
