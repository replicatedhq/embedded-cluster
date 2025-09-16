package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"helm.sh/helm/v3/pkg/repo"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
)

var registryRepo = &repo.Entry{
	Name: "twuni",
	URL:  "https://helm.twun.io",
}

var registryImageComponents = map[string]addonComponent{
	"docker.io/library/registry": {
		name: "registry",
		getCustomImageName: func(opts addonComponentOptions) (string, error) {
			ref := "registry.replicated.com/library/registry"
			// TODO: unpin this
			return fmt.Sprintf("%s:%s", ref, "2.8.3"), nil
			// constraints := mustParseSemverConstraints(latestPatchConstraint(opts.upstreamVersion))
			// return getLatestImageNameAndTag(opts.ctx, ref, constraints)
		},
		upstreamVersionInputOverride: "INPUT_REGISTRY_VERSION",
	},
}

var updateRegistryAddonCommand = &cli.Command{
	Name:      "registry",
	Usage:     "Updates the Registry addon",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating registry addon")

		hcli, err := NewHelm()
		if err != nil {
			return fmt.Errorf("failed to create helm client: %w", err)
		}
		defer hcli.Close()

		nextChartVersion := os.Getenv("INPUT_REGISTRY_CHART_VERSION")
		if nextChartVersion != "" {
			logrus.Infof("using input override from INPUT_REGISTRY_CHART_VERSION: %s", nextChartVersion)
		} else {
			logrus.Infof("fetching the latest registry chart version")
			latest, err := LatestChartVersion(hcli, registryRepo, "docker-registry")
			if err != nil {
				return fmt.Errorf("failed to get the latest registry chart version: %v", err)
			}
			nextChartVersion = latest
			logrus.Printf("latest registry chart version: %s", latest)
		}
		nextChartVersion = strings.TrimPrefix(nextChartVersion, "v")

		current := registry.Metadata
		if current.Version == nextChartVersion && !c.Bool("force") {
			logrus.Infof("registry chart version is already up-to-date")
			return nil
		}

		logrus.Infof("mirroring registry chart version %s", nextChartVersion)
		if err := MirrorChart(hcli, registryRepo, "registry", nextChartVersion); err != nil {
			return fmt.Errorf("failed to mirror registry chart: %v", err)
		}

		upstream := fmt.Sprintf("%s/registry", os.Getenv("CHARTS_DESTINATION"))
		withproto := fmt.Sprintf("oci://proxy.replicated.com/anonymous/%s", upstream)

		logrus.Infof("updating registry images")

		err = updateRegistryAddonImages(c.Context, hcli, withproto, nextChartVersion, nil)
		if err != nil {
			return fmt.Errorf("failed to update registry images: %w", err)
		}

		logrus.Infof("successfully updated registry addon")

		return nil
	},
}

var updateRegistryImagesCommand = &cli.Command{
	Name:      "registry",
	Usage:     "Updates the registry images",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating registry images")

		hcli, err := NewHelm()
		if err != nil {
			return fmt.Errorf("failed to create helm client: %w", err)
		}
		defer hcli.Close()

		current := registry.Metadata

		err = updateRegistryAddonImages(c.Context, hcli, current.Location, current.Version, c.StringSlice("image"))
		if err != nil {
			return fmt.Errorf("failed to update registry images: %w", err)
		}

		logrus.Infof("successfully updated registry images")

		return nil
	},
}

func updateRegistryAddonImages(ctx context.Context, hcli helm.Client, chartURL string, chartVersion string, filteredImages []string) error {
	newmeta := release.AddonMetadata{
		Version:  chartVersion,
		Location: chartURL,
		Images:   make(map[string]release.AddonImage),
	}

	values, err := release.GetValuesWithOriginalImages("registry")
	if err != nil {
		return fmt.Errorf("unable to get registry values: %v", err)
	}

	logrus.Infof("extracting images from chart")
	images, err := helm.ExtractImagesFromChart(hcli, chartURL, chartVersion, values)
	if err != nil {
		return fmt.Errorf("failed to get images from chart: %w", err)
	}

	metaImages, err := UpdateImages(ctx, registryImageComponents, registry.Metadata.Images, images, filteredImages)
	if err != nil {
		return fmt.Errorf("failed to update images: %w", err)
	}
	newmeta.Images = metaImages

	logrus.Infof("saving addon manifest")
	if err := newmeta.Save("registry"); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	return nil
}
