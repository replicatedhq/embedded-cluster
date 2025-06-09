package embeddedclusteroperator

import (
	_ "embed"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"k8s.io/utils/ptr"
)

var (
	//go:embed static/metadata.yaml
	rawmetadata []byte
	// Metadata is the unmarshal version of rawmetadata.
	Metadata release.AddonMetadata
)

func Version() map[string]string {
	return map[string]string{
		"EmbeddedClusterOperator": "v" + Metadata.Version,
	}
}

func GetImages() []string {
	var images []string
	for _, image := range Metadata.Images {
		images = append(images, image.String())
	}
	return images
}

func GetAdditionalImages() []string {
	var images []string
	if image, ok := Metadata.Images["utils"]; ok {
		images = append(images, image.String())
	}
	return images
}

func GenerateChartConfig() ([]ecv1beta1.Chart, []k0sv1beta1.Repository, error) {
	values, err := helm.MarshalValues(helmValues)
	if err != nil {
		return nil, nil, errors.Wrap(err, "marshal helm values")
	}

	chartConfig := ecv1beta1.Chart{
		Name:         _releaseName,
		ChartName:    (&EmbeddedClusterOperator{}).ChartLocation(ecv1beta1.Domains{}),
		Version:      Metadata.Version,
		Values:       string(values),
		TargetNS:     _namespace,
		ForceUpgrade: ptr.To(false),
		Order:        3,
	}

	return []ecv1beta1.Chart{chartConfig}, nil, nil
}
