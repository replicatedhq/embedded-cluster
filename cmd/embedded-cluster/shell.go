package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/creack/pty"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
)

const welcome = `
    __4___
 _  \ \ \ \   Welcome to %s debug shell.
<'\ /_/_/_/   This terminal is now configured to access your cluster.
 ((____!___/) Type 'exit' (or CTRL+d) to exit.
  \0\0\0\0\/  Happy hacking.
 ~~~~~~~~~~~
`

// handleResize is a helper function to handle pty resizes.
func handleResize(ch chan os.Signal, tty *os.File) {
	for range ch {
		if err := pty.InheritSize(os.Stdin, tty); err != nil {
			logrus.Errorf("unable to resize pty: %v", err)
		}
	}
}

var shellCommand = &cli.Command{
	Name:  "shell",
	Usage: "Start a shell with access to the running cluster",
	Before: func(c *cli.Context) error {
		if os.Getuid() != 0 {
			return fmt.Errorf("shell command must be run as root")
		}
		return nil
	},
	Action: func(c *cli.Context) error {
		cfgpath := defaults.PathToConfig("kubeconfig")
		if _, err := os.Stat(cfgpath); err != nil {
			return fmt.Errorf("kubeconfig not found at %s", cfgpath)
		}

		shpath := os.Getenv("SHELL")
		if shpath == "" {
			shpath = "/bin/bash"
		}

		fmt.Printf(welcome, defaults.BinaryName())
		shell := exec.Command(shpath)
		shell.Env = os.Environ()
		shell.Dir = defaults.EmbeddedClusterBinsSubDir()
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

		kcpath := defaults.PathToConfig("kubeconfig")
		config := fmt.Sprintf("export KUBECONFIG=%q\n", kcpath)
		_, _ = shellpty.WriteString(config)
		_, _ = io.CopyN(io.Discard, shellpty, int64(len(config)+1))

		config = `alias kubectl="/usr/local/bin/k0s kubectl\n"`
		_, _ = shellpty.WriteString(config)
		_, _ = io.CopyN(io.Discard, shellpty, int64(len(config)+1))

		bindir := defaults.EmbeddedClusterBinsSubDir()
		config = fmt.Sprintf("export PATH=\"$PATH:%s\"\n", bindir)
		_, _ = shellpty.WriteString(config)
		_, _ = io.CopyN(io.Discard, shellpty, int64(len(config)+1))

		// if /etc/bash_completion is present enable kubectl auto completion.
		if _, err := os.Stat("/etc/bash_completion"); err == nil {
			config = "source <(kubectl completion $(basename ${SHELL}))\n"
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
