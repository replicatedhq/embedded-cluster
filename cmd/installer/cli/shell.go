package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/creack/pty"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
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

func ShellCmd(ctx context.Context, appTitle string) *cobra.Command {
	var rc runtimeconfig.RuntimeConfig
	var command string

	cmd := &cobra.Command{
		Use:   "shell",
		Short: fmt.Sprintf("Start a shell with access to the %s cluster", appTitle),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip root check if dryrun mode is enabled
			if !dryrun.Enabled() && os.Getuid() != 0 {
				return fmt.Errorf("shell command must be run as root")
			}

			rc = rcutil.InitBestRuntimeConfig(cmd.Context())
			os.Setenv("TMPDIR", rc.EmbeddedClusterTmpSubDir())

			if _, err := os.Stat(rc.PathToKubeConfig()); err != nil {
				return fmt.Errorf("kubeconfig not found at %s", rc.PathToKubeConfig())
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			shpath := os.Getenv("SHELL")
			if shpath == "" {
				shpath = "/bin/bash"
			}

			// Command execution mode
			if command != "" {
				return executeCommand(shpath, command, rc)
			}

			// Interactive shell mode
			return openInteractiveShell(shpath, rc)
		},
	}

	cmd.Flags().StringVarP(&command, "command", "c", "", "Command to execute in the shell environment instead of opening an interactive shell")

	return cmd
}

// executeCommand executes a command in the shell with the embedded cluster environment configured.
func executeCommand(shpath string, command string, rc runtimeconfig.RuntimeConfig) error {
	// Build the command with environment setup
	shell := exec.Command(shpath, "-c", command)

	// Set environment variables
	shell.Env = os.Environ()
	kcpath := rc.PathToKubeConfig()
	shell.Env = append(shell.Env, fmt.Sprintf("KUBECONFIG=%s", kcpath))
	bindir := rc.EmbeddedClusterBinsSubDir()
	shell.Env = append(shell.Env, fmt.Sprintf("PATH=%s:%s", os.Getenv("PATH"), bindir))

	// Set working directory
	var err error
	shell.Dir, err = os.Getwd()
	if err != nil {
		return fmt.Errorf("unable to get current working directory: %w", err)
	}

	// Connect stdio
	shell.Stdin = os.Stdin
	shell.Stdout = os.Stdout
	shell.Stderr = os.Stderr

	// Execute and return exit code
	if err := shell.Run(); err != nil {
		// Preserve exit code from the command
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}

	return nil
}

// openInteractiveShell opens an interactive shell with the embedded cluster environment configured.
func openInteractiveShell(shpath string, rc runtimeconfig.RuntimeConfig) error {
	fmt.Printf(welcome, runtimeconfig.AppSlug())
	shell := exec.Command(shpath)
	shell.Env = os.Environ()

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

	kcpath := rc.PathToKubeConfig()
	config := fmt.Sprintf("export KUBECONFIG=%q\n", kcpath)
	_, _ = shellpty.WriteString(config)
	_, _ = io.CopyN(io.Discard, shellpty, int64(len(config)+1))

	bindir := rc.EmbeddedClusterBinsSubDir()
	config = fmt.Sprintf("export PATH=\"$PATH:%s\"\n", bindir)
	_, _ = shellpty.WriteString(config)
	_, _ = io.CopyN(io.Discard, shellpty, int64(len(config)+1))

	config = "export TROUBLESHOOT_AUTO_UPDATE=false\nPREFLIGHT_AUTO_UPDATE=false\n"
	_, _ = shellpty.WriteString(config)
	_, _ = io.CopyN(io.Discard, shellpty, int64(len(config)+1))

	// if /etc/bash_completion is present enable kubectl auto completion.
	if _, err := os.Stat("/etc/bash_completion"); err == nil {
		config = fmt.Sprintf("source <(k0s completion %s)\n", filepath.Base(shpath))
		_, _ = shellpty.WriteString(config)
		_, _ = io.CopyN(io.Discard, shellpty, int64(len(config)+1))

		comppath := rc.PathToEmbeddedClusterBinary("kubectl_completion_bash.sh")
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
}

// handleResize is a helper function to handle pty resizes.
func handleResize(ch chan os.Signal, tty *os.File) {
	for range ch {
		if err := pty.InheritSize(os.Stdin, tty); err != nil {
			logrus.Errorf("unable to resize pty: %v", err)
		}
	}
}
