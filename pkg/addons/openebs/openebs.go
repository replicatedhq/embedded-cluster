// Package openebs manages the OpenEBS storage provisioner helm chart
// installation or upgrade in the cluster.
package openebs

import (
	"context"
	"fmt"

	helmv1beta1 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
func (o *OpenEBS) GenerateHelmConfig() ([]helmv1beta1.Chart, []v1beta1.Repository, error) {

	chartConfig := helmv1beta1.Chart{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Chart",
			APIVersion: "helm.k0sproject.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      releaseName,
			Namespace: "kube-system",
		},
		Spec: helmv1beta1.ChartSpec{
			ReleaseName: releaseName,
			ChartName:   ChartName,
			Version:     Version,
			Namespace:   "openebs",
			Order:       1,
		},
		Status: helmv1beta1.ChartStatus{},
	}

	repositoryConfig := v1beta1.Repository{
		Name: "openebs",
		URL:  ChartURL,
	}

	valuesStringData, err := yaml.Marshal(helmValues)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal helm values: %w", err)
	}
	chartConfig.Spec.Values = string(valuesStringData)

	return []helmv1beta1.Chart{chartConfig}, []v1beta1.Repository{repositoryConfig}, nil
}

// Outro is executed after the cluster deployment.
func (o *OpenEBS) Outro(_ context.Context, _ client.Client) error {
	return nil
}

// New creates a new OpenEBS addon.
func New() (*OpenEBS, error) {
	return &OpenEBS{}, nil
}
