package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/creack/pty"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	rcutil "github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const welcome = `
    __4___
 _  \ \ \ \   Welcome to %s debug shell.
<'\ /_/_/_/   This terminal is now configured to access your cluster.
 ((____!___/) Type 'exit' (or Ctrl+D) to exit.
  \0\0\0\0\/
 ~~~~~~~~~~~
`

func ShellCmd(ctx context.Context, name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shell",
		Short: "Start a shell with access to the cluster",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("shell command must be run as root")
			}

			rcutil.InitBestRuntimeConfig(cmd.Context())
			os.Setenv("TMPDIR", runtimeconfig.EmbeddedClusterTmpSubDir())

			if _, err := os.Stat(runtimeconfig.PathToKubeConfig()); err != nil {
				return fmt.Errorf("kubeconfig not found at %s", runtimeconfig.PathToKubeConfig())
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			shpath := os.Getenv("SHELL")
			if shpath == "" {
				shpath = "/bin/bash"
			}

			fmt.Printf(welcome, runtimeconfig.BinaryName())
			shell := exec.Command(shpath)
			shell.Env = os.Environ()

			// get the current working directory
			var err error
			shell.Dir, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("unable to get current working directory: %w", err)
			}

			shellpty, err := pty.Start(shell)
			if err != nil {
				return fmt.Errorf("unable to start shell: %w", err)
			}

			sigch := make(chan os.Signal, 1)
			signal.Notify(sigch, syscall.SIGWINCH)
			go handleResize(sigch, shellpty)
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

			kcpath := runtimeconfig.PathToKubeConfig()
			config := fmt.Sprintf("export KUBECONFIG=%q\n", kcpath)
			_, _ = shellpty.WriteString(config)
			_, _ = io.CopyN(io.Discard, shellpty, int64(len(config)+1))

			bindir := runtimeconfig.EmbeddedClusterBinsSubDir()
			config = fmt.Sprintf("export PATH=\"$PATH:%s\"\n", bindir)
			_, _ = shellpty.WriteString(config)
			_, _ = io.CopyN(io.Discard, shellpty, int64(len(config)+1))

			// if /etc/bash_completion is present enable kubectl auto completion.
			if _, err := os.Stat("/etc/bash_completion"); err == nil {
				config = fmt.Sprintf("source <(k0s completion %s)\n", filepath.Base(shpath))
				_, _ = shellpty.WriteString(config)
				_, _ = io.CopyN(io.Discard, shellpty, int64(len(config)+1))

				comppath := runtimeconfig.PathToEmbeddedClusterBinary("kubectl_completion_bash.sh")
				config = fmt.Sprintf("source <(cat %s)\n", comppath)
				_, _ = shellpty.WriteString(config)
				_, _ = io.CopyN(io.Discard, shellpty, int64(len(config)+1))

				config = "source /etc/bash_completion\n"
				_, _ = shellpty.WriteString(config)
				_, _ = io.CopyN(io.Discard, shellpty, int64(len(config)+1))
			}

			go func() { _, _ = io.Copy(shellpty, os.Stdin) }()
			go func() { _, _ = io.Copy(os.Stdout, shellpty) }()
			_ = shell.Wait()
			return nil
		},
	}

	return cmd
}

// handleResize is a helper function to handle pty resizes.
func handleResize(ch chan os.Signal, tty *os.File) {
	for range ch {
		if err := pty.InheritSize(os.Stdin, tty); err != nil {
			logrus.Errorf("unable to resize pty: %v", err)
		}
	}
}
