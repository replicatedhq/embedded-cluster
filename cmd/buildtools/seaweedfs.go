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

		if original, err := GetMakefileVariable("SEAWEEDFS_CHART_VERSION"); err != nil {
			return fmt.Errorf("unable to get seaweedfs chart version: %w", err)
		} else if original == latest && !c.Bool("force") {
			logrus.Infof("seaweedfs chart is up to date: %s", original)
			return nil
		}

		if err := MirrorChart("seaweedfs", "seaweedfs", latest); err != nil {
			return fmt.Errorf("unable to mirror seaweedfs chart: %w", err)
		}

		if err := SetMakefileVariable("SEAWEEDFS_CHART_VERSION", latest); err != nil {
			return fmt.Errorf("unable to set seaweedfs chart version: %w", err)
		}

		return nil
	},
}
