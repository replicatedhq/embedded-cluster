package main

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var updateSeaweedFSAddonCommand = &cli.Command{
	Name:      "seaweedfs",
	Usage:     "Updates the SeaweedFS addon",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating seaweedfs addon")
		latest, err := LatestChartVersion("seaweedfs", "seaweedfs")
		if err != nil {
			return fmt.Errorf("unable to get the latest seaweedfs version: %v", err)
		}
		logrus.Infof("found seaweedfs chart version %s", latest)

		if original, err := GetMakefileVariable("SEAWEEDFS_CHART_VERSION"); err != nil {
			return fmt.Errorf("unable to get seaweedfs chart version: %w", err)
		} else if original == latest && !c.Bool("force") {
			logrus.Infof("seaweedfs chart is up to date: %s", original)
			return nil
		}

		logrus.Infof("mirroring seaweedfs chart")
		if err := MirrorChart("seaweedfs", "seaweedfs", latest); err != nil {
			return fmt.Errorf("unable to mirror seaweedfs chart: %w", err)
		}

		logrus.Infof("updating seaweedfs chart version")
		if err := SetMakefileVariable("SEAWEEDFS_CHART_VERSION", latest); err != nil {
			return fmt.Errorf("unable to set seaweedfs chart version: %w", err)
		}

		imgver, err := RenderChartAndFindImageDigest(
			c.Context,
			"seaweedfs",
			"seaweedfs",
			latest,
			map[string]interface{}{},
			"chrislusf/seaweedfs",
		)
		if err != nil {
			return fmt.Errorf("unable to find seaweedfs image digest: %v", err)
		}

		logrus.Infof("updating seaweedfs image tag on makefile")
		if err := SetMakefileVariable("SEAWEEDFS_IMAGE_TAG", imgver); err != nil {
			return fmt.Errorf("unable to set seaweedfs image version")
		}

		logrus.Infof("successfully updated seaweed addon")
		return nil
	},
}
