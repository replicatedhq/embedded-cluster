package main

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var updateVeleroAddonCommand = &cli.Command{
	Name:      "velero",
	Usage:     "Updates the Velero addon",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating velero addon")
		latest, err := GetLatestGitHubTag(c.Context, "vmware-tanzu", "velero")
		if err != nil {
			return fmt.Errorf("failed to get latest velero release: %w", err)
		}
		if err := SetMakefileVariable("VELERO_IMAGE_VERSION", latest); err != nil {
			return fmt.Errorf("failed to set velero image version: %w", err)
		}

		latest, err = GetLatestGitHubTag(c.Context, "vmware-tanzu", "velero-plugin-for-aws")
		if err != nil {
			return fmt.Errorf("failed to get latest velero plugin release: %w", err)
		}
		if err := SetMakefileVariable("VELERO_AWS_PLUGIN_IMAGE_VERSION", latest); err != nil {
			return fmt.Errorf("failed to set velero plugin version: %w", err)
		}

		latest, err = LatestChartVersion("vmware-tanzu", "velero")
		if err != nil {
			return fmt.Errorf("unable to get the latest registry version: %v", err)
		}

		if original, err := GetMakefileVariable("VELERO_CHART_VERSION"); err != nil {
			return fmt.Errorf("unable to get velero chart version: %w", err)
		} else if original == latest && !c.Bool("force") {
			logrus.Infof("velero chart is up to date: %s", original)
			return nil
		}

		if err := MirrorChart("vmware-tanzu", "velero", latest, c.Bool("force")); err != nil {
			return fmt.Errorf("unable to mirror velero chart: %w", err)
		}

		if err := SetMakefileVariable("VELERO_CHART_VERSION", latest); err != nil {
			return fmt.Errorf("unable to set velero chart version: %w", err)
		}
		logrus.Infof("successfully updated velero addon")

		return nil
	},
}
