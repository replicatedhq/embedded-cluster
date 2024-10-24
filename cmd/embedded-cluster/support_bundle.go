package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
)

func supportBundleCommand() *cli.Command {
	return &cli.Command{
		Name:  "support-bundle",
		Usage: fmt.Sprintf("Generate a %s support bundle", defaults.BinaryName()),
		Before: func(c *cli.Context) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("support-bundle command must be run as root")
			}
			return nil
		},
		Action: func(c *cli.Context) error {
			provider := discoverBestProvider(c.Context)
			os.Setenv("TMPDIR", provider.EmbeddedClusterTmpSubDir())

			supportBundle := provider.PathToEmbeddedClusterBinary("kubectl-support_bundle")
			if _, err := os.Stat(supportBundle); err != nil {
				logrus.Errorf("Support bundle binary not found. The support-bundle command can only be run after an 'install' attempt.")
				return ErrNothingElseToAdd
			}

			hostSupportBundle := provider.PathToEmbeddedClusterSupportFile("host-support-bundle.yaml")
			if _, err := os.Stat(hostSupportBundle); err != nil {
				return fmt.Errorf("unable to find host support bundle: %w", err)
			}

			kubeConfig := provider.PathToKubeConfig()
			var env map[string]string
			if _, err := os.Stat(kubeConfig); err == nil {
				// if we have a kubeconfig, use it.
				env = map[string]string{"KUBECONFIG": kubeConfig}
			}

			spin := spinner.Start()
			spin.Infof("Collecting support bundle (this may take a while)")

			stdout := bytes.NewBuffer(nil)
			stderr := bytes.NewBuffer(nil)
			if err := helpers.RunCommandWithOptions(
				helpers.RunCommandOptions{
					Writer:       stdout,
					ErrWriter:    stderr,
					LogOnSuccess: true,
					Env:          env,
				},
				supportBundle,
				"--interactive=false",
				"--load-cluster-specs",
				hostSupportBundle,
			); err != nil {
				spin.Infof("Failed to collect support bundle")
				spin.CloseWithError()
				io.Copy(os.Stdout, stdout)
				io.Copy(os.Stderr, stderr)
				return ErrNothingElseToAdd
			}

			spin.Infof("Support bundle collected!")
			spin.Close()

			printSupportBundlePath(stdout)
			return nil
		},
	}
}

// printSupportBundlePath attempts to parse the support bundle command output
// and find the location where the tgz was written. support bundle output isn't
// always a json so we can't rely on unmarshaling it. depending on the kinds of
// errors returned during the collection the output may differ quite a bunch.
// in order to find where the support bundle was stored we look for a line
// containing "archivePath", if we find it we attempt to parse it. if we can't
// find a match or failed to parse it we simply don't print anything. XXX this
// should be addressed upstream, we should be able to cleanly parse the output
// as a json (i.e. errors should be printed into stderr).
func printSupportBundlePath(stdout io.Reader) {
	pwd, err := os.Getwd()
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, `"archivePath":`) {
			continue
		}

		components := strings.Split(line, `"`)
		if len(components) < 4 {
			return
		}

		fname := components[3]
		if !strings.HasPrefix(fname, "support-bundle-") {
			return
		}

		logrus.Infof("Support bundle written to %s", filepath.Join(pwd, fname))
	}
}
