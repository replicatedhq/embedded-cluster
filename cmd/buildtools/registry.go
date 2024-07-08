package main

import (
	"fmt"

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
		logrus.Printf("latest registry chart version: %s", latest)

		original, err := GetMakefileVariable("REGISTRY_CHART_VERSION")
		if err != nil {
			return fmt.Errorf("unable to get value: %w", err)
		} else if latest == original && !c.Bool("force") {
			logrus.Infof("registry version is already up-to-date: %s", original)
			return nil
		}

		logrus.Printf("updating registry makefile chart version to %s", latest)
		if err := SetMakefileVariable("REGISTRY_CHART_VERSION", latest); err != nil {
			return fmt.Errorf("unable to patch makefile: %w", err)
		}

		logrus.Infof("mirroring registry chart version %s", latest)
		if err := MirrorChart("twuni", "docker-registry", latest); err != nil {
			return fmt.Errorf("unable to mirror chart: %w", err)
		}

		imgver, err := RenderChartAndFindImageDigest(
			c.Context,
			"twuni",
			"docker-registry",
			latest,
			map[string]interface{}{},
			"registry",
		)
		if err != nil {
			return fmt.Errorf("unable to find registry image digest: %v", err)
		}

		logrus.Infof("updating registry image tag to %s", imgver)
		if err := SetMakefileVariable("REGISTRY_IMAGE_TAG", imgver); err != nil {
			return fmt.Errorf("unable to patch makefile: %w", err)
		}

		logrus.Infof("successfully updated registry addon")
		return nil
	},
}
