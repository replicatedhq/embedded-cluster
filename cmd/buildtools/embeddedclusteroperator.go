package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

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

		upstream := "registry.replicated.com/library/embedded-cluster-operator"
		withproto := getChartURL(upstream)

		logrus.Infof("updating embedded cluster operator images")

		err := updateOperatorAddonImages(c.Context, withproto, nextChartVersion)
		if err != nil {
			return fmt.Errorf("failed to update embedded cluster operator images: %w", err)
		}

		logrus.Infof("successfully updated embedded cluster operator addon")

		return nil
	},
}

func updateOperatorAddonImages(ctx context.Context, chartURL string, chartVersion string) error {
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
		repo, tag, err := component.resolveImageRepoAndTag(ctx, image)
		if err != nil {
			return fmt.Errorf("failed to resolve image and tag for %s: %w", image, err)
		}
		newmeta.Images[component.name] = release.AddonImage{
			Registry: getImageRegistry(),
			Repo:     repo,
			Tag:      tag,
		}
	}

	logrus.Infof("saving addon manifest")
	newmeta.ReplaceImages = true
	if err := newmeta.Save("embeddedclusteroperator"); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	return nil
}

func getGitCommitHash() (string, error) {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	return string(out), err
}
