package main

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var updateRegistryAddonCommand = &cli.Command{
	Name:      "registry",
	Usage:     "Updates the Registry addon",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating registry addon")
		latest, err := LatestChartVersion("twuni", "docker-registry")
		if err != nil {
			return fmt.Errorf("unable to get the latest registry version: %v", err)
		}
		original, err := GetMakefileVariable("REGISTRY_CHART_VERSION")
		if err != nil {
			return fmt.Errorf("unable to get value: %w", err)
		} else if latest == original && !c.Bool("force") {
			logrus.Infof("registry version is already up-to-date: %s", original)
			return nil
		}

		if err := MirrorChart("twuni", "docker-registry", latest, c.Bool("force")); err != nil {
			return fmt.Errorf("unable to mirror chart: %w", err)
		}

		if err := SetMakefileVariable("REGISTRY_CHART_VERSION", latest); err != nil {
			return fmt.Errorf("unable to patch makefile: %w", err)
		}

		latest, err = GetLatestGitHubTag(c.Context, "distribution", "distribution")
		if err != nil {
			return fmt.Errorf("unable to fetch distribution tag: %w", err)
		}
		latest = strings.TrimPrefix(latest, "v")

		if err := SetMakefileVariable("REGISTRY_IMAGE_VERSION", latest); err != nil {
			return fmt.Errorf("unable to patch makefile: %w", err)
		}
		logrus.Infof("successfully updated registry addon")

		return nil
	},
}
