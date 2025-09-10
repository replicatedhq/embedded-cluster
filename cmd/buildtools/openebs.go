package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"helm.sh/helm/v3/pkg/repo"
)

var openebsRepo = &repo.Entry{
	Name: "openebs",
	URL:  "https://openebs.github.io/openebs",
}

var openebsImageComponents = map[string]addonComponent{
	"docker.io/openebs/provisioner-localpv": {
		name: "openebs-provisioner-localpv",
		getWolfiPackageName: func(opts addonComponentOptions) string {
			// package name is not the same as the component name
			return "dynamic-localpv-provisioner"
		},
		upstreamVersionInputOverride: "INPUT_OPENEBS_VERSION",
	},
	"docker.io/openebs/linux-utils": {
		name:                         "openebs-linux-utils",
		upstreamVersionInputOverride: "INPUT_OPENEBS_VERSION",
	},
	"docker.io/bitnami/kubectl": {
		name: "kubectl",
		getWolfiPackageName: func(opts addonComponentOptions) string {
			return "kubectl"
		},
		upstreamVersionInputOverride: "INPUT_KUBECTL_VERSION",
	},
	"docker.io/bitnamilegacy/kubectl": {
		name: "kubectl",
		getWolfiPackageName: func(opts addonComponentOptions) string {
			return "kubectl"
		},
		upstreamVersionInputOverride: "INPUT_KUBECTL_VERSION",
	},
	"docker.io/openebs/kubectl": {
		name: "kubectl",
		getWolfiPackageName: func(opts addonComponentOptions) string {
			return "kubectl"
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

		hcli, err := NewHelm()
		if err != nil {
			return fmt.Errorf("failed to create helm client: %w", err)
		}
		defer hcli.Close()

		nextChartVersion := os.Getenv("INPUT_OPENEBS_CHART_VERSION")
		if nextChartVersion != "" {
			logrus.Infof("using input override from INPUT_OPENEBS_CHART_VERSION: %s", nextChartVersion)
		} else {
			logrus.Infof("fetching the latest openebs chart version")
			latest, err := LatestChartVersion(hcli, openebsRepo, "openebs")
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
			return nil
		}

		logrus.Infof("mirroring openebs chart version %s", nextChartVersion)
		if err := MirrorChart(hcli, openebsRepo, "openebs", nextChartVersion); err != nil {
			return fmt.Errorf("failed to mirror openebs chart: %v", err)
		}

		upstream := fmt.Sprintf("%s/openebs", os.Getenv("CHARTS_DESTINATION"))
		withproto := fmt.Sprintf("oci://proxy.replicated.com/anonymous/%s", upstream)

		logrus.Infof("updating openebs images")

		err = updateOpenEBSAddonImages(c.Context, hcli, withproto, nextChartVersion, nextChartVersion, nil)
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

		hcli, err := NewHelm()
		if err != nil {
			return fmt.Errorf("failed to create helm client: %w", err)
		}
		defer hcli.Close()

		current := openebs.Metadata

		err = updateOpenEBSAddonImages(c.Context, hcli, current.Location, current.Version, current.Version, c.StringSlice("image"))
		if err != nil {
			return fmt.Errorf("failed to update openebs images: %w", err)
		}

		logrus.Infof("successfully updated openebs images")

		return nil
	},
}

func updateOpenEBSAddonImages(ctx context.Context, hcli helm.Client, chartURL string, chartVersion string, linuxUtilsVersion string, filteredImages []string) error {
	newmeta := release.AddonMetadata{
		Version:  chartVersion,
		Location: chartURL,
		Images:   make(map[string]release.AddonImage),
	}

	values, err := release.GetValuesWithOriginalImages("openebs")
	if err != nil {
		return fmt.Errorf("failed to get openebs values: %v", err)
	}

	logrus.Infof("extracting images from chart version %s", chartVersion)
	images, err := helm.ExtractImagesFromChart(hcli, chartURL, chartVersion, values)
	if err != nil {
		return fmt.Errorf("failed to get images from openebs chart: %w", err)
	}

	// make sure we include the linux-utils image.
	images = append(images, fmt.Sprintf("docker.io/openebs/linux-utils:%s", linuxUtilsVersion))

	metaImages, err := UpdateImages(ctx, openebsImageComponents, openebs.Metadata.Images, images, filteredImages)
	if err != nil {
		return fmt.Errorf("failed to update images: %w", err)
	}
	newmeta.Images = metaImages

	logrus.Infof("saving addon manifest")
	if err := newmeta.Save("openebs"); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	return nil
}
