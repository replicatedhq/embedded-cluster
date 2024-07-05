package main

import (
	"fmt"
	"os"
	"path/filepath"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/sirupsen/logrus"
)

// createSystemdUnitFiles links the k0s systemd unit file. this also creates a new
// systemd unit file for the local artifact mirror service.
func createSystemdUnitFiles(isWorker bool, proxy *ecv1beta1.ProxySpec) error {
	dst := systemdUnitFileName()
	if _, err := os.Lstat(dst); err == nil {
		if err := os.Remove(dst); err != nil {
			return err
		}
	}
	src := "/etc/systemd/system/k0scontroller.service"
	if isWorker {
		src = "/etc/systemd/system/k0sworker.service"
	}
	if proxy != nil {
		ensureProxyConfig(fmt.Sprintf("%s.d", src), proxy.HTTPProxy, proxy.HTTPSProxy, proxy.NoProxy)
	}
	logrus.Debugf("linking %q to %q", src, dst)
	if err := os.Symlink(src, dst); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	if _, err := helpers.RunCommand("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("unable to get reload systemctl daemon: %w", err)
	}
	return installAndEnableLocalArtifactMirror()
}

// ensureProxyConfig creates a new http-proxy.conf configuration file. The file is saved in the
// systemd directory (/etc/systemd/system/k0scontroller.service.d/).
func ensureProxyConfig(servicePath string, httpProxy string, httpsProxy string, noProxy string) error {
	// create the directory
	if err := os.MkdirAll(servicePath, 0755); err != nil {
		return fmt.Errorf("unable to create directory: %w", err)
	}

	// create the file
	fp, err := os.OpenFile(filepath.Join(servicePath, "http-proxy.conf"), os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("unable to create proxy file: %w", err)
	}
	defer fp.Close()

	// write the file
	if _, err := fp.WriteString(fmt.Sprintf(`[Service]
Environment="HTTP_PROXY=%s"
Environment="HTTPS_PROXY=%s"
Environment="NO_PROXY=%s"`,
		httpProxy, httpsProxy, noProxy)); err != nil {
		return fmt.Errorf("unable to write proxy file: %w", err)
	}

	return nil
}
