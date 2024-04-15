package helpers

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"

	"github.com/sirupsen/logrus"
)

// RunCommandWithWriter runs a the provided command. The stdout of the command is
// written to the provider writer.
func RunCommandWithWriter(to io.Writer, bin string, args ...string) error {
	fullcmd := append([]string{bin}, args...)
	logrus.Debugf("running command: %v", fullcmd)

	stderr := bytes.NewBuffer(nil)
	stdout := bytes.NewBuffer(nil)
	cmd := exec.Command(bin, args...)
	cmd.Stdout = io.MultiWriter(to, stdout)
	cmd.Stderr = stderr
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
	if err := RunCommandWithWriter(stdout, bin, args...); err != nil {
		return "", err
	}
	return stdout.String(), nil
}
