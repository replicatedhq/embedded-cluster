package embeddedclusteroperator

import (
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"k8s.io/utils/ptr"
	"strings"
)

func Version() map[string]string {
	return map[string]string{
		"EmbeddedClusterOperator": "v" + Metadata.Version,
	}
}

func GetImages() []string {
	var images []string
	proxyRegistryDomain := runtimeconfig.ProxyRegistryDomain(true)
	for _, image := range Metadata.Images {
		images = append(images, strings.ReplaceAll(image.String(), "proxy.replicated.com", proxyRegistryDomain))
	}
	return images
}

func GetAdditionalImages() []string {
	var images []string
	proxyRegistryDomain := runtimeconfig.ProxyRegistryDomain(true)
	if image, ok := Metadata.Images["utils"]; ok {
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
		ChartName:    (&EmbeddedClusterOperator{}).ChartLocation(),
		Version:      Metadata.Version,
		Values:       string(values),
		TargetNS:     namespace,
		ForceUpgrade: ptr.To(false),
		Order:        3,
	}

	return []ecv1beta1.Chart{chartConfig}, nil, nil
}
