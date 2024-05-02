package helpers

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/sirupsen/logrus"
)

type RunCommandWithWriterOptions struct {
	Env map[string]string
}

// RunCommandWithWriter runs a the provided command. The stdout of the command is
// written to the provider writer.
func RunCommandWithWriter(opts RunCommandWithWriterOptions, to io.Writer, bin string, args ...string) error {
	fullcmd := append([]string{bin}, args...)
	logrus.Debugf("running command: %v", fullcmd)

	stderr := bytes.NewBuffer(nil)
	stdout := bytes.NewBuffer(nil)
	cmd := exec.Command(bin, args...)
	cmd.Stdout = io.MultiWriter(to, stdout)
	cmd.Stderr = stderr
	cmd.Env = os.Environ()
	for k, v := range opts.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
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
	if err := RunCommandWithWriter(RunCommandWithWriterOptions{}, stdout, bin, args...); err != nil {
		return "", err
	}
	return stdout.String(), nil
}
