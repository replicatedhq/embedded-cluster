package registry

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
	return map[string]string{"Registry": "v" + Metadata.Version}
}

func GetImages() []string {
	var images []string
	for _, image := range Metadata.Images {
		images = append(images, image.String())
	}
	return images
}

func GetAdditionalImages() []string {
	return nil
}

func GenerateChartConfig(isHA bool) ([]ecv1beta1.Chart, []k0sv1beta1.Repository, error) {
	var hv map[string]interface{}
	if isHA {
		v, err := helmValuesHA()
		if err != nil {
			return nil, nil, errors.Wrap(err, "get helm values")
		}
		hv = v
	} else {
		v, err := helmValues()
		if err != nil {
			return nil, nil, errors.Wrap(err, "get helm values")
		}
		hv = v
	}

	marshalled, err := helm.MarshalValues(hv)
	if err != nil {
		return nil, nil, errors.Wrap(err, "marshal helm values")
	}

	chartConfig := ecv1beta1.Chart{
		Name:         _releaseName,
		ChartName:    (&Registry{}).ChartLocation(ecv1beta1.Domains{}),
		Version:      Metadata.Version,
		Values:       string(marshalled),
		TargetNS:     _namespace,
		ForceUpgrade: ptr.To(false),
		Order:        3,
	}
	return []ecv1beta1.Chart{chartConfig}, nil, nil
}
