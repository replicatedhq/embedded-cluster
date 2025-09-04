package helpers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/sirupsen/logrus"
)

// RunCommandWithOptions runs a the provided command with the options specified.
func (h *Helpers) RunCommandWithOptions(opts RunCommandOptions, bin string, args ...string) error {
	fullcmd := append([]string{bin}, args...)
	logrus.Debugf("running command: %v", fullcmd)

	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}

	stderr := bytes.NewBuffer(nil)
	stdout := bytes.NewBuffer(nil)
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdout = stdout
	if opts.Stdout != nil {
		cmd.Stdout = io.MultiWriter(opts.Stdout, stdout)
	}
	if opts.Stdin != nil {
		cmd.Stdin = opts.Stdin
	}
	cmd.Stderr = stderr
	if opts.Stderr != nil {
		cmd.Stderr = io.MultiWriter(opts.Stderr, stderr)
	}
	cmdEnv := cmd.Environ()
	for k, v := range opts.Env {
		cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = cmdEnv
	if err := cmd.Run(); err != nil {
		logrus.Debugf("failed to run command:")
		logrus.Debugf("stdout: %s", stdout.String())
		logrus.Debugf("stderr: %s", stderr.String())

		// Check if it's a context error and return it instead
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if stderr.String() != "" {
			return fmt.Errorf("%w: %s", err, stderr.String())
		}
		return err
	}

	if !opts.LogOnSuccess {
		return nil
	}

	logrus.Debugf("command succeeded:")
	logrus.Debugf("stdout: %s", stdout.String())
	logrus.Debugf("stderr: %s", stderr.String())
	return nil
}

// RunCommand spawns a command and capture its output. Outputs are logged using the
// logrus package and stdout is returned as a string.
func (h *Helpers) RunCommand(bin string, args ...string) (string, error) {
	stdout := bytes.NewBuffer(nil)
	if err := h.RunCommandWithOptions(RunCommandOptions{Stdout: stdout}, bin, args...); err != nil {
		return "", err
	}
	return stdout.String(), nil
}
