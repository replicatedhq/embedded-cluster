package main

import (
	"fmt"
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var updateOperatorAddonCommand = &cli.Command{
	Name:      "embeddedclusteroperator",
	Usage:     "Updates the Embedded Cluster Operator addon",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating operator addon")

		logrus.Infof("getting embedded cluster operator release")
		latest, err := GetGitHubRelease(
			c.Context, "replicatedhq", "embedded-cluster-operator",
			func(tag string) bool {
				return !strings.Contains(tag, "build")
			},
		)
		if err != nil {
			return fmt.Errorf("failed to get embedded cluster operator release: %w", err)
		}
		latest = strings.TrimPrefix(latest, "v")
		logrus.Infof("embedded cluster operator release found: %s", latest)

		current := embeddedclusteroperator.Metadata
		if current.Version == latest && !c.Bool("force") {
			logrus.Infof("operator chart version is already up-to-date")
			return nil
		}

		upstream := "registry.replicated.com/library/embedded-cluster-operator"
		newmeta := release.AddonMetadata{
			Version:  latest,
			Location: fmt.Sprintf("oci://proxy.replicated.com/anonymous/%s", upstream),
			Images:   make(map[string]string),
		}

		values, err := release.GetValuesWithOriginalImages("embeddedclusteroperator")
		if err != nil {
			return fmt.Errorf("unable to get openebs values: %v", err)
		}

		logrus.Infof("extracting images from chart")
		withproto := fmt.Sprintf("oci://%s", upstream)
		images, err := GetImagesFromOCIChart(withproto, "embeddedclusteroperator", latest, values)
		if err != nil {
			return fmt.Errorf("failed to get images from embedded cluster operator chart: %w", err)
		}

		// make sure we include the operator util image as it does not show up
		// when rendering the helm chart.
		images = append(images, "busybox:1.36")

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
		if err := newmeta.Save("embeddedclusteroperator"); err != nil {
			return fmt.Errorf("failed to save embedded cluster operator metadata: %w", err)
		}
		return nil
	},
}
