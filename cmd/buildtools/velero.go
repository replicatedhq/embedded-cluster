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
	"helm.sh/helm/v3/pkg/repo"
)

// From: https://github.com/vmware-tanzu/velero-plugin-for-aws/blob/26bf6253ff0d74f8e5ce6aeb3053f31b7a297a99/README.md#compatibility
var veleroPluginForAWSCompatibility = map[string]*semver.Constraints{
	"1.14": mustParseSemverConstraints(">=1.10,<1.11"),
	"1.13": mustParseSemverConstraints(">=1.9,<1.10"),
}

var veleroImageComponents = map[string]addonComponent{
	"docker.io/velero/velero": {
		name: "velero",
		getWolfiPackageName: func(opts addonComponentOptions) string {
			return "velero"
		},
		upstreamVersionInputOverride: "INPUT_VELERO_VERSION",
	},
	"docker.io/velero/velero-plugin-for-aws": {
		name: "velero-plugin-for-aws",
		getWolfiPackageName: func(opts addonComponentOptions) string {
			return "velero-plugin-for-aws"
		},
		upstreamVersionInputOverride: "INPUT_VELERO_AWS_PLUGIN_VERSION",
	},
	"docker.io/velero/velero-restore-helper": {
		name: "velero-restore-helper",
		getWolfiPackageName: func(opts addonComponentOptions) string {
			return "velero-restore-helper"
		},
		upstreamVersionInputOverride: "INPUT_VELERO_VERSION",
	},
	"docker.io/bitnami/kubectl": {
		name: "kubectl",
		getWolfiPackageName: func(opts addonComponentOptions) string {
			return "kubectl"
		},
		upstreamVersionInputOverride: "INPUT_KUBECTL_VERSION",
	},
}

var veleroRepo = &repo.Entry{
	Name: "vmware-tanzu",
	URL:  "https://vmware-tanzu.github.io/helm-charts",
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
			latest, err := LatestChartVersion(veleroRepo, "velero")
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
			if err := MirrorChart(veleroRepo, "velero", nextChartVersion); err != nil {
				return fmt.Errorf("failed to mirror velero chart: %v", err)
			}
		}

		upstream := fmt.Sprintf("%s/velero", os.Getenv("CHARTS_DESTINATION"))
		withproto := fmt.Sprintf("oci://proxy.replicated.com/anonymous/%s", upstream)

		veleroVersion, err := findVeleroVersionFromChart(c.Context, withproto, nextChartVersion)
		if err != nil {
			return fmt.Errorf("failed to find velero version from chart: %w", err)
		}
		restoreHelperVersion := veleroVersion
		logrus.Infof("found latest velero restore helper version %s", restoreHelperVersion)

		awsPluginVersion, err := findBestAWSPluginVersion(c.Context, veleroVersion)
		if err != nil {
			return fmt.Errorf("failed to find latest velero plugin for aws version: %w", err)
		}
		logrus.Infof("found best velero plugin for aws version %s", awsPluginVersion)

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
		restoreHelperVersion, _, _ := strings.Cut(image.Tag["amd64"], "@")
		restoreHelperVersion = strings.TrimSuffix(restoreHelperVersion, "-amd64")
		restoreHelperVersion = strings.TrimPrefix(restoreHelperVersion, "v")

		image, ok = velero.Metadata.Images["velero-plugin-for-aws"]
		if !ok {
			return fmt.Errorf("failed to find velero plugin for aws image")
		}
		awsPluginVersion, _, _ := strings.Cut(image.Tag["amd64"], "@")
		awsPluginVersion = strings.TrimSuffix(awsPluginVersion, "-amd64")
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
		return "", fmt.Errorf("failed to get images from velero chart: %w", err)
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

func findBestAWSPluginVersion(ctx context.Context, veleroVersion string) (string, error) {
	sv, err := semver.NewVersion(veleroVersion)
	if err != nil {
		return "", fmt.Errorf("failed to parse velero version: %w", err)
	}
	constraints, ok := veleroPluginForAWSCompatibility[fmt.Sprintf("%d.%d", sv.Major(), sv.Minor())]
	if !ok {
		return "", fmt.Errorf("no aws plugin compatibility constraints found for velero version %s", veleroVersion)
	}
	awsPluginVersion, err := GetGreatestGitHubTag(ctx, "vmware-tanzu", "velero-plugin-for-aws", constraints)
	if err != nil {
		return "", fmt.Errorf("failed to get best velero plugin for aws release with constraints %s: %w", constraints, err)
	}
	return strings.TrimPrefix(awsPluginVersion, "v"), nil
}

func updateVeleroAddonImages(ctx context.Context, chartURL string, chartVersion string, restoreHelperVersion string, awsPluginVersion string) error {
	newmeta := release.AddonMetadata{
		Version:  chartVersion,
		Location: chartURL,
		Images:   make(map[string]release.AddonImage),
	}

	values, err := release.GetValuesWithOriginalImages("velero")
	if err != nil {
		return fmt.Errorf("failed to get velero values: %v", err)
	}

	logrus.Infof("extracting images from chart version %s", chartVersion)
	images, err := GetImagesFromOCIChart(chartURL, "velero", chartVersion, values)
	if err != nil {
		return fmt.Errorf("failed to get images from velero chart: %w", err)
	}

	// make sure we include additional images
	images = append(images, fmt.Sprintf("docker.io/velero/velero-restore-helper:%s", restoreHelperVersion))
	images = append(images, fmt.Sprintf("docker.io/velero/velero-plugin-for-aws:%s", awsPluginVersion))

	metaImages, err := UpdateImages(ctx, veleroImageComponents, velero.Metadata.Images, images)
	if err != nil {
		return fmt.Errorf("failed to update images: %w", err)
	}
	newmeta.Images = metaImages

	logrus.Infof("saving addon manifest")
	if err := newmeta.Save("velero"); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	return nil
}

func mustParseSemverConstraints(s string) *semver.Constraints {
	c, err := semver.NewConstraint(s)
	if err != nil {
		panic(err)
	}
	return c
}
