package main

import (
	"fmt"
	"os"

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

var registryImageComponents = map[string]addonComponent{
	"docker.io/library/registry": {
		name: "registry",
		getCustomImageName: func(opts addonComponentOptions) (string, error) {
			// TODO (@salah): build with apko once distribution is out of beta: https://github.com/wolfi-dev/os/blob/main/distribution.yaml
			return "docker.io/replicated/ec-registry:2.8.3-r0", nil
		},
	},
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
			Images:   make(map[string]release.AddonImage),
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

		for _, image := range images {
			component, ok := registryImageComponents[RemoveTagFromImage(image)]
			if !ok {
				return fmt.Errorf("no component found for image %s", image)
			}
			repo, tag, err := component.resolveImageRepoAndTag(c.Context, image)
			if err != nil {
				return fmt.Errorf("failed to resolve image and tag for %s: %w", image, err)
			}
			newmeta.Images[component.name] = release.AddonImage{
				Repo: repo,
				Tag: map[string]string{
					"amd64": tag,
					// TODO (@salah): automate updating the arm64 tag
					"arm64": registry.Metadata.Images[component.name].Tag["arm64"],
				},
			}
		}

		logrus.Infof("saving addon manifest")
		if err := newmeta.Save("registry"); err != nil {
			return fmt.Errorf("failed to save metadata: %w", err)
		}

		logrus.Infof("successfully updated registry addon")
		return nil
	},
}
