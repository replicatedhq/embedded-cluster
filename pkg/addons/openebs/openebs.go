// Package openebs manages the OpenEBS storage provisioner helm chart
// installation or upgrade in the cluster.
package openebs

import (
	"context"
	_ "embed"
	"fmt"

	eckinds "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"gopkg.in/yaml.v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

const (
	releaseName = "openebs"
	namespace   = "openebs"
)

var (
	//go:embed static/values.yaml
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

	helmValues = make(map[string]interface{})
	if err := yaml.Unmarshal(rawvalues, &helmValues); err != nil {
		panic(fmt.Sprintf("unable to unmarshal metadata: %v", err))
	}
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
func (o *OpenEBS) GenerateHelmConfig(onlyDefaults bool) ([]eckinds.Chart, []eckinds.Repository, error) {
	chartConfig := eckinds.Chart{
		Name:      releaseName,
		ChartName: Metadata.Location,
		Version:   Metadata.Version,
		TargetNS:  namespace,
		Order:     1,
	}

	valuesStringData, err := yaml.Marshal(helmValues)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal helm values: %w", err)
	}
	chartConfig.Values = string(valuesStringData)

	return []eckinds.Chart{chartConfig}, nil, nil
}

func (a *OpenEBS) GetImages() []string {
	var images []string
	for component, tag := range Metadata.Images {
		images = append(images, fmt.Sprintf("%s:%s", helpers.AddonImageFromComponentName(component), tag))
	}
	return images
}

func (o *OpenEBS) GetAdditionalImages() []string {
	var images []string
	if tag, ok := Metadata.Images["openebs-linux-utils"]; ok {
		images = append(images, fmt.Sprintf("%s:%s", helpers.AddonImageFromComponentName("openebs-linux-utils"), tag))
	}
	return images
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
