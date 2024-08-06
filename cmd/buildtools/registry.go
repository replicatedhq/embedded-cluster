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
	"github.com/replicatedhq/embedded-cluster/pkg/release"
)

var registryRepo = &repo.Entry{
	Name: "twuni",
	URL:  "https://helm.twun.io",
}

var registryImageComponents = map[string]addonComponent{
	"docker.io/library/registry": {
		name: "registry",
		getWolfiPackageName: func(opts addonComponentOptions) string {
			return "distribution"
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

		nextChartVersion := os.Getenv("INPUT_REGISTRY_CHART_VERSION")
		if nextChartVersion != "" {
			logrus.Infof("using input override from INPUT_REGISTRY_CHART_VERSION: %s", nextChartVersion)
		} else {
			logrus.Infof("fetching the latest registry chart version")
			latest, err := LatestChartVersion(registryRepo, "docker-registry")
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
		} else {
			logrus.Infof("mirroring registry chart version %s", nextChartVersion)
			if err := MirrorChart(registryRepo, "docker-registry", nextChartVersion); err != nil {
				return fmt.Errorf("failed to mirror registry chart: %v", err)
			}
		}

		upstream := fmt.Sprintf("%s/docker-registry", os.Getenv("CHARTS_DESTINATION"))
		withproto := fmt.Sprintf("oci://proxy.replicated.com/anonymous/%s", upstream)

		logrus.Infof("updating registry images")

		err := updateRegistryAddonImages(c.Context, withproto, nextChartVersion)
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

		current := registry.Metadata

		err := updateRegistryAddonImages(c.Context, current.Location, current.Version)
		if err != nil {
			return fmt.Errorf("failed to update registry images: %w", err)
		}

		logrus.Infof("successfully updated registry images")

		return nil
	},
}

func updateRegistryAddonImages(ctx context.Context, chartURL string, chartVersion string) error {
	newmeta := release.AddonMetadata{
		Version:  chartVersion,
		Location: chartURL,
		Images:   make(map[string]release.AddonImage),
	}

	values, err := release.GetValuesWithOriginalImages("registry")
	if err != nil {
		return fmt.Errorf("failed to get registry values: %v", err)
	}

	logrus.Infof("extracting images from chart version %s", chartVersion)
	images, err := GetImagesFromOCIChart(chartURL, "docker-registry", chartVersion, values)
	if err != nil {
		return fmt.Errorf("failed to get images from registry chart: %w", err)
	}

	if err := ApkoLogin(); err != nil {
		return fmt.Errorf("failed to apko login: %w", err)
	}

	for _, image := range images {
		component, ok := registryImageComponents[RemoveTagFromImage(image)]
		if !ok {
			return fmt.Errorf("no component found for image %s", image)
		}
		repo, tag, err := component.resolveImageRepoAndTag(ctx, image)
		if err != nil {
			return fmt.Errorf("failed to resolve image and tag for %s: %w", image, err)
		}
		newmeta.Images[component.name] = release.AddonImage{
			Repo: repo,
			Tag:  tag,
		}
	}

	logrus.Infof("saving addon manifest")
	newmeta.ReplaceImages = true
	if err := newmeta.Save("registry"); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	logrus.Infof("rendering values for registry ha")
	if err := newmeta.RenderValues("registry", "values-ha.tpl.yaml", "values-ha.yaml"); err != nil {
		return fmt.Errorf("failed to render ha values: %w", err)
	}

	return nil
}
