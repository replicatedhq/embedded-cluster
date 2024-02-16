package kubeutils

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// KubeClient returns a new kubernetes client.
func KubeClient() (client.Client, error) {
	k8slogger := zap.New(func(o *zap.Options) {
		o.DestWriter = io.Discard
	})

	stdout, err := runCommand(defaults.K0sBinaryPath(), "kubeconfig", "admin")
	if err != nil {
		return nil, fmt.Errorf("unable to generate kubeconfig: %w", err)
	}
	confDir, err := os.MkdirTemp("", "embedded-cluster-")
	if err != nil {
		return nil, fmt.Errorf("unable to create temp dir: %w", err)
	}
	//defer os.RemoveAll(confDir) // TODO remove this conf dir somewhere
	err = os.Setenv("KUBECONFIG", filepath.Join(confDir, "kubeconfig.yaml"))
	if err != nil {
		return nil, fmt.Errorf("unable to set KUBECONFIG: %w", err)
	}

	fp, err := os.OpenFile(filepath.Join(confDir, "kubeconfig.yaml"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("unable to open kubeconfig: %w", err)
	}
	defer fp.Close()
	if _, err := fp.WriteString(stdout); err != nil {
		return nil, fmt.Errorf("unable to write kubeconfig: %w", err)
	}

	log.SetLogger(k8slogger)
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to process kubernetes config: %w", err)
	}
	return client.New(cfg, client.Options{})
}

// runCommand spawns a command and capture its output. Outputs are logged using the
// logrus package and stdout is returned as a string.
func runCommand(bin string, args ...string) (string, error) {
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
