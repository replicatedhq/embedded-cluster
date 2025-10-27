package hostutils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

const registryConfigTemplate = `
[plugins."io.containerd.grpc.v1.cri".registry]
  [plugins."io.containerd.grpc.v1.cri".registry.configs]
    [plugins."io.containerd.grpc.v1.cri".registry.configs."%s".tls]
      insecure_skip_verify = true
`

// AddInsecureRegistry adds a registry to the list of registries that
// are allowed to be accessed over HTTP.
func (h *HostUtils) AddInsecureRegistry(registry string) error {
	parentDir := runtimeconfig.K0sContainerdConfigPath
	contents := fmt.Sprintf(registryConfigTemplate, registry)

	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to ensure containerd directory exists: %w", err)
	}

	err := helpers.WriteFile(filepath.Join(parentDir, "embedded-registry.toml"), []byte(contents), 0644)
	if err != nil {
		return fmt.Errorf("failed to write embedded-registry.toml: %w", err)
	}

	return nil
}
