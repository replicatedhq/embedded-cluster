package main

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var updateVeleroAddonCommand = &cli.Command{
	Name:      "velero",
	Usage:     "Updates the Velero addon",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating velero addon")

		// check what is the latest version of the chart.
		latest, err := LatestChartVersion("vmware-tanzu", "velero")
		if err != nil {
			return fmt.Errorf("unable to get the latest registry version: %v", err)
		}

		// compare with the version we are currently using.
		if original, err := GetMakefileVariable("VELERO_CHART_VERSION"); err != nil {
			return fmt.Errorf("unable to get velero chart version: %w", err)
		} else if original == latest && !c.Bool("force") {
			logrus.Infof("velero chart is up to date: %s", original)
			return nil
		}

		// attempt to mirror the chart.
		if err := MirrorChart("vmware-tanzu", "velero", latest); err != nil {
			return fmt.Errorf("unable to mirror velero chart: %w", err)
		}

		// set the ner chart version in the makefile.
		logrus.Infof("updating velero chart version to %s", latest)
		if err := SetMakefileVariable("VELERO_CHART_VERSION", latest); err != nil {
			return fmt.Errorf("unable to set velero chart version: %w", err)
		}

		// verify what version of the velero/velero image the chart renders and
		// fetches the digest for it.
		imgver, err := RenderChartAndFindImageDigest(
			c.Context,
			"vmware-tanzu",
			"velero",
			latest,
			map[string]interface{}{},
			"velero/velero",
		)
		if err != nil {
			return fmt.Errorf("unable to find velero image digest: %v", err)
		}

		// set the velero image chart in the makefile.
		logrus.Infof("setting velero image tag to %s", imgver)
		if err := SetMakefileVariable("VELERO_IMAGE_TAG", imgver); err != nil {
			return fmt.Errorf("failed to set velero image tag: %w", err)
		}

		// find out what is the digest for the velero helper image.
		tag, _, found := strings.Cut(imgver, "@")
		if !found {
			return fmt.Errorf("unable to extract tag from %s", imgver)
		}
		helper := fmt.Sprintf("velero/velero-restore-helper:%s", tag)
		helperdigest, err := GetImageDigest(c.Context, helper)
		if err != nil {
			return fmt.Errorf("unable to get velero restore helper digest: %v", err)
		}
		helpertag := fmt.Sprintf("%s@%s", tag, helperdigest)
		logrus.Infof("setting velero restore helper image tag on makefile")
		if err := SetMakefileVariable("VELERO_RESTORE_HELPER_IMAGE_TAG", helpertag); err != nil {
			return fmt.Errorf("failed to set velero plugin version: %w", err)
		}

		// we need to process the velero image used for kubectl.
		logrus.Infof("finding tag of kubectl image")
		imgver, err = RenderChartAndFindImageDigest(
			c.Context,
			"vmware-tanzu",
			"velero",
			latest,
			map[string]interface{}{},
			"docker.io/bitnami/kubectl",
		)
		if err != nil {
			return fmt.Errorf("unable to find kubectl image digest: %v", err)
		}

		logrus.Infof("setting kubectl image tag on makefile")
		if err := SetMakefileVariable("VELERO_KUBECTL_IMAGE_TAG", imgver); err != nil {
			return fmt.Errorf("failed to set velero plugin version: %w", err)
		}

		// we need to process a few images individually, the first one is the velero
		// plugin for the aws provider.
		logrus.Infof("finding latest stable version of velero plugin for aws")
		latest, err = GetLatestGitHubTag(c.Context, "vmware-tanzu", "velero-plugin-for-aws")
		if err != nil {
			return fmt.Errorf("failed to get latest velero plugin release: %w", err)
		}
		logrus.Infof("found latest velero plugin for aws version %s", latest)

		logrus.Infof("finding digest for velero plugin for aws version %s", latest)
		imgpath := fmt.Sprintf("velero/velero-plugin-for-aws:%s", latest)
		digest, err := GetImageDigest(c.Context, imgpath)
		if err != nil {
			return fmt.Errorf("failed to get velero plugin for aws image digest: %w", err)
		}
		latest = fmt.Sprintf("%s@%s", latest, digest)
		logrus.Infof("found velero plugin for aws image digest: %s", digest)

		logrus.Infof("setting velero plugin for aws image tag on makefile")
		if err := SetMakefileVariable("VELERO_AWS_PLUGIN_IMAGE_TAG", latest); err != nil {
			return fmt.Errorf("failed to set velero plugin tag: %w", err)
		}

		logrus.Infof("successfully updated velero addon")
		return nil
	},
}
