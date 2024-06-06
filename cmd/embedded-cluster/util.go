package main

import (
	"fmt"
	"os"
	"path/filepath"
)

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
