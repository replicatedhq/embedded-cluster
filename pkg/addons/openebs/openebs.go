// Package openebs manages the OpenEBS storage provisioner helm chart
// installation or upgrade in the cluster.
package openebs

import (
	"context"
	_ "embed"
	"fmt"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

const (
	releaseName = "openebs"
	namespace   = "openebs"
)

var (
	//go:embed static/values.tpl.yaml
	rawvalues []byte
	// helmValues is the unmarshal version of rawvalues.
	helmValues map[string]interface{}
	//go:embed static/metadata.yaml
	rawmetadata []byte
	// Metadata is the unmarshal version of rawmetadata.
	Metadata release.AddonMetadata
)

func init() {
	if err := yaml.Unmarshal(rawmetadata, &Metadata); err != nil {
		panic(fmt.Sprintf("unable to unmarshal metadata: %v", err))
	}

	hv, err := release.RenderHelmValues(rawvalues, Metadata)
	if err != nil {
		panic(fmt.Sprintf("unable to unmarshal values: %v", err))
	}
	helmValues = hv
}

// OpenEBS manages the installation of the OpenEBS helm chart.
type OpenEBS struct{}

// Version returns the version of the OpenEBS chart.
func (o *OpenEBS) Version() (map[string]string, error) {
	return map[string]string{"OpenEBS": "v" + Metadata.Version}, nil
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
func (o *OpenEBS) GenerateHelmConfig(k0sCfg *k0sv1beta1.ClusterConfig, onlyDefaults bool) ([]ecv1beta1.Chart, []ecv1beta1.Repository, error) {
	chartConfig := ecv1beta1.Chart{
		Name:         releaseName,
		ChartName:    Metadata.Location,
		Version:      Metadata.Version,
		TargetNS:     namespace,
		ForceUpgrade: ptr.To(false),
		Order:        1,
	}

	valuesStringData, err := yaml.Marshal(helmValues)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal helm values: %w", err)
	}
	chartConfig.Values = string(valuesStringData)

	return []ecv1beta1.Chart{chartConfig}, nil, nil
}

func (a *OpenEBS) GetImages() []string {
	var images []string
	for _, image := range Metadata.Images {
		images = append(images, image.String())
	}
	return images
}

func (o *OpenEBS) GetAdditionalImages() []string {
	var images []string
	if image, ok := Metadata.Images["openebs-linux-utils"]; ok {
		images = append(images, image.String())
	}
	return images
}

// Outro is executed after the cluster deployment.
func (o *OpenEBS) Outro(ctx context.Context, cli client.Client, k0sCfg *k0sv1beta1.ClusterConfig, releaseMetadata *types.ReleaseMetadata) error {
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
