package main

import (
	"fmt"
	"os"

	"github.com/coreos/go-semver/semver"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"helm.sh/helm/v3/pkg/repo"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
)

var registryRepo = &repo.Entry{
	Name: "twuni",
	URL:  "https://helm.twun.io",
}

var updateRegistryAddonCommand = &cli.Command{
	Name:      "registry",
	Usage:     "Updates the Registry addon",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating registry addon")
		latest, err := LatestChartVersion(registryRepo, "docker-registry")
		if err != nil {
			return fmt.Errorf("unable to get the latest registry version: %v", err)
		}
		logrus.Printf("latest registry chart version: %s", latest)

		current := registry.Metadata
		if current.Version == latest && !c.Bool("force") {
			logrus.Infof("registry version is already up-to-date")
			return nil
		}

		logrus.Infof("mirroring registry chart version %s", latest)
		if err := MirrorChart(registryRepo, "docker-registry", latest); err != nil {
			return fmt.Errorf("unable to mirror chart: %w", err)
		}

		upstream := fmt.Sprintf("%s/docker-registry", os.Getenv("CHARTS_DESTINATION"))
		newmeta := release.AddonMetadata{
			Version:  latest,
			Location: fmt.Sprintf("oci://proxy.replicated.com/anonymous/%s", upstream),
			Images:   make(map[string]string),
		}

		values, err := release.GetValuesWithOriginalImages("registry")
		if err != nil {
			return fmt.Errorf("unable to get openebs values: %v", err)
		}

		logrus.Infof("extracting images from chart")
		withproto := fmt.Sprintf("oci://%s", upstream)
		images, err := GetImagesFromOCIChart(withproto, "docker-registry", latest, values)
		if err != nil {
			return fmt.Errorf("failed to get images from chart: %w", err)
		}

		// XXX we have already released a helm chart using registry 2.8.3 so we need
		// to avoid downgrading the registry version.
		minver, err := semver.NewVersion("2.8.3")
		if err != nil {
			return fmt.Errorf("unable to parse min version: %v", err)
		}

		logrus.Infof("fetching digest for images")
		for _, image := range images {
			tag := TagFromImage(image)
			withoutTag := RemoveTagFromImage(image)
			if withoutTag == "registry" {
				// replace the registry image with the minimum version if needed.
				if v, err := semver.NewVersion(tag); err == nil && v.LessThan(*minver) {
					logrus.Warnf("using registry %s instead of %s", minver, v)
					tag = minver.String()
					image = fmt.Sprintf("%s:%s", withoutTag, tag)
				}
			}

			sha, err := GetImageDigest(c.Context, image)
			if err != nil {
				return fmt.Errorf("failed to get image %s digest: %w", image, err)
			}
			logrus.Infof("image %s digest: %s", image, sha)

			newmeta.Images[FamiliarImageName(withoutTag)] = fmt.Sprintf("%s@%s", tag, sha)
		}

		logrus.Infof("saving addon manifest")
		newmeta.ReplaceImages = true
		if err := newmeta.Save("registry"); err != nil {
			return fmt.Errorf("failed to save metadata: %w", err)
		}

		logrus.Infof("rendering values for registry ha")
		err = newmeta.RenderValues("registry", "values-ha.tpl.yaml", "values-ha.yaml")
		if err != nil {
			return fmt.Errorf("failed to render values-ha: %w", err)
		}

		logrus.Infof("successfully updated registry addon")
		return nil
	},
}
