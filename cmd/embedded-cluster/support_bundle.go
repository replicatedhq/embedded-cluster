package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/creack/pty"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
)

func supportBundleCommand() *cli.Command {
	runtimeConfig := ecv1beta1.GetDefaultRuntimeConfig()

	return &cli.Command{
		Name:  "support-bundle",
		Usage: fmt.Sprintf("Generate a %s support bundle", defaults.BinaryName()),
		Flags: []cli.Flag{
			getDataDirFlag(runtimeConfig),
		},
		Before: func(c *cli.Context) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("support-bundle command must be run as root")
			}
			return nil
		},
		Action: func(c *cli.Context) error {
			provider := discoverBestProvider(c.Context, runtimeConfig)
			os.Setenv("TMPDIR", provider.EmbeddedClusterTmpSubDir())

			supportBundle := provider.PathToEmbeddedClusterBinary("kubectl-support_bundle")
			if _, err := os.Stat(supportBundle); err != nil {
				return fmt.Errorf("unable to find support bundle binary")
			}

			kubeConfig := provider.PathToKubeConfig()
			hostSupportBundle := provider.PathToEmbeddedClusterSupportFile("host-support-bundle.yaml")
			command := exec.Command(supportBundle, "--load-cluster-specs", hostSupportBundle)
			command.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeConfig))

			// get the current working directory
			var err error
			command.Dir, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("unable to get current working directory: %w", err)
			}

			commandPTY, err := pty.Start(command)
			if err != nil {
				return fmt.Errorf("unable to start shell: %w", err)
			}

			sigch := make(chan os.Signal, 1)
			signal.Notify(sigch, syscall.SIGWINCH)
			go handleResize(sigch, commandPTY)
			sigch <- syscall.SIGWINCH
			state, err := term.MakeRaw(int(os.Stdin.Fd()))
			if err != nil {
				return fmt.Errorf("unable to make raw terminal: %w", err)
			}

			defer func() {
				signal.Stop(sigch)
				close(sigch)
				fd := int(os.Stdin.Fd())
				_ = term.Restore(fd, state)
			}()

			go func() { _, _ = io.Copy(commandPTY, os.Stdin) }()
			go func() { _, _ = io.Copy(os.Stdout, commandPTY) }()
			return command.Wait()
		},
	}
}
