package helpers

import (
	"bytes"
	"os/exec"

	"github.com/sirupsen/logrus"
)

// RunCommand spawns a command and capture its output. Outputs are logged using the
// logrus package and stdout is returned as a string.
func RunCommand(bin string, args ...string) (string, error) {
	fullcmd := append([]string{bin}, args...)
	logrus.Debugf("running command: %v", fullcmd)

	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	cmd := exec.Command(bin, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		logrus.Debugf("failed to run command:")
		logrus.Debugf("stdout: %s", stdout.String())
		logrus.Debugf("stderr: %s", stderr.String())
		return "", err
	}
	return stdout.String(), nil
}
