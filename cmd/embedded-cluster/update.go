package main

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
)

func updateCommand() *cli.Command {
	return &cli.Command{
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
			return nil
		},
		Action: func(c *cli.Context) error {
			provider, err := getProviderFromCluster(c.Context)
			if err != nil {
				return err
			}
			os.Setenv("KUBECONFIG", provider.PathToKubeConfig())
			os.Setenv("TMPDIR", provider.EmbeddedClusterTmpSubDir())

			defer tryRemoveTmpDirContents(provider)

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

			if err := kotscli.AirgapUpdate(provider, kotscli.AirgapUpdateOptions{
				AppSlug:      rel.AppSlug,
				Namespace:    defaults.KotsadmNamespace,
				AirgapBundle: c.String("airgap-bundle"),
			}); err != nil {
				return err
			}

			return nil
		},
	}
}
