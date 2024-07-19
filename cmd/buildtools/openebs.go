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

type addonComponent struct {
	name                        string
	wolfiPackageName            string
	getWolfiPackageName         func(upstreamVersion string) string
	upstreamVersionMakefileVar  string
	upstreamVersionFlagOverride string
}

var openebsComponents = []addonComponent{
	{
		name:                        "openebs-provisioner-localpv",
		wolfiPackageName:            "dynamic-localpv-provisioner",
		upstreamVersionMakefileVar:  "OPENEBS_IMAGE_VERSION",
		upstreamVersionFlagOverride: "openebs-version",
	},
	{
		name:                        "openebs-linux-utils",
		upstreamVersionMakefileVar:  "OPENEBS_IMAGE_VERSION",
		upstreamVersionFlagOverride: "openebs-version",
	},
	{
		name:                        "openebs-kubectl",
		getWolfiPackageName:         func(upstreamVersion string) string { return "kubectl-" + upstreamVersion + "-default" },
		upstreamVersionMakefileVar:  "OPENEBS_KUBECTL_IMAGE_VERSION",
		upstreamVersionFlagOverride: "kubectl-version",
	},
}

var updateOpenEBSImagesCommand = &cli.Command{
	Name:      "openebs",
	Usage:     "Updates the openebs images",
	UsageText: environmentUsageText,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "openebs-version",
			Usage: "The version of openebs to use to determine image versions",
		},
		&cli.StringFlag{
			Name:  "kubectl-version",
			Usage: "The version of kubectl to use to determine image versions",
		},
	},
	Action: func(c *cli.Context) error {
		logrus.Infof("updating openebs images")

		if err := ApkoLogin(); err != nil {
			return fmt.Errorf("failed to apko login: %w", err)
		}

		wolfiAPKIndex, err := GetWolfiAPKIndex()
		if err != nil {
			return fmt.Errorf("failed to get APK index: %w", err)
		}

		for _, component := range openebsComponents {
			upstreamVersion, err := getAddonComponentUpstreamVersion(c, component)
			if err != nil {
				return fmt.Errorf("failed to get upstream version for %s: %w", component.name, err)
			}

			packageName := component.wolfiPackageName
			if component.getWolfiPackageName != nil {
				packageName = component.getWolfiPackageName(upstreamVersion)
			}
			packageVersion := upstreamVersion
			if packageName != "" {
				packageVersion, err = GetWolfiPackageVersion(wolfiAPKIndex, packageName, upstreamVersion)
				if err != nil {
					return fmt.Errorf("failed to get package version for %s: %w", component.name, err)
				}
			}

			extraArgs := []string{}
			if packageName != "" {
				extraArgs = append(extraArgs, fmt.Sprintf("PACKAGE_NAME=%s", packageName))
			}
			if err := ApkoBuildAndPublish(component.name, packageVersion, extraArgs...); err != nil {
				return fmt.Errorf("failed to apko build and publish for %s: %w", component.name, err)
			}

			digest, err := GetDigestFromBuildFile()
			if err != nil {
				return fmt.Errorf("failed to get digest from build file: %w", err)
			}

			makefileVar := getAddonComponentImageTagMakefileVar(component)
			if err := SetMakefileVariable(makefileVar, fmt.Sprintf("%s@%s", packageVersion, digest)); err != nil {
				return fmt.Errorf("failed to set %s version: %w", component.name, err)
			}
		}

		return nil
	},
}

func getAddonComponentUpstreamVersion(c *cli.Context, component addonComponent) (string, error) {
	if ver := c.String(component.upstreamVersionFlagOverride); ver != "" {
		return ver, nil
	}
	ver, err := GetMakefileVariable(component.upstreamVersionMakefileVar)
	if err != nil {
		return "", fmt.Errorf("get version from makefile: %w", err)
	}
	return ver, nil
}

func getAddonComponentImageTagMakefileVar(component addonComponent) string {
	return fmt.Sprintf("%s_IMAGE_TAG", strings.ReplaceAll(strings.ToUpper(component.name), "-", "_"))
}
