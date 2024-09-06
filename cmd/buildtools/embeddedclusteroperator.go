package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var operatorImageComponents = map[string]addonComponent{
	"docker.io/replicated/embedded-cluster-operator-image": {
		name:             "embedded-cluster-operator",
		useUpstreamImage: true,
	},
	"docker.io/library/busybox": {
		name: "utils",
	},
}

var updateOperatorAddonCommand = &cli.Command{
	Name:      "embeddedclusteroperator",
	Usage:     "Updates the Embedded Cluster Operator addon",
	UsageText: environmentUsageText,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "skip-build-and-publish",
			Usage: "Skip building and publishing the operator image",
		},
	},
	Action: func(c *cli.Context) error {
		logrus.Infof("updating embedded cluster operator addon")

		nextChartVersion := os.Getenv("INPUT_OPERATOR_CHART_VERSION")
		if nextChartVersion != "" {
			logrus.Infof("using input override from INPUT_OPERATOR_CHART_VERSION: %s", nextChartVersion)
		} else {
			logrus.Infof("fetching the latest embedded cluster operator release")
			latest, err := GetGitHubRelease(
				c.Context, "replicatedhq", "embedded-cluster-operator",
				func(tag string) bool {
					return !strings.Contains(tag, "build")
				},
			)
			if err != nil {
				return fmt.Errorf("failed to get embedded cluster operator release: %w", err)
			}
			nextChartVersion = strings.TrimPrefix(latest, "v")
			logrus.Printf("latest embedded cluster operator release: %s", latest)
		}
		nextChartVersion = strings.TrimPrefix(nextChartVersion, "v")

		chartURL := os.Getenv("INPUT_OPERATOR_CHART_URL")
		if chartURL != "" {
			logrus.Infof("using input override from INPUT_OPERATOR_CHART_URL: %s", chartURL)
			chartURL = strings.TrimPrefix(chartURL, "oci://")
			chartURL = strings.TrimPrefix(chartURL, "proxy.replicated.com/anonymous/")
		} else {
			chartURL = "registry.replicated.com/library/embedded-cluster-operator"
		}
		chartURL = fmt.Sprintf("oci://proxy.replicated.com/anonymous/%s", chartURL)

		imageOverride := os.Getenv("INPUT_OPERATOR_IMAGE")
		if imageOverride != "" {
			logrus.Infof("using input override from INPUT_OPERATOR_IMAGE: %s", imageOverride)

			operatorImageComponents[imageOverride] = addonComponent{
				name:             "embedded-cluster-operator",
				useUpstreamImage: true,
			}
		}

		err := updateOperatorAddonImages(c.Context, chartURL, nextChartVersion, c.Bool("skip-build-and-publish"))
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
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "skip-build-and-publish",
			Usage: "Skip building and publishing the operator image",
		},
	},
	Action: func(c *cli.Context) error {
		logrus.Infof("updating embedded cluster operator images")

		current := embeddedclusteroperator.Metadata

		err := updateOperatorAddonImages(c.Context, current.Location, current.Version, c.Bool("skip-build-and-publish"))
		if err != nil {
			return fmt.Errorf("failed to update embedded cluster operator images: %w", err)
		}

		logrus.Infof("successfully updated embedded cluster operator images")

		return nil
	},
}

func updateOperatorAddonImages(ctx context.Context, chartURL string, chartVersion string, skipBuildAndPublish bool) error {
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
	images, err := GetImagesFromOCIChart(chartURL, "embeddedclusteroperator", chartVersion, values)
	if err != nil {
		return fmt.Errorf("failed to get images from embedded cluster operator chart: %w", err)
	}

	// make sure we include the operator util image as it does not show up when rendering the helm
	// chart.
	images = append(images, "docker.io/library/busybox:latest")

	if err := ApkoLogin(); err != nil {
		return fmt.Errorf("failed to apko login: %w", err)
	}

	for _, image := range images {
		component, ok := operatorImageComponents[RemoveTagFromImage(image)]
		if !ok {
			return fmt.Errorf("no component found for image %s", image)
		}
		repo, tag, err := component.resolveImageRepoAndTag(ctx, image, skipBuildAndPublish)
		if err != nil {
			return fmt.Errorf("failed to resolve image and tag for %s: %w", image, err)
		}
		newmeta.Images[component.name] = release.AddonImage{
			Repo: repo,
			Tag: map[string]string{
				"amd64": tag,
				// TODO (@salah): automate updating the arm64 tag
				"arm64": embeddedclusteroperator.Metadata.Images[component.name].Tag["arm64"],
			},
		}
	}

	logrus.Infof("saving addon manifest")
	if err := newmeta.Save("embeddedclusteroperator"); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	return nil
}

func getGitCommitHash() (string, error) {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	return string(out), err
}
