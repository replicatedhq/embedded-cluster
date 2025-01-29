package openebs

import (
	_ "embed"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"gopkg.in/yaml.v2"
	"k8s.io/utils/ptr"
)

type OpenEBS struct{}

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
		panic(errors.Wrap(err, "unable to unmarshal metadata"))
	}
	hv, err := release.RenderHelmValues(rawvalues, Metadata)
	if err != nil {
		panic(errors.Wrap(err, "unable to unmarshal values"))
	}
	helmValues = hv
}

func (o *OpenEBS) Name() string {
	return "Storage"
}

func (o *OpenEBS) Version() map[string]string {
	return map[string]string{"OpenEBS": "v" + Metadata.Version}
}

func (o *OpenEBS) ReleaseName() string {
	return releaseName
}

func (o *OpenEBS) Namespace() string {
	return namespace
}

func (o *OpenEBS) GetImages() []string {
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

func (o *OpenEBS) GenerateChartConfig() ([]ecv1beta1.Chart, []k0sv1beta1.Repository, error) {
	values, err := helm.MarshalValues(helmValues)
	if err != nil {
		return nil, nil, errors.Wrap(err, "marshal helm values")
	}

	chartConfig := ecv1beta1.Chart{
		Name:         releaseName,
		ChartName:    Metadata.Location,
		Version:      Metadata.Version,
		Values:       string(values),
		TargetNS:     namespace,
		ForceUpgrade: ptr.To(false),
		Order:        1,
	}
	return []ecv1beta1.Chart{chartConfig}, nil, nil
}
