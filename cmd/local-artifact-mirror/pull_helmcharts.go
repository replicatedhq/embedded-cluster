package main

import (
	"fmt"
	"path/filepath"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/tgzutils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// helmChartsCommand pulls helm charts from the registry running in the cluster and
// stores them locally. This command is used during cluster upgrades when we want to
// fetch the most up to date helm charts. Helm charts are stored in a tarball file
// in the default location.
func PullHelmChartsCmd(cli *CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "helmcharts INSTALLATION",
		Short: "Pull Helm chart artifacts for an airgap installation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			kcli, err := cli.KCLIGetter()
			if err != nil {
				return fmt.Errorf("unable to create kube client: %w", err)
			}

			in, err := fetchAndValidateInstallation(ctx, kcli, args[0])
			if err != nil {
				return err
			}

			if !in.Spec.AirGap {
				return fmt.Errorf("pulling helm charts is not supported for online installations")
			}

			from := in.Spec.Artifacts.HelmCharts
			logrus.Infof("fetching helm charts artifact from %s", from)
			location, err := cli.PullArtifact(ctx, kcli, from)
			if err != nil {
				return fmt.Errorf("unable to fetch artifact: %w", err)
			}
			defer func() {
				logrus.Infof("removing temporary directory %s", location)
				helpers.RemoveAll(location)
			}()

			dst := cli.RC.EmbeddedClusterChartsSubDir()
			src := filepath.Join(location, HelmChartsArtifactName)
			logrus.Infof("uncompressing %s", src)
			if err := tgzutils.Decompress(src, dst); err != nil {
				return fmt.Errorf("unable to uncompress helm charts: %w", err)
			}

			logrus.Infof("helm charts materialized under %s", dst)
			return nil
		},
	}

	return cmd
}
