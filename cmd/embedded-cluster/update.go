package main

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
)

var updateCommand = &cli.Command{
	Name:   "update",
	Usage:  fmt.Sprintf("Update %s", binName),
	Hidden: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "airgap-bundle",
			Usage:    "Path to the airgap bundle",
			Required: true,
		},
	},
	Before: func(c *cli.Context) error {
		if os.Getuid() != 0 {
			return fmt.Errorf("update command must be run as root")
		}
		os.Setenv("KUBECONFIG", defaults.PathToKubeConfig())
		return nil
	},
	Action: func(c *cli.Context) error {
		if c.String("airgap-bundle") != "" {
			logrus.Debugf("checking airgap bundle matches binary")
			if err := checkAirgapMatches(c); err != nil {
				return err // we want the user to see the error message without a prefix
			}
		}

		rel, err := release.GetChannelRelease()
		if err != nil {
			return fmt.Errorf("unable to get channel release: %w", err)
		}
		if rel == nil {
			return fmt.Errorf("no channel release found")
		}

		if c.String("airgap-bundle") != "" {
			kcli, err := kubeutils.KubeClient()
			if err != nil {
				return fmt.Errorf("failed to create kube client: %w", err)
			}

			creds, err := adminconsole.GetEmbeddedRegistryCredentials(c.Context, kcli)
			if err != nil {
				return fmt.Errorf("failed to get embedded registry credentials: %w", err)
			}

			if err := kotscli.AdminConsolePushImages(kotscli.AdminConsolePushImagesOptions{
				AirgapBundle:     c.String("airgap-bundle"),
				RegistryHost:     fmt.Sprintf("%s/%s", creds.Hostname, rel.AppSlug),
				RegistryUsername: creds.Username,
				RegistryPassword: creds.Password,
			}); err != nil {
				return err
			}
			if err := kotscli.AirgapUpload(kotscli.AirgapUploadOptions{
				AppSlug:      rel.AppSlug,
				Namespace:    defaults.KotsadmNamespace,
				AirgapBundle: c.String("airgap-bundle"),
			}); err != nil {
				return err
			}
			return nil
		}

		if err := kotscli.UpstreamUpgrade(kotscli.UpstreamUpgradeOptions{
			AppSlug:   rel.AppSlug,
			Namespace: defaults.KotsadmNamespace,
		}); err != nil {
			return err
		}

		return nil
	},
}
