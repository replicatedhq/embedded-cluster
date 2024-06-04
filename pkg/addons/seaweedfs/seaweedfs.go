package seaweedfs

import (
	"context"
	"fmt"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	releaseName = "seaweedfs"
)

// Overwritten by -ldflags in Makefile
var (
	ChartURL  = "https://url"
	ChartName = "name"
	Version   = "v0.0.0"
)

var (
	helmValues map[string]interface{}
)

// SeaweedFS manages the installation of the SeaweedFS helm chart.
type SeaweedFS struct {
	namespace string
	config    v1beta1.ClusterConfig
	isAirgap  bool
}

// Version returns the version of the SeaweedFS chart.
func (o *SeaweedFS) Version() (map[string]string, error) {
	return map[string]string{"SeaweedFS": "v" + Version}, nil
}

func (a *SeaweedFS) Name() string {
	return "SeaweedFS"
}

// HostPreflights returns the host preflight objects found inside the SeaweedFS
// Helm Chart, this is empty as there is no host preflight on there.
func (o *SeaweedFS) HostPreflights() (*v1beta2.HostPreflightSpec, error) {
	return nil, nil
}

// GetProtectedFields returns the protected fields for the embedded charts.
// placeholder for now.
func (o *SeaweedFS) GetProtectedFields() map[string][]string {
	protectedFields := []string{}
	return map[string][]string{releaseName: protectedFields}
}

// GenerateHelmConfig generates the helm config for the SeaweedFS chart.
func (o *SeaweedFS) GenerateHelmConfig(onlyDefaults bool) ([]v1beta1.Chart, []v1beta1.Repository, error) {
	if !o.isAirgap {
		return nil, nil, nil
	}

	chartConfig := v1beta1.Chart{
		Name:      releaseName,
		ChartName: ChartName,
		Version:   Version,
		TargetNS:  o.namespace,
		Order:     2,
	}

	repositoryConfig := v1beta1.Repository{
		Name: "seaweedfs",
		URL:  ChartURL,
	}

	valuesStringData, err := yaml.Marshal(helmValues)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal helm values: %w", err)
	}
	chartConfig.Values = string(valuesStringData)

	return []v1beta1.Chart{chartConfig}, []v1beta1.Repository{repositoryConfig}, nil
}

func (o *SeaweedFS) GetAdditionalImages() []string {
	return nil
}

// Outro is executed after the cluster deployment.
func (o *SeaweedFS) Outro(ctx context.Context, cli client.Client) error {
	// SeaweedFS is applied by the operator
	return nil
}

// New creates a new SeaweedFS addon.
func New(namespace string, config v1beta1.ClusterConfig, isAirgap bool) (*SeaweedFS, error) {
	return &SeaweedFS{namespace: namespace, config: config, isAirgap: isAirgap}, nil
}

func init() {
	helmValues = make(map[string]interface{})
	if err := yaml.Unmarshal(helmValuesYAML, &helmValues); err != nil {
		panic(fmt.Errorf("failed to unmarshal helm values: %w", err))
	}
}
