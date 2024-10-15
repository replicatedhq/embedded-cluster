package helpers

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"

	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/sirupsen/logrus"
)

type RunCommandOptions struct {
	// Stdout is an additional io.Stdout to write the stdout of the command to.
	Stdout io.Writer
	// Stderr is an additional io.Stderr to write the stderr of the command to.
	Stderr io.Writer
	// Env is a map of additional environment variables to set for the command.
	Env map[string]string
}

// RunCommandWithOptions runs a the provided command with the options specified.
func RunCommandWithOptions(opts RunCommandOptions, bin string, args ...string) error {
	if dryrun.IsDryRun() {
		dryrun.RecordCommand(bin, args, opts.Env)
		return nil
	}

	fullcmd := append([]string{bin}, args...)
	logrus.Debugf("running command: %v", fullcmd)

	stderr := bytes.NewBuffer(nil)
	stdout := bytes.NewBuffer(nil)
	cmd := exec.Command(bin, args...)
	cmd.Stdout = stdout
	if opts.Stdout != nil {
		cmd.Stdout = io.MultiWriter(opts.Stdout, stdout)
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
		if stderr.String() != "" {
			return fmt.Errorf("%w: %s", err, stderr.String())
		}
		return err
	}
	return nil
}

// RunCommand spawns a command and capture its output. Outputs are logged using the
// logrus package and stdout is returned as a string.
func RunCommand(bin string, args ...string) (string, error) {
	stdout := bytes.NewBuffer(nil)
	if err := RunCommandWithOptions(RunCommandOptions{Stdout: stdout}, bin, args...); err != nil {
		return "", err
	}
	return stdout.String(), nil
}
