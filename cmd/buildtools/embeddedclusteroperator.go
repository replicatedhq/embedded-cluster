package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var operatorImageComponents = map[string]addonComponent{
	"docker.io/replicated/embedded-cluster-operator-image": {
		name:             "embedded-cluster-operator",
		useUpstreamImage: true,
	},
	"proxy.replicated.com/library/embedded-cluster-utils": {
		name:             "utils",
		useUpstreamImage: true,
	},
	"proxy.replicated.com/library/goldpinger": {
		name:             "goldpinger",
		useUpstreamImage: true,
	},
}

var updateOperatorAddonCommand = &cli.Command{
	Name:      "embeddedclusteroperator",
	Usage:     "Updates the Embedded Cluster Operator addon",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating embedded cluster operator addon")

		hcli, err := NewHelm()
		if err != nil {
			return fmt.Errorf("failed to create helm client: %w", err)
		}
		defer hcli.Close()

		nextChartVersion := os.Getenv("INPUT_OPERATOR_CHART_VERSION")
		if nextChartVersion != "" {
			logrus.Infof("using input override from INPUT_OPERATOR_CHART_VERSION: %s", nextChartVersion)
		} else {
			logrus.Infof("fetching the latest embedded cluster release")
			latest, err := GetGitHubRelease(
				c.Context, "replicatedhq", "embedded-cluster",
				func(tag string) bool {
					return !strings.Contains(tag, "build")
				},
			)
			if err != nil {
				return fmt.Errorf("failed to get embedded cluster release: %w", err)
			}
			nextChartVersion = strings.TrimPrefix(latest, "v")
			logrus.Printf("latest embedded cluster release: %s", latest)
		}
		nextChartVersion = strings.TrimPrefix(nextChartVersion, "v")

		chartURL := os.Getenv("INPUT_OPERATOR_CHART_URL")
		if chartURL != "" {
			logrus.Infof("using input override from INPUT_OPERATOR_CHART_URL: %s", chartURL)
			chartURL = strings.TrimPrefix(chartURL, "oci://")
		} else {
			chartURL = "proxy.replicated.com/library/embedded-cluster-operator"
		}
		chartURL = addProxyAnonymousPrefix(chartURL)
		chartURL = fmt.Sprintf("oci://%s", chartURL)

		imageOverride := os.Getenv("INPUT_OPERATOR_IMAGE")
		if imageOverride != "" {
			logrus.Infof("using input override from INPUT_OPERATOR_IMAGE: %s", imageOverride)

			operatorImageComponents[imageOverride] = addonComponent{
				name:             "embedded-cluster-operator",
				useUpstreamImage: true,
			}
		}

		err = updateOperatorAddonImages(c.Context, hcli, chartURL, nextChartVersion, nil)
		if err != nil {
			return fmt.Errorf("failed to update embedded cluster operator images: %w", err)
		}

		logrus.Infof("successfully updated embedded cluster operator addon")

		return nil
	},
}

var updateOperatorImagesCommand = &cli.Command{
	Name:      "embeddedclusteroperator",
	Usage:     "Updates the embedded cluster operator images",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating embedded cluster operator images")

		hcli, err := NewHelm()
		if err != nil {
			return fmt.Errorf("failed to create helm client: %w", err)
		}
		defer hcli.Close()

		current := embeddedclusteroperator.Metadata

		err = updateOperatorAddonImages(c.Context, hcli, current.Location, current.Version, c.StringSlice("image"))
		if err != nil {
			return fmt.Errorf("failed to update embedded cluster operator images: %w", err)
		}

		logrus.Infof("successfully updated embedded cluster operator images")

		return nil
	},
}

func updateOperatorAddonImages(ctx context.Context, hcli helm.Client, chartURL string, chartVersion string, filteredImages []string) error {
	newmeta := release.AddonMetadata{
		Version:  chartVersion,
		Location: chartURL,
		Images:   make(map[string]release.AddonImage),
	}

	values, err := release.GetValuesWithOriginalImages("embeddedclusteroperator")
	if err != nil {
		return fmt.Errorf("failed to get embedded cluster operator values: %v", err)
	}

	logrus.Infof("extracting images from chart version %s", chartVersion)
	images, err := helm.ExtractImagesFromChart(hcli, chartURL, chartVersion, values)
	if err != nil {
		return fmt.Errorf("failed to get images from embedded cluster operator chart: %w", err)
	}

	// make sure we include the operator util and goldpinger images as they don't show up when rendering the helm chart.
	images = append(images, "proxy.replicated.com/library/embedded-cluster-utils:latest")
	images = append(images, "proxy.replicated.com/library/goldpinger:latest")

	metaImages, err := UpdateImages(ctx, operatorImageComponents, embeddedclusteroperator.Metadata.Images, images, filteredImages)
	if err != nil {
		return fmt.Errorf("failed to update images: %w", err)
	}
	newmeta.Images = metaImages

	logrus.Infof("saving addon manifest")
	if err := newmeta.Save("embeddedclusteroperator"); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	return nil
}
