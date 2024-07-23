package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
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
		latest = strings.TrimPrefix(latest, "v")
		logrus.Infof("found seaweedfs chart version %s", latest)

		current := seaweedfs.Metadata
		if current.Version == latest && !c.Bool("force") {
			logrus.Infof("seaweedfs chart is up to date")
			return nil
		}

		logrus.Infof("mirroring seaweedfs chart")
		if err := MirrorChart("seaweedfs", "seaweedfs", latest); err != nil {
			return fmt.Errorf("unable to mirror seaweedfs chart: %w", err)
		}

		upstream := fmt.Sprintf("%s/seaweedfs", os.Getenv("CHARTS_DESTINATION"))
		newmeta := release.AddonMetadata{
			Version:  latest,
			Location: fmt.Sprintf("oci://proxy.replicated.com/anonymous/%s", upstream),
			Images:   make(map[string]string),
		}

		values, err := release.GetValuesWithOriginalImages("seaweedfs")
		if err != nil {
			return fmt.Errorf("unable to get openebs values: %v", err)
		}

		logrus.Infof("extracting images from chart")
		withproto := fmt.Sprintf("oci://%s", upstream)
		images, err := GetImagesFromOCIChart(withproto, "seaweedfs", latest, values)
		if err != nil {
			return fmt.Errorf("failed to get images from admin console chart: %w", err)
		}

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
		if err := newmeta.Save("seaweedfs"); err != nil {
			return fmt.Errorf("failed to save metadata: %w", err)
		}

		logrus.Infof("successfully updated seaweed addon")
		return nil
	},
}
