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

var operatorImageComponents = map[string]string{
	"docker.io/replicated/embedded-cluster-operator-image": "embedded-cluster-operator",
	"docker.io/library/busybox":                            "utils",
}

var operatorComponents = map[string]addonComponent{
	"embedded-cluster-operator": {
		useUpstreamImage: true,
	},
	"utils": {
		upstreamVersionInputOverride: "INPUT_EMBEDDED_CLUSTER_VERSION",
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
		withproto := fmt.Sprintf("oci://proxy.replicated.com/anonymous/%s", upstream)

		logrus.Infof("updating embedded cluster operator images")

		err := updateOperatorAddonImages(c.Context, withproto, nextChartVersion, nextChartVersion)
		if err != nil {
			return fmt.Errorf("failed to update embedded cluster operator images: %w", err)
		}

		logrus.Infof("successfully updated embedded cluster operator addon")

		return nil
	},
}

func updateOperatorAddonImages(ctx context.Context, chartURL string, chartVersion string, linuxUtilsVersion string) error {
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
		logrus.Infof("updating image %s", image)

		upstreamVersion := TagFromImage(image)
		imageNoTag := RemoveTagFromImage(image)

		componentName, ok := operatorImageComponents[imageNoTag]
		if !ok {
			return fmt.Errorf("no component found for image %s", imageNoTag)
		}

		component, ok := operatorComponents[componentName]
		if !ok {
			return fmt.Errorf("no component found for component name %s", componentName)
		}

		if component.useUpstreamImage {
			logrus.Infof("fetching digest for image %s", image)
			sha, err := GetImageDigest(ctx, image)
			if err != nil {
				return fmt.Errorf("failed to get image %s digest: %w", image, err)
			}
			logrus.Infof("image %s digest: %s", image, sha)
			tag := TagFromImage(image)
			image = RemoveTagFromImage(image)
			newmeta.Images[FamiliarImageName(image)] = fmt.Sprintf("%s@%s", tag, sha)
			continue
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
	if err := newmeta.Save("embeddedclusteroperator"); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	return nil
}

func getGitCommitHash() (string, error) {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	return string(out), err
}
