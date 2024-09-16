package main

import (
	"fmt"
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
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

		current := adminconsole.Metadata
		if current.Version == latest && !c.Bool("force") {
			logrus.Infof("admin console chart version is already up-to-date")
			return nil
		}

		upstream := "registry.replicated.com/library/admin-console"
		newmeta := release.AddonMetadata{
			Version:  latest,
			Location: fmt.Sprintf("oci://proxy.replicated.com/anonymous/%s", upstream),
			Images:   make(map[string]release.AddonImage),
		}

		values, err := release.GetValuesWithOriginalImages("adminconsole")
		if err != nil {
			return fmt.Errorf("unable to get openebs values: %v", err)
		}

		logrus.Infof("extracting images from chart")
		withproto := fmt.Sprintf("oci://%s", upstream)
		images, err := GetImagesFromOCIChart(withproto, "adminconsole", latest, values)
		if err != nil {
			return fmt.Errorf("failed to get images from admin console chart: %w", err)
		}

		metaImages, err := UpdateImages(c.Context, adminconsoleImageComponents, adminconsole.Metadata.Images, images)
		if err != nil {
			return fmt.Errorf("failed to update images: %w", err)
		}
		newmeta.Images = metaImages

		logrus.Infof("saving addon manifest")
		if err := newmeta.Save("adminconsole"); err != nil {
			return fmt.Errorf("failed to save admin console metadata: %w", err)
		}

		logrus.Infof("admin console addon updated")
		return nil
	},
}
