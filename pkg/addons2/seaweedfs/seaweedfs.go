package seaweedfs

import (
	_ "embed"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"gopkg.in/yaml.v2"
	"k8s.io/utils/ptr"
)

type SeaweedFS struct {
	ServiceCIDR string
}

const (
	releaseName = "seaweedfs"
	namespace   = runtimeconfig.SeaweedFSNamespace

	// s3SVCName is the name of the Seaweedfs S3 service managed by the operator.
	// HACK: This service has a hardcoded service IP shared by the cli and operator as it is used
	// by the registry to redirect requests for blobs.
	s3SVCName = "ec-seaweedfs-s3"

	// lowerBandIPIndex is the index of the seaweedfs service IP in the service CIDR.
	lowerBandIPIndex = 11

	// s3SecretName is the name of the Seaweedfs secret.
	// This secret name is defined in the chart in the release metadata.
	s3SecretName = "secret-seaweedfs-s3"
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

func (s *SeaweedFS) Name() string {
	return "SeaweedFS"
}

func (o *SeaweedFS) Version() map[string]string {
	return map[string]string{"SeaweedFS": "v" + Metadata.Version}
}

func (s *SeaweedFS) ReleaseName() string {
	return releaseName
}

func (s *SeaweedFS) Namespace() string {
	return namespace
}

func (s *SeaweedFS) GetImages() []string {
	var images []string
	for _, image := range Metadata.Images {
		images = append(images, image.String())
	}
	return images
}

func (s *SeaweedFS) GetAdditionalImages() []string {
	return nil
}

func (s *SeaweedFS) GenerateChartConfig() ([]ecv1beta1.Chart, []k0sv1beta1.Repository, error) {
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
		Order:        2,
	}
	return []ecv1beta1.Chart{chartConfig}, nil, nil
}
