package openebs

import (
	"strings"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"k8s.io/utils/ptr"
)

func Version() map[string]string {
	return map[string]string{"OpenEBS": "v" + Metadata.Version}
}

func GetImages() []string {
	proxyRegistryDomain := runtimeconfig.ProxyRegistryDomain(true)
	var images []string
	for _, image := range Metadata.Images {
		images = append(images, strings.ReplaceAll(image.String(), "proxy.replicated.com", proxyRegistryDomain))
	}
	return images
}

func GetAdditionalImages() []string {
	proxyRegistryDomain := runtimeconfig.ProxyRegistryDomain(true)
	var images []string
	if image, ok := Metadata.Images["openebs-linux-utils"]; ok {
		images = append(images, strings.ReplaceAll(image.String(), "proxy.replicated.com", proxyRegistryDomain))
	}
	return images
}

func GenerateChartConfig() ([]ecv1beta1.Chart, []k0sv1beta1.Repository, error) {
	values, err := helm.MarshalValues(helmValues)
	if err != nil {
		return nil, nil, errors.Wrap(err, "marshal helm values")
	}

	chartConfig := ecv1beta1.Chart{
		Name:         releaseName,
		ChartName:    (&OpenEBS{}).ChartLocation(),
		Version:      Metadata.Version,
		Values:       string(values),
		TargetNS:     namespace,
		ForceUpgrade: ptr.To(false),
		Order:        1,
	}
	return []ecv1beta1.Chart{chartConfig}, nil, nil
}
