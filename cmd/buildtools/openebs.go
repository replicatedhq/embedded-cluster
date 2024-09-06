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
		Images:   make(map[string]release.AddonImage),
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
		component, ok := openebsImageComponents[RemoveTagFromImage(image)]
		if !ok {
			return fmt.Errorf("no component found for image %s", image)
		}
		repo, tag, err := component.resolveImageRepoAndTag(ctx, image, false)
		if err != nil {
			return fmt.Errorf("failed to resolve image and tag for %s: %w", image, err)
		}
		newmeta.Images[component.name] = release.AddonImage{
			Repo: repo,
			Tag: map[string]string{
				"amd64": tag,
				// TODO (@salah): automate updating the arm64 tag
				"arm64": openebs.Metadata.Images[component.name].Tag["arm64"],
			},
		}
	}

	logrus.Infof("saving addon manifest")
	if err := newmeta.Save("openebs"); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	return nil
}
