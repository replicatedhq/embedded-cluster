package main

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

const UtilsImageJson = "https://raw.githubusercontent.com/docker-library/busybox/master/versions.json"

func SetOpenEBSUtilsImageVersion(c *cli.Context) error {
	logrus.Infof("fetching the latest openebs utils image version")
	version, err := GetLatestGitHubRelease(c.Context, "openebs", "linux-utils")
	if err != nil {
		return fmt.Errorf("unable to get the latest utils image version: %v", err)
	}

	version = strings.TrimPrefix(version, "v")
	logrus.Infof("updating utils image version (%s) in makefile", version)
	if err := SetMakefileVariable("OPENEBS_UTILS_VERSION", version); err != nil {
		return fmt.Errorf("unable to update the makefile: %v", err)
	}

	logrus.Info("successfully updated the utils image version in makefile")
	return nil
}

func SetOpenEBSVersion(c *cli.Context) (string, bool, error) {
	logrus.Infof("fetching the latest openebs version")
	latest, err := LatestChartVersion("openebs", "openebs")
	if err != nil {
		return "", false, fmt.Errorf("unable to get the latest openebs version: %v", err)
	}
	logrus.Printf("latest github openebs release: %s", latest)

	original, err := GetMakefileVariable("OPENEBS_CHART_VERSION")
	if err != nil {
		return "", false, fmt.Errorf("unable to get value: %w", err)
	} else if latest == original {
		logrus.Infof("openebs version is already up-to-date: %s", original)
		return latest, false, nil
	}

	logrus.Infof("updating openebs makefile version to %s", latest)
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
		logrus.Infof("updating openebs utils image version")
		if err := SetOpenEBSUtilsImageVersion(c); err != nil {
			return fmt.Errorf("unable to update the openebs utils image version: %v", err)
		}

		logrus.Infof("updating openebs version")
		newver, updated, err := SetOpenEBSVersion(c)
		if err != nil {
			return fmt.Errorf("unable to update the openebs version: %v", err)
		} else if !updated && !c.Bool("force") {
			return nil
		}

		logrus.Infof("mirroring new openebs chart version %s", newver)
		if err := MirrorChart("openebs", "openebs", newver); err != nil {
			return fmt.Errorf("unable to mirror openebs chart: %v", err)
		}

		logrus.Infof("successfully updated openebs addon")
		return nil
	},
}
