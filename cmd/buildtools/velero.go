package main

import (
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/velero"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
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

		current := velero.Metadata
		// compare with the version we are currently using.
		if current.Version == latest && !c.Bool("force") {
			logrus.Infof("velero chart is up to date")
			return nil
		}

		// attempt to mirror the chart.
		if err := MirrorChart("vmware-tanzu", "velero", latest); err != nil {
			return fmt.Errorf("unable to mirror velero chart: %w", err)
		}

		upstream := fmt.Sprintf("%s/velero", os.Getenv("DESTINATION"))
		newmeta := release.AddonMetadata{
			Version:  latest,
			Location: fmt.Sprintf("oci://proxy.replicated.com/anonymous/%s", upstream),
			Images:   make(map[string]string),
		}

		values, err := release.GetValuesWithOriginalImages("velero")
		if err != nil {
			return fmt.Errorf("unable to get openebs values: %v", err)
		}

		logrus.Infof("extracting images from chart")
		withproto := fmt.Sprintf("oci://%s", upstream)
		images, err := GetImagesFromOCIChart(withproto, "velero", latest, values)
		if err != nil {
			return fmt.Errorf("failed to get images from chart: %w", err)
		}

		awsver, err := GetLatestGitHubTag(c.Context, "vmware-tanzu", "velero-plugin-for-aws")
		if err != nil {
			return fmt.Errorf("failed to get latest velero plugin release: %w", err)
		}
		logrus.Infof("found latest velero plugin for aws version %s", latest)
		images = append(images, fmt.Sprintf("velero/velero-plugin-for-aws:%s", awsver))

		logrus.Infof("including velero helper image (using the same tag as the velero image)")
		var helper string
		for _, image := range images {
			tag := TagFromImage(image)
			image = RemoveTagFromImage(image)
			if image == "velero/velero" {
				helper = fmt.Sprintf("velero/velero-restore-helper:%s", tag)
				break
			}
		}
		if helper == "" {
			return fmt.Errorf("failed to find velero image tag")
		}
		images = append(images, helper)

		logrus.Infof("fetching digest for images")
		for _, image := range images {
			sha, err := GetImageDigest(c.Context, image)
			if err != nil {
				return fmt.Errorf("failed to get image %s digest: %w", image, err)
			}
			logrus.Infof("image %s digest: %s", image, sha)
			tag := TagFromImage(image)
			image = RemoveTagFromImage(image)
			newmeta.Images[image] = fmt.Sprintf("%s@%s", tag, sha)
		}

		logrus.Infof("saving addon manifest")
		newmeta.ReplaceImages = true
		if err := newmeta.Save("velero"); err != nil {
			return fmt.Errorf("failed to save metadata: %w", err)
		}

		logrus.Infof("successfully updated velero addon")
		return nil
	},
}
