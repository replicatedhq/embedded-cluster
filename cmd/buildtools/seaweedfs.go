package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"helm.sh/helm/v3/pkg/repo"
)

var seaweedfsRepo = &repo.Entry{
	Name: "seaweedfs",
	URL:  "https://seaweedfs.github.io/seaweedfs/helm",
}

var seaweedfsImageComponents = map[string]addonComponent{
	"docker.io/chrislusf/seaweedfs": {
		name: "seaweedfs",
		getWolfiPackageName: func(opts addonComponentOptions) string {
			return "seaweedfs"
		},
		upstreamVersionInputOverride: "INPUT_SEAWEEDFS_VERSION",
	},
}

var updateSeaweedFSAddonCommand = &cli.Command{
	Name:      "seaweedfs",
	Usage:     "Updates the SeaweedFS addon",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating seaweedfs addon")

		nextChartVersion := os.Getenv("INPUT_SEAWEEDFS_CHART_VERSION")
		if nextChartVersion != "" {
			logrus.Infof("using input override from INPUT_SEAWEEDFS_CHART_VERSION: %s", nextChartVersion)
		} else {
			logrus.Infof("fetching the latest seaweedfs chart version")
			latest, err := LatestChartVersion(seaweedfsRepo, "seaweedfs")
			if err != nil {
				return fmt.Errorf("failed to get the latest seaweedfs chart version: %v", err)
			}
			nextChartVersion = latest
			logrus.Printf("latest seaweedfs chart version: %s", latest)
		}
		nextChartVersion = strings.TrimPrefix(nextChartVersion, "v")

		current := seaweedfs.Metadata
		if current.Version == nextChartVersion && !c.Bool("force") {
			logrus.Infof("seaweedfs chart version is already up-to-date")
		} else {
			logrus.Infof("mirroring seaweedfs chart version %s", nextChartVersion)
			if err := MirrorChart(seaweedfsRepo, "seaweedfs", nextChartVersion); err != nil {
				return fmt.Errorf("failed to mirror seaweedfs chart: %v", err)
			}
		}

		upstream := fmt.Sprintf("%s/seaweedfs", os.Getenv("CHARTS_DESTINATION"))
		withproto := fmt.Sprintf("oci://proxy.replicated.com/anonymous/%s", upstream)

		logrus.Infof("updating seaweedfs images")

		err := updateSeaweedFSAddonImages(c.Context, withproto, nextChartVersion)
		if err != nil {
			return fmt.Errorf("failed to update seaweedfs images: %w", err)
		}

		logrus.Infof("successfully updated seaweedfs addon")

		return nil
	},
}

var updateSeaweedFSImagesCommand = &cli.Command{
	Name:      "seaweedfs",
	Usage:     "Updates the seaweedfs images",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating seaweedfs images")

		current := seaweedfs.Metadata

		err := updateSeaweedFSAddonImages(c.Context, current.Location, current.Version)
		if err != nil {
			return fmt.Errorf("failed to update seaweedfs images: %w", err)
		}

		logrus.Infof("successfully updated seaweedfs images")

		return nil
	},
}

func updateSeaweedFSAddonImages(ctx context.Context, chartURL string, chartVersion string) error {
	newmeta := release.AddonMetadata{
		Version:  chartVersion,
		Location: chartURL,
		Images:   make(map[string]release.AddonImage),
	}

	values, err := release.GetValuesWithOriginalImages("seaweedfs")
	if err != nil {
		return fmt.Errorf("failed to get seaweedfs values: %v", err)
	}

	logrus.Infof("extracting images from chart version %s", chartVersion)
	images, err := GetImagesFromOCIChart(chartURL, "seaweedfs", chartVersion, values)
	if err != nil {
		return fmt.Errorf("failed to get images from seaweedfs chart: %w", err)
	}

	if err := ApkoLogin(); err != nil {
		return fmt.Errorf("failed to apko login: %w", err)
	}

	for _, image := range images {
		component, ok := seaweedfsImageComponents[RemoveTagFromImage(image)]
		if !ok {
			return fmt.Errorf("no component found for image %s", image)
		}
		img, tag, err := component.resolveImageAndTag(ctx, image)
		if err != nil {
			return fmt.Errorf("failed to resolve image and tag for %s: %w", image, err)
		}
		newmeta.Images[component.name] = release.AddonImage{
			Image: img,
			Tag:   tag,
		}
	}

	logrus.Infof("saving addon manifest")
	newmeta.ReplaceImages = true
	if err := newmeta.Save("seaweedfs"); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	return nil
}
