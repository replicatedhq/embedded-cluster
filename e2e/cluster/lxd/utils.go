package lxd

import (
	"bytes"
	"path/filepath"
	"strings"
)

type buffer struct {
	*bytes.Buffer
}

func (b *buffer) Close() error {
	return nil
}

func mergeMaps(maps ...map[string]string) map[string]string {
	merged := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			merged[k] = v
		}
	}
	return merged
}

func WithECShellEnv(dataDir string) map[string]string {
	return map[string]string{
		"KUBECONFIG": filepath.Join(dataDir, "k0s/pki/admin.conf"),
		"PATH":       filepath.Join(dataDir, "bin"),
	}
}

func WithMITMProxyEnv(nodeIPs []string) map[string]string {
	return map[string]string{
		"HTTP_PROXY":  HTTPMITMProxy,
		"HTTPS_PROXY": HTTPMITMProxy,
		"NO_PROXY":    strings.Join(nodeIPs, ","),
	}
}

func WithProxyEnv(nodeIPs []string) map[string]string {
	return map[string]string{
		"HTTP_PROXY":  HTTPProxy,
		"HTTPS_PROXY": HTTPProxy,
		"NO_PROXY":    strings.Join(nodeIPs, ","),
	}
}
