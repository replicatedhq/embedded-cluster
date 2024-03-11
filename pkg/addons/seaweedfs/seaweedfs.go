package seaweedfs

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
	releaseName = "seaweedfs"
	namespace   = "seaweedfs"
)

// Overwritten by -ldflags in Makefile
var (
	ChartURL  = "https://url"
	ChartName = "name"
	Version   = "v0.0.0"
)

var helmValues = map[string]interface{}{
	// TODO
}

// Registry manages the installation of the Registry helm chart.
type Registry struct {
	isAirgap bool
}

// Version returns the version of the Registry chart.
func (o *Registry) Version() (map[string]string, error) {
	return map[string]string{"Registry": "v" + Version}, nil
}

func (a *Registry) Name() string {
	return "Registry"
}

// HostPreflights returns the host preflight objects found inside the Registry
// Helm Chart, this is empty as there is no host preflight on there.
func (o *Registry) HostPreflights() (*v1beta2.HostPreflightSpec, error) {
	return nil, nil
}

// GetProtectedFields returns the protected fields for the embedded charts.
// placeholder for now.
func (o *Registry) GetProtectedFields() map[string][]string {
	protectedFields := []string{}
	return map[string][]string{releaseName: protectedFields}
}

// GenerateHelmConfig generates the helm config for the Registry chart.
func (o *Registry) GenerateHelmConfig(onlyDefaults bool) ([]v1beta1.Chart, []v1beta1.Repository, error) {
	if !o.isAirgap {
		return nil, nil, nil
	}

	chartConfig := v1beta1.Chart{
		Name:      releaseName,
		ChartName: ChartName,
		Version:   Version,
		TargetNS:  namespace,
		Order:     1,
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

// Outro is executed after the cluster deployment.
func (o *Registry) Outro(ctx context.Context, cli client.Client) error {
	loading := spinner.Start()
	loading.Infof("Waiting for Registry to be ready")
	if err := kubeutils.WaitForDeployment(ctx, cli, namespace, "seaweedfs"); err != nil {
		loading.Close()
		return err
	}
	loading.Closef("Registry is ready!")
	return nil
}

// New creates a new Registry addon.
func New(isAirgap bool) (*Registry, error) {
	return &Registry{isAirgap: isAirgap}, nil
}
