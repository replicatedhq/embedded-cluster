package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// binariesCommands pulls the binary artifact from the registry running in the cluster and stores
// it locally. This command is used during cluster upgrades when we want to fetch the most up to
// date binaries. The binaries are stored in the /usr/local/bin directory and they overwrite the
// existing binaries.
func PullBinariesCmd(cli *CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "binaries INSTALLATION",
		Short: "Pull binaries artifacts for an airgap installation",
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

			from := in.Spec.Artifacts.EmbeddedClusterBinary
			logrus.Infof("fetching embedded cluster binary artifact from %s", from)
			location, err := cli.PullArtifact(ctx, kcli, from)
			if err != nil {
				return fmt.Errorf("unable to fetch artifact: %w", err)
			}
			defer func() {
				logrus.Infof("removing temporary directory %s", location)
				os.RemoveAll(location)
			}()
			bin := filepath.Join(location, EmbeddedClusterBinaryArtifactName)
			namedBin := filepath.Join(location, in.Spec.BinaryName)
			if err := os.Rename(bin, namedBin); err != nil {
				return fmt.Errorf("unable to rename binary: %w", err)
			}

			if err := os.Chmod(namedBin, 0755); err != nil {
				return fmt.Errorf("unable to change permissions on %s: %w", bin, err)
			}

			out := bytes.NewBuffer(nil)

			materializeCmdArgs := []string{"materialize", "--data-dir", runtimeconfig.EmbeddedClusterHomeDirectory()}
			materializeCmd := exec.Command(namedBin, materializeCmdArgs...)
			materializeCmd.Stdout = out
			materializeCmd.Stderr = out

			logrus.Infof("running command: %s with args: %v", namedBin, materializeCmdArgs)
			if err := materializeCmd.Run(); err != nil {
				logrus.Error(out.String())
				return err
			}

			logrus.Infof("embedded cluster binaries materialized")

			return nil
		},
	}

	return cmd
}
