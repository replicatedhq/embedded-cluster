package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var updateOpenEBSAddonCommand = &cli.Command{
	Name:      "openebs",
	Usage:     "Updates the OpenEBS addon",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating openebs addon")

		logrus.Infof("fetching the latest openebs chart version")
		latest, err := LatestChartVersion("openebs", "openebs")
		if err != nil {
			return fmt.Errorf("unable to get the latest openebs chart version: %v", err)
		}
		latest = strings.TrimPrefix(latest, "v")
		logrus.Printf("latest openebs chart version: %s", latest)

		current := openebs.Metadata
		if current.Version == latest && !c.Bool("force") {
			logrus.Infof("openebs chart version is already up-to-date")
			return nil
		}

		logrus.Infof("mirroring openebs chart version %s", latest)
		if err := MirrorChart("openebs", "openebs", latest); err != nil {
			return fmt.Errorf("unable to mirror openebs chart: %v", err)
		}

		upstream := fmt.Sprintf("%s/openebs", os.Getenv("DESTINATION"))
		newmeta := release.AddonMetadata{
			Version:  latest,
			Location: fmt.Sprintf("oci://proxy.replicated.com/anonymous/%s", upstream),
			Images:   make(map[string]string),
		}

		values, err := release.GetValuesWithOriginalImages("openebs")
		if err != nil {
			return fmt.Errorf("unable to get openebs values: %v", err)
		}

		logrus.Infof("extracting images from chart")
		withproto := fmt.Sprintf("oci://%s", upstream)
		images, err := GetImagesFromOCIChart(withproto, "openebs", latest, values)
		if err != nil {
			return fmt.Errorf("failed to get images from admin console chart: %w", err)
		}

		// make sure we include the linux-utils image.
		logrus.Infof("fetching the latest openebs utils image tag")
		version, err := GetLatestGitHubRelease(c.Context, "openebs", "linux-utils")
		if err != nil {
			return fmt.Errorf("unable to get the latest utils image version: %v", err)
		}
		logrus.Infof("latest github openebs utils image release: %s", version)
		tag := strings.TrimPrefix(version, "v")
		images = append(images, fmt.Sprintf("openebs/linux-utils:%s", tag))

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
		if err := newmeta.Save("openebs"); err != nil {
			return fmt.Errorf("failed to save metadata: %w", err)
		}

		logrus.Infof("successfully updated openebs addon")
		return nil
	},
}
