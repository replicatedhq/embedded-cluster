package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/sirupsen/logrus"
)

// ensureProxyConfig creates a new http-proxy.conf configuration file. The file is saved in the
// systemd directory (/etc/systemd/system/k0scontroller.service.d/).
func ensureProxyConfig(servicePath string, httpProxy string, httpsProxy string, noProxy string) error {
	// create the directory
	if err := os.MkdirAll(servicePath, 0755); err != nil {
		return fmt.Errorf("unable to create directory: %w", err)
	}

	// create and write the file
	content := fmt.Sprintf(`[Service]
Environment="HTTP_PROXY=%s"
Environment="HTTPS_PROXY=%s"
Environment="NO_PROXY=%s"`, httpProxy, httpsProxy, noProxy)

	err := os.WriteFile(filepath.Join(servicePath, "http-proxy.conf"), []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("unable to create and write proxy file: %w", err)
	}

	return nil
}

func tryRemoveTmpDirContents(provider *defaults.Provider) {
	err := helpers.RemoveAll(provider.EmbeddedClusterTmpSubDir())
	if err != nil {
		logrus.Debugf("failed to remove tmp dir contents: %v", err)
	}
}
