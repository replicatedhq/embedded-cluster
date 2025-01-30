package registry

import (
	_ "embed"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"gopkg.in/yaml.v2"
	"k8s.io/utils/ptr"
)

type Registry struct {
	ServiceCIDR string
	IsHA        bool
}

const (
	releaseName      = "docker-registry"
	namespace        = runtimeconfig.RegistryNamespace
	tlsSecretName    = "registry-tls"
	lowerBandIPIndex = 10
)

var (
	//go:embed static/values.tpl.yaml
	rawvalues []byte
	// helmValues is the unmarshal version of rawvalues.
	helmValues map[string]interface{}
	//go:embed static/values-ha.tpl.yaml
	rawvaluesha []byte
	// helmValuesHA is the unmarshal version of rawvaluesha.
	helmValuesHA map[string]interface{}
	//go:embed static/metadata.yaml
	rawmetadata []byte
	// Metadata is the unmarshal version of rawmetadata.
	Metadata release.AddonMetadata

	registryPassword = helpers.RandString(20)
	registryAddress  = ""
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

	hvHA, err := release.RenderHelmValues(rawvaluesha, Metadata)
	if err != nil {
		panic(errors.Wrap(err, "unable to unmarshal ha values"))
	}
	helmValuesHA = hvHA
}

func (r *Registry) Name() string {
	return "Registry"
}

func (r *Registry) Version() map[string]string {
	return map[string]string{"Registry": "v" + Metadata.Version}
}

func (r *Registry) ReleaseName() string {
	return releaseName
}

func (r *Registry) Namespace() string {
	return namespace
}

func (r *Registry) GetImages() []string {
	var images []string
	for _, image := range Metadata.Images {
		images = append(images, image.String())
	}
	return images
}

func (r *Registry) GetAdditionalImages() []string {
	return nil
}

func (r *Registry) GenerateChartConfig() ([]ecv1beta1.Chart, []k0sv1beta1.Repository, error) {
	var v map[string]interface{}
	if r.IsHA {
		v = helmValuesHA
	} else {
		v = helmValues
	}

	values, err := helm.MarshalValues(v)
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
		Order:        3,
	}
	return []ecv1beta1.Chart{chartConfig}, nil, nil
}

func GetRegistryPassword() string {
	return registryPassword
}

func GetRegistryClusterIP() string {
	return registryAddress
}
