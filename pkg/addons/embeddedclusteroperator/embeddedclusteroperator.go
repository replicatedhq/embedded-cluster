// Package embeddedclusteroperator manages the installation of the embedded cluster
// operator chart.
package embeddedclusteroperator

import (
	"context"
	"fmt"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster-operator/api/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/embed"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	pb "github.com/replicatedhq/embedded-cluster/pkg/progressbar"
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

// EmbeddedClusterOperator manages the installation of the embedded cluster operator
// helm chart.
type EmbeddedClusterOperator struct {
	namespace     string
	deployName    string
	endUserConfig *embeddedclusterv1beta1.Config
}

// Version returns the version of the embedded cluster operator chart.
func (e *EmbeddedClusterOperator) Version() (map[string]string, error) {
	return map[string]string{"EmbeddedClusterOperator": "v" + Version}, nil
}

// HostPreflights returns the host preflight objects found inside the EmbeddedClusterOperator
// Helm Chart, this is empty as there is no host preflight on there.
func (e *EmbeddedClusterOperator) HostPreflights() (*v1beta2.HostPreflightSpec, error) {
	return nil, nil
}

// GetProtectedFields returns the protected fields for the embedded charts.
// placeholder for now.
func (e *EmbeddedClusterOperator) GetProtectedFields() map[string][]string {
	protectedFields := []string{
		"embeddedBinaryName",
	}
	return map[string][]string{releaseName: protectedFields}
}

// GenerateHelmConfig generates the helm config for the embedded cluster operator chart.
func (e *EmbeddedClusterOperator) GenerateHelmConfig(onlyDefaults bool) ([]v1beta1.Chart, []v1beta1.Repository, error) {
	chartConfig := v1beta1.Chart{
		Name:      releaseName,
		ChartName: fmt.Sprintf("%s/%s", ChartURL, ChartName),
		Version:   Version,
		TargetNS:  "embedded-cluster",
		Order:     2,
	}
	valuesStringData, err := yaml.Marshal(helmValues)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal helm values: %w", err)
	}
	chartConfig.Values = string(valuesStringData)
	return []v1beta1.Chart{chartConfig}, nil, nil
}

// Outro is executed after the cluster deployment.
func (e *EmbeddedClusterOperator) Outro(ctx context.Context, cli client.Client) error {
	loading := pb.Start()
	loading.Infof("Waiting for Embedded Cluster Operator to be ready")
	if err := kubeutils.WaitForDeployment(ctx, cli, e.namespace, e.deployName); err != nil {
		loading.Close()
		return err
	}
	loading.Closef("Embedded Cluster Operator is ready!")
	cfg, err := embed.GetEmbeddedClusterConfig()
	if err != nil {
		return err
	}
	var cfgspec *embeddedclusterv1beta1.ConfigSpec
	if cfg != nil {
		cfgspec = &cfg.Spec
	}
	var euOverrides string
	if e.endUserConfig != nil {
		euOverrides = e.endUserConfig.Spec.UnsupportedOverrides.K0s
	}
	installation := embeddedclusterv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{
			Name: time.Now().Format("20060102150405"),
		},
		Spec: embeddedclusterv1beta1.InstallationSpec{
			ClusterID:                 metrics.ClusterID().String(),
			MetricsBaseURL:            metrics.BaseURL(),
			AirGap:                    false,
			Config:                    cfgspec,
			EndUserK0sConfigOverrides: euOverrides,
		},
	}
	embeddedclusterv1beta1.AddToScheme(cli.Scheme())
	if err := cli.Create(ctx, &installation); err != nil {
		return fmt.Errorf("unable to create installation: %w", err)
	}
	return nil
}

// New creates a new EmbeddedClusterOperator addon.
func New(endUserConfig *embeddedclusterv1beta1.Config) (*EmbeddedClusterOperator, error) {
	return &EmbeddedClusterOperator{
		namespace:     "embedded-cluster",
		deployName:    "embedded-cluster-operator",
		endUserConfig: endUserConfig,
	}, nil
}
