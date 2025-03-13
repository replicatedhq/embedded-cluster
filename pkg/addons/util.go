package addons

import (
	"errors"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
)

func addOnOverrides(addon types.AddOn, embCfgSpec *ecv1beta1.ConfigSpec, euCfgSpec *ecv1beta1.ConfigSpec) []string {
	overrides := []string{}
	if embCfgSpec != nil {
		overrides = append(overrides, embCfgSpec.OverrideForBuiltIn(addon.ReleaseName()))
	}
	if euCfgSpec != nil {
		overrides = append(overrides, euCfgSpec.OverrideForBuiltIn(addon.ReleaseName()))
	}
	return overrides
}

func operatorChart(meta *ectypes.ReleaseMetadata) (string, string, error) {
	// search through for the operator chart, and find the location
	for _, chart := range meta.Configs.Charts {
		if chart.Name == "embedded-cluster-operator" {
			return chart.ChartName, chart.Version, nil
		}
	}
	return "", "", errors.New("no embedded-cluster-operator chart found in release metadata")
}

func operatorImages(images []string, proxyRegistryDomain string) (string, string, string, error) {
	// determine the images to use for the operator chart
	ecOperatorImage := ""
	ecUtilsImage := ""

	for _, image := range images {
		if strings.Contains(image, "/embedded-cluster-operator-image:") {
			ecOperatorImage = image
		}
		if strings.Contains(image, "/ec-utils:") {
			ecUtilsImage = image
		}
	}

	if ecOperatorImage == "" {
		return "", "", "", errors.New("no embedded-cluster-operator-image found in images")
	}
	if ecUtilsImage == "" {
		return "", "", "", errors.New("no ec-utils found in images")
	}

	// the override images for operator during upgrades also need to be updated to use a whitelabeled proxy registry
	if proxyRegistryDomain != "" {
		ecOperatorImage = strings.ReplaceAll(ecOperatorImage, "proxy.replicated.com", proxyRegistryDomain)
		ecUtilsImage = strings.ReplaceAll(ecUtilsImage, "proxy.replicated.com", proxyRegistryDomain)
	}

	repo := strings.Split(ecOperatorImage, ":")[0]
	tag := strings.Join(strings.Split(ecOperatorImage, ":")[1:], ":")

	return repo, tag, ecUtilsImage, nil
}
