package velero

import (
	_ "embed"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"gopkg.in/yaml.v3"
	"k8s.io/utils/ptr"
)

var (
	//go:embed static/metadata.yaml
	rawmetadata []byte
	// Metadata is the unmarshal version of rawmetadata.
	Metadata release.AddonMetadata
)

func init() {
	if err := yaml.Unmarshal(rawmetadata, &Metadata); err != nil {
		panic(errors.Wrap(err, "unable to unmarshal metadata"))
	}
}

func Version() map[string]string {
	return map[string]string{"Velero": "v" + Metadata.Version}
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
	if image, ok := Metadata.Images["velero-restore-helper"]; ok {
		images = append(images, image.String())
	}
	if image, ok := Metadata.Images["kubectl"]; ok {
		images = append(images, image.String())
	}
	return images
}

func GenerateChartConfig() ([]ecv1beta1.Chart, []k0sv1beta1.Repository, error) {
	hv, err := helmValues()
	if err != nil {
		return nil, nil, errors.Wrap(err, "get helm values")
	}

	marshalled, err := helm.MarshalValues(hv)
	if err != nil {
		return nil, nil, errors.Wrap(err, "marshal helm values")
	}

	chartConfig := ecv1beta1.Chart{
		Name:         _releaseName,
		ChartName:    (&Velero{}).ChartLocation(ecv1beta1.Domains{}),
		Version:      Metadata.Version,
		Values:       string(marshalled),
		TargetNS:     _namespace,
		ForceUpgrade: ptr.To(false),
		Order:        3,
	}
	return []ecv1beta1.Chart{chartConfig}, nil, nil
}
