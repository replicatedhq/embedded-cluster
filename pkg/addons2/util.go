package addons2

import (
	"errors"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/types"
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

func operatorImages(images []string) (string, string, string, error) {
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

	repo := strings.Split(ecOperatorImage, ":")[0]
	tag := strings.Join(strings.Split(ecOperatorImage, ":")[1:], ":")

	return repo, tag, ecUtilsImage, nil
}

func cleanErrorMessage(err error) string {
	msg := err.Error()
	if len(msg) > 1024 {
		msg = msg[:1024]
	}
	return msg
}
