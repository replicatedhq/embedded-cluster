package main

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

const UtilsImageJson = "https://raw.githubusercontent.com/docker-library/busybox/master/versions.json"

func SetOpenEBSUtilsImageTag(c *cli.Context) error {
	logrus.Infof("fetching the latest openebs utils image tag")
	version, err := GetLatestGitHubRelease(c.Context, "openebs", "linux-utils")
	if err != nil {
		return fmt.Errorf("unable to get the latest utils image version: %v", err)
	}
	logrus.Infof("latest github openebs utils image release: %s", version)

	tag := strings.TrimPrefix(version, "v")
	img := fmt.Sprintf("openebs/linux-utils:%s", tag)
	digest, err := GetImageDigest(c.Context, img)
	if err != nil {
		return fmt.Errorf("unable to get the digest for %s: %v", img, err)
	}

	tag = fmt.Sprintf("%s@%s", tag, digest)
	logrus.Infof("found openebs/linux-utils image tag: %s", tag)
	if err := SetMakefileVariable("OPENEBS_UTILS_IMAGE_TAG", tag); err != nil {
		return fmt.Errorf("unable to update the makefile: %v", err)
	}

	logrus.Info("successfully updated the utils image tag in makefile")
	return nil
}

func SetOpenEBSVersion(c *cli.Context) (string, bool, error) {
	logrus.Infof("fetching the latest openebs chart version")
	latest, err := LatestChartVersion("openebs", "openebs")
	if err != nil {
		return "", false, fmt.Errorf("unable to get the latest openebs chart version: %v", err)
	}
	logrus.Printf("latest openebs chart version: %s", latest)

	original, err := GetMakefileVariable("OPENEBS_CHART_VERSION")
	if err != nil {
		return "", false, fmt.Errorf("unable to get value: %w", err)
	} else if latest == original {
		logrus.Infof("openebs chart version is already up-to-date: %s", original)
		return latest, false, nil
	}

	logrus.Infof("updating openebs makefile chart version to %s", latest)
	if err := SetMakefileVariable("OPENEBS_CHART_VERSION", latest); err != nil {
		return "", false, fmt.Errorf("unable to patch makefile: %w", err)
	}
	return latest, true, nil
}

var updateOpenEBSAddonCommand = &cli.Command{
	Name:      "openebs",
	Usage:     "Updates the OpenEBS addon",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating openebs addon")

		logrus.Infof("updating openebs chart version")
		newver, updated, err := SetOpenEBSVersion(c)
		if err != nil {
			return fmt.Errorf("unable to update the openebs chart version: %v", err)
		} else if !updated && !c.Bool("force") {
			return nil
		}

		logrus.Infof("mirroring openebs chart version %s", newver)
		if err := MirrorChart("openebs", "openebs", newver); err != nil {
			return fmt.Errorf("unable to mirror openebs chart: %v", err)
		}

		imgver, err := RenderChartAndFindImageDigest(
			c.Context,
			"openebs",
			"openebs",
			newver,
			map[string]interface{}{},
			"openebs/provisioner-localpv",
		)
		if err != nil {
			return fmt.Errorf("unable to find openebs image digest: %v", err)
		}

		if err := SetMakefileVariable("OPENEBS_IMAGE_TAG", imgver); err != nil {
			return fmt.Errorf("unable to set openebs image tag")
		}

		logrus.Infof("updating openebs utils image tag")
		if err := SetOpenEBSUtilsImageTag(c); err != nil {
			return fmt.Errorf("unable to update the openebs utils image version: %v", err)
		}

		logrus.Infof("successfully updated openebs addon")
		return nil
	},
}
