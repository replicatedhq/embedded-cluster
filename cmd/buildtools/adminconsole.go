package main

import (
	"fmt"
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var adminconsoleImageComponents = map[string]addonComponent{
	"docker.io/kotsadm/kotsadm": {
		name:             "kotsadm",
		useUpstreamImage: true,
	},
	"docker.io/kotsadm/kotsadm-migrations": {
		name:             "kotsadm-migrations",
		useUpstreamImage: true,
	},
	"docker.io/kotsadm/kurl-proxy": {
		name:             "kurl-proxy",
		useUpstreamImage: true,
	},
	"docker.io/kotsadm/rqlite": {
		name:             "rqlite",
		useUpstreamImage: true,
	},
}

var updateAdminConsoleAddonCommand = &cli.Command{
	Name:      "adminconsole",
	Usage:     "Updates the Admin Console addon",
	UsageText: environmentUsageText,
	Action: func(c *cli.Context) error {
		logrus.Infof("updating admin console addon")

		logrus.Infof("getting admin console latest tag")
		latest, err := GetLatestGitHubTag(c.Context, "replicatedhq", "kots-helm")
		if err != nil {
			return fmt.Errorf("failed to get admin console latest tag: %w", err)
		}
		logrus.Infof("latest tag found: %s", latest)
		latest = strings.TrimPrefix(latest, "v")

		upstream := "registry.replicated.com/library/admin-console"
		chartURL := getChartURL(upstream)

		newmeta := release.AddonMetadata{
			Version:  latest,
			Location: chartURL,
			Images:   make(map[string]release.AddonImage),
		}

		values, err := release.GetValuesWithOriginalImages("adminconsole")
		if err != nil {
			return fmt.Errorf("unable to get openebs values: %v", err)
		}

		logrus.Infof("extracting images from chart")
		images, err := GetImagesFromOCIChart(chartURL, "adminconsole", latest, values)
		if err != nil {
			return fmt.Errorf("failed to get images from admin console chart: %w", err)
		}

		for _, image := range images {
			component, ok := adminconsoleImageComponents[RemoveTagFromImage(image)]
			if !ok {
				return fmt.Errorf("no component found for image %s", image)
			}
			repo, tag, err := component.resolveImageRepoAndTag(c.Context, image)
			if err != nil {
				return fmt.Errorf("failed to resolve image and tag for %s: %w", image, err)
			}
			newmeta.Images[component.name] = release.AddonImage{
				Registry: getImageRegistry(),
				Repo:     repo,
				Tag:      tag,
			}
		}

		logrus.Infof("saving addon manifest")
		newmeta.ReplaceImages = true
		if err := newmeta.Save("adminconsole"); err != nil {
			return fmt.Errorf("failed to save admin console metadata: %w", err)
		}

		logrus.Infof("admin console addon updated")
		return nil
	},
}
