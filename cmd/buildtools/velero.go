package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/velero"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var veleroImageComponents = map[string]string{
	"docker.io/velero/velero":                "velero",
	"docker.io/velero/velero-plugin-for-aws": "velero-plugin-for-aws",
	"docker.io/velero/velero-restore-helper": "velero-restore-helper",
	"docker.io/bitnami/kubectl":              "kubectl",
}

var veleroComponents = map[string]addonComponent{
	"velero": {
		upstreamVersionInputOverride: "INPUT_VELERO_VERSION",
	},
	"velero-plugin-for-aws": {
		upstreamVersionInputOverride: "INPUT_VELERO_AWS_PLUGIN_VERSION",
	},
	"velero-restore-helper": {
		upstreamVersionInputOverride: "INPUT_VELERO_VERSION",
	},
	"kubectl": {
		getWolfiPackageName: func(k0sVersion *semver.Version, upstreamVersion string) string {
			return fmt.Sprintf("kubectl-%d.%d-default", k0sVersion.Major(), k0sVersion.Minor())
		},
		getWolfiPackageVersionComparison: func(k0sVersion *semver.Version, upstreamVersion string) string {
			// match the greatest patch version of the same minor version
			return fmt.Sprintf(">=%d.%d, <%d.%d", k0sVersion.Major(), k0sVersion.Minor(), k0sVersion.Major(), k0sVersion.Minor()+1)
		},
		upstreamVersionInputOverride: "INPUT_KUBECTL_VERSION",
	},
}

var updateVeleroAddonCommand = &cli.Command{
	Name:      "velero",
	Usage:     "Updates the Velero addon",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating velero addon")

		nextChartVersion := os.Getenv("INPUT_VELERO_CHART_VERSION")
		if nextChartVersion != "" {
			logrus.Infof("using input override from INPUT_VELERO_CHART_VERSION: %s", nextChartVersion)
		} else {
			logrus.Infof("fetching the latest velero chart version")
			latest, err := LatestChartVersion("vmware-tanzu", "velero")
			if err != nil {
				return fmt.Errorf("failed to get the latest velero chart version: %v", err)
			}
			nextChartVersion = latest
			logrus.Printf("latest velero chart version: %s", latest)
		}
		nextChartVersion = strings.TrimPrefix(nextChartVersion, "v")

		current := velero.Metadata
		if current.Version == nextChartVersion && !c.Bool("force") {
			logrus.Infof("velero chart version is already up-to-date")
		} else {
			logrus.Infof("mirroring velero chart version %s", nextChartVersion)
			if err := MirrorChart("vmware-tanzu", "velero", nextChartVersion); err != nil {
				return fmt.Errorf("failed to mirror velero chart: %v", err)
			}
		}

		upstream := fmt.Sprintf("%s/velero", os.Getenv("CHARTS_DESTINATION"))
		withproto := fmt.Sprintf("oci://proxy.replicated.com/anonymous/%s", upstream)

		restoreHelperVersion, err := findVeleroVersionFromChart(c.Context, withproto, nextChartVersion)
		if err != nil {
			return fmt.Errorf("failed to find restore helper version: %w", err)
		}
		logrus.Infof("found latest velero restore helper version %s", restoreHelperVersion)

		awsPluginVersion, err := findLatestAWSPluginVersion(c.Context)
		if err != nil {
			return fmt.Errorf("failed to find latest velero plugin for aws version: %w", err)
		}
		logrus.Infof("found latest velero plugin for aws version %s", awsPluginVersion)

		logrus.Infof("updating velero images")

		err = updateVeleroAddonImages(c.Context, withproto, nextChartVersion, restoreHelperVersion, awsPluginVersion)
		if err != nil {
			return fmt.Errorf("failed to update velero images: %w", err)
		}

		logrus.Infof("successfully updated velero addon")

		return nil
	},
}

var updateVeleroImagesCommand = &cli.Command{
	Name:      "velero",
	Usage:     "Updates the velero images",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating velero images")

		current := velero.Metadata

		image, ok := velero.Metadata.Images["velero-restore-helper"]
		if !ok {
			return fmt.Errorf("failed to find velero restore helper image")
		}
		restoreHelperVersion, _, _ := strings.Cut(image, "@")
		restoreHelperVersion = strings.TrimPrefix(restoreHelperVersion, "v")

		image, ok = velero.Metadata.Images["velero-plugin-for-aws"]
		if !ok {
			return fmt.Errorf("failed to find velero plugin for aws image")
		}
		awsPluginVersion, _, _ := strings.Cut(image, "@")
		awsPluginVersion = strings.TrimPrefix(awsPluginVersion, "v")

		err := updateVeleroAddonImages(c.Context, current.Location, current.Version, restoreHelperVersion, awsPluginVersion)
		if err != nil {
			return fmt.Errorf("failed to update velero images: %w", err)
		}

		logrus.Infof("successfully updated velero images")

		return nil
	},
}

func findVeleroVersionFromChart(ctx context.Context, chartURL string, chartVersion string) (string, error) {
	values, err := release.GetValuesWithOriginalImages("velero")
	if err != nil {
		return "", fmt.Errorf("failed to get velero values: %v", err)
	}
	images, err := GetImagesFromOCIChart(chartURL, "velero", chartVersion, values)
	if err != nil {
		return "", fmt.Errorf("failed to get images from admin console chart: %w", err)
	}

	for _, image := range images {
		tag := TagFromImage(image)
		image = RemoveTagFromImage(image)
		if image == "docker.io/velero/velero" {
			return strings.TrimPrefix(tag, "v"), nil
		}
	}

	return "", fmt.Errorf("failed to find velero image tag")
}

func findLatestAWSPluginVersion(ctx context.Context) (string, error) {
	awsPluginVersion, err := GetLatestGitHubTag(ctx, "vmware-tanzu", "velero-plugin-for-aws")
	if err != nil {
		return "", fmt.Errorf("failed to get latest velero plugin for aws release: %w", err)
	}
	return strings.TrimPrefix(awsPluginVersion, "v"), nil
}

func updateVeleroAddonImages(ctx context.Context, chartURL string, chartVersion string, restoreHelperVersion string, awsPluginVersion string) error {
	newmeta := release.AddonMetadata{
		Version:  chartVersion,
		Location: chartURL,
		Images:   make(map[string]string),
	}

	rawver := os.Getenv("INPUT_K0S_VERSION")
	if rawver == "" {
		v, err := GetMakefileVariable("K0S_VERSION")
		if err != nil {
			return fmt.Errorf("failed to get k0s version: %w", err)
		}
		rawver = v
	}
	k0sVersion := semver.MustParse(rawver)

	logrus.Infof("fetching wolfi apk index")
	wolfiAPKIndex, err := GetWolfiAPKIndex()
	if err != nil {
		return fmt.Errorf("failed to get APK index: %w", err)
	}

	values, err := release.GetValuesWithOriginalImages("velero")
	if err != nil {
		return fmt.Errorf("failed to get velero values: %v", err)
	}

	logrus.Infof("extracting images from chart")
	images, err := GetImagesFromOCIChart(chartURL, "velero", chartVersion, values)
	if err != nil {
		return fmt.Errorf("failed to get images from admin console chart: %w", err)
	}

	// make sure we include additional images
	images = append(images, fmt.Sprintf("docker.io/velero/velero-restore-helper:%s", restoreHelperVersion))
	images = append(images, fmt.Sprintf("docker.io/velero/velero-plugin-for-aws:%s", awsPluginVersion))

	if err := ApkoLogin(); err != nil {
		return fmt.Errorf("failed to apko login: %w", err)
	}

	for _, image := range images {
		logrus.Infof("updating image %s", image)

		upstreamVersion := strings.TrimPrefix(TagFromImage(image), "v")
		image = RemoveTagFromImage(image)

		componentName, ok := veleroImageComponents[image]
		if !ok {
			return fmt.Errorf("no component found for image %s", image)
		}

		component, ok := veleroComponents[componentName]
		if !ok {
			return fmt.Errorf("no component found for component name %s", componentName)
		}

		if component.upstreamVersionInputOverride != "" {
			v := os.Getenv(component.upstreamVersionInputOverride)
			if v != "" {
				logrus.Infof("using input override from %s: %s", component.upstreamVersionInputOverride, v)
				upstreamVersion = v
			}
		}

		packageName, packageVersion, err := component.getPackageNameAndVersion(wolfiAPKIndex, k0sVersion, upstreamVersion)
		if err != nil {
			return fmt.Errorf("failed to get package name and version for %s: %w", componentName, err)
		}

		logrus.Infof("building and publishing %s, %s=%s", componentName, packageName, packageVersion)

		if err := ApkoBuildAndPublish(componentName, packageName, packageVersion); err != nil {
			return fmt.Errorf("failed to apko build and publish for %s: %w", componentName, err)
		}

		digest, err := GetDigestFromBuildFile()
		if err != nil {
			return fmt.Errorf("failed to get digest from build file: %w", err)
		}

		newmeta.Images[componentName] = fmt.Sprintf("%s@%s", packageVersion, digest)
	}

	logrus.Infof("saving addon manifest")
	newmeta.ReplaceImages = true
	if err := newmeta.Save("velero"); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	return nil
}
