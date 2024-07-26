package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"helm.sh/helm/v3/pkg/repo"
)

var openebsImageComponents = map[string]string{
	"docker.io/bitnami/kubectl":             "kubectl",
	"docker.io/openebs/linux-utils":         "openebs-linux-utils",
	"docker.io/openebs/provisioner-localpv": "openebs-provisioner-localpv",
}

var openebsRepo = &repo.Entry{
	Name: "openebs",
	URL:  "https://openebs.github.io/openebs",
}

var openebsComponents = map[string]addonComponent{
	"openebs-provisioner-localpv": {
		getWolfiPackageName: func(opts commonOptions) string {
			// package name is not the same as the component name
			return "dynamic-localpv-provisioner"
		},
		upstreamVersionInputOverride: "INPUT_OPENEBS_VERSION",
	},
	"openebs-linux-utils": {
		upstreamVersionInputOverride: "INPUT_OPENEBS_VERSION",
	},
	"kubectl": {
		getWolfiPackageName: func(opts commonOptions) string {
			return fmt.Sprintf("kubectl-%d.%d-default", opts.latestK8sVersion.Major(), opts.latestK8sVersion.Minor())
		},
		getWolfiPackageVersionComparison: func(opts commonOptions) string {
			// use latest available patch in wolfi as latest upstream might not be available yet
			return latestPatchComparison(opts.latestK8sVersion)
		},
		upstreamVersionInputOverride: "INPUT_KUBECTL_VERSION",
	},
}

var updateOpenEBSAddonCommand = &cli.Command{
	Name:      "openebs",
	Usage:     "Updates the OpenEBS addon",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating openebs addon")

		nextChartVersion := os.Getenv("INPUT_OPENEBS_CHART_VERSION")
		if nextChartVersion != "" {
			logrus.Infof("using input override from INPUT_OPENEBS_CHART_VERSION: %s", nextChartVersion)
		} else {
			logrus.Infof("fetching the latest openebs chart version")
			latest, err := LatestChartVersion(openebsRepo, "openebs")
			if err != nil {
				return fmt.Errorf("failed to get the latest openebs chart version: %v", err)
			}
			nextChartVersion = latest
			logrus.Printf("latest openebs chart version: %s", latest)
		}
		nextChartVersion = strings.TrimPrefix(nextChartVersion, "v")

		current := openebs.Metadata
		if current.Version == nextChartVersion && !c.Bool("force") {
			logrus.Infof("openebs chart version is already up-to-date")
		} else {
			logrus.Infof("mirroring openebs chart version %s", nextChartVersion)
			if err := MirrorChart(openebsRepo, "openebs", nextChartVersion); err != nil {
				return fmt.Errorf("failed to mirror openebs chart: %v", err)
			}
		}

		upstream := fmt.Sprintf("%s/openebs", os.Getenv("CHARTS_DESTINATION"))
		withproto := fmt.Sprintf("oci://proxy.replicated.com/anonymous/%s", upstream)

		logrus.Infof("updating openebs images")

		err := updateOpenEBSAddonImages(c.Context, withproto, nextChartVersion, nextChartVersion)
		if err != nil {
			return fmt.Errorf("failed to update openebs images: %w", err)
		}

		logrus.Infof("successfully updated openebs addon")

		return nil
	},
}

var updateOpenEBSImagesCommand = &cli.Command{
	Name:      "openebs",
	Usage:     "Updates the openebs images",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating openebs images")

		current := openebs.Metadata

		err := updateOpenEBSAddonImages(c.Context, current.Location, current.Version, current.Version)
		if err != nil {
			return fmt.Errorf("failed to update openebs images: %w", err)
		}

		logrus.Infof("successfully updated openebs images")

		return nil
	},
}

func updateOpenEBSAddonImages(ctx context.Context, chartURL string, chartVersion string, linuxUtilsVersion string) error {
	newmeta := release.AddonMetadata{
		Version:  chartVersion,
		Location: chartURL,
		Images:   make(map[string]string),
	}

	logrus.Infof("fetching wolfi apk index")
	wolfiAPKIndex, err := GetWolfiAPKIndex()
	if err != nil {
		return fmt.Errorf("failed to get APK index: %w", err)
	}

	values, err := release.GetValuesWithOriginalImages("openebs")
	if err != nil {
		return fmt.Errorf("failed to get openebs values: %v", err)
	}

	logrus.Infof("extracting images from chart version %s", chartVersion)
	images, err := GetImagesFromOCIChart(chartURL, "openebs", chartVersion, values)
	if err != nil {
		return fmt.Errorf("failed to get images from openebs chart: %w", err)
	}

	// make sure we include the linux-utils image.
	images = append(images, fmt.Sprintf("docker.io/openebs/linux-utils:%s", linuxUtilsVersion))

	if err := ApkoLogin(); err != nil {
		return fmt.Errorf("failed to apko login: %w", err)
	}

	for _, image := range images {
		logrus.Infof("updating image %s", image)

		upstreamVersion := TagFromImage(image)
		image = RemoveTagFromImage(image)

		componentName, ok := openebsImageComponents[image]
		if !ok {
			return fmt.Errorf("no component found for image %s", image)
		}

		component, ok := openebsComponents[componentName]
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

		packageName, packageVersion, err := component.getPackageNameAndVersion(wolfiAPKIndex, upstreamVersion)
		if err != nil {
			return fmt.Errorf("failed to get package name and version for %s: %w", componentName, err)
		}

		logrus.Infof("building and publishing %s, %s=%s", componentName, packageName, packageVersion)

		if err := ApkoBuildAndPublish(componentName, packageName, packageVersion, upstreamVersion); err != nil {
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
	if err := newmeta.Save("openebs"); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	return nil
}
