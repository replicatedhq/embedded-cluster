package main

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"helm.sh/helm/v3/pkg/repo"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
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
			ref := "registry.replicated.com/library/registry"
			constraints := mustParseSemverConstraints(latestPatchConstraint(opts.upstreamVersion))
			return getLatestImageNameAndTag(opts.ctx, ref, constraints)
		},
	},
}

var updateRegistryAddonCommand = &cli.Command{
	Name:      "registry",
	Usage:     "Updates the Registry addon",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating registry addon")

		hcli, err := NewHelm()
		if err != nil {
			return fmt.Errorf("failed to create helm client: %w", err)
		}
		defer hcli.Close()

		latest := "2.8.3"

		// TODO: unpin this
		// latest, err := LatestChartVersion(hcli, registryRepo, "docker-registry")
		// if err != nil {
		// 	return fmt.Errorf("unable to get the latest registry version: %v", err)
		// }
		// logrus.Printf("latest registry chart version: %s", latest)

		current := registry.Metadata
		if current.Version == latest && !c.Bool("force") {
			logrus.Infof("registry version is already up-to-date")
			return nil
		}

		logrus.Infof("mirroring registry chart version %s", latest)
		if err := MirrorChart(hcli, registryRepo, "docker-registry", latest); err != nil {
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
		images, err := helm.ExtractImagesFromChart(hcli, withproto, latest, values)
		if err != nil {
			return fmt.Errorf("failed to get images from chart: %w", err)
		}

		metaImages, err := UpdateImages(c.Context, registryImageComponents, registry.Metadata.Images, images, nil)
		if err != nil {
			return fmt.Errorf("failed to update images: %w", err)
		}
		newmeta.Images = metaImages

		logrus.Infof("saving addon manifest")
		if err := newmeta.Save("registry"); err != nil {
			return fmt.Errorf("failed to save metadata: %w", err)
		}

		logrus.Infof("successfully updated registry addon")
		return nil
	},
}
