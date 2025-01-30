package embeddedclusteroperator

import (
	_ "embed"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"gopkg.in/yaml.v2"
	"k8s.io/utils/ptr"
)

type EmbeddedClusterOperator struct {
	BinaryNameOverride string
	IsAirgap           bool
	Proxy              *ecv1beta1.ProxySpec
}

const (
	releaseName = "embedded-cluster-operator"
	namespace   = "embedded-cluster"
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
		panic(errors.Wrap(err, "unmarshal metadata"))
	}
	Render()
}

func Render() {
	hv, err := release.RenderHelmValues(rawvalues, Metadata)
	if err != nil {
		panic(errors.Wrap(err, "unmarshal values"))
	}
	helmValues = hv

	helmValues["kotsVersion"] = adminconsole.Metadata.Version
	helmValues["embeddedClusterVersion"] = versions.Version
	helmValues["embeddedClusterK0sVersion"] = versions.K0sVersion
}

func (e *EmbeddedClusterOperator) Name() string {
	return "Embedded Cluster Operator"
}

func (e *EmbeddedClusterOperator) Version() map[string]string {
	return map[string]string{
		"EmbeddedClusterOperator": "v" + Metadata.Version,
	}
}

func (e *EmbeddedClusterOperator) ReleaseName() string {
	return releaseName
}

func (e *EmbeddedClusterOperator) Namespace() string {
	return namespace
}

func (e *EmbeddedClusterOperator) GetImages() []string {
	var images []string
	for _, image := range Metadata.Images {
		images = append(images, image.String())
	}
	return images
}

func (e *EmbeddedClusterOperator) GetAdditionalImages() []string {
	var images []string
	if image, ok := Metadata.Images["utils"]; ok {
		images = append(images, image.String())
	}
	return images
}

func (e *EmbeddedClusterOperator) GenerateChartConfig() ([]ecv1beta1.Chart, []k0sv1beta1.Repository, error) {
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
		Order:        3,
	}
	return []ecv1beta1.Chart{chartConfig}, nil, nil
}
