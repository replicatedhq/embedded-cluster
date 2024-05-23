package airgap

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
)

const registryConfigTemplate = `
[plugins."io.containerd.grpc.v1.cri".registry]
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
    [plugins."io.containerd.grpc.v1.cri".registry.mirrors."%[1]s"]
      endpoint = ["http://%[1]s"]
  [plugins."io.containerd.grpc.v1.cri".registry.configs]
    [plugins."io.containerd.grpc.v1.cri".registry.configs."%[1]s".tls]
      insecure_skip_verify = true
`

// RenderContainerdRegistryConfig returns the contents of the containerd configuration
// allowing insecure access to the embedded cluster internal registry.
func RenderContainerdRegistryConfig(registry string) string {
	return fmt.Sprintf(registryConfigTemplate, registry)
}

// AddInsecureRegistry adds a registry to the list of registries that
// are allowed to be accessed over HTTP.
func AddInsecureRegistry(registry string) error {
	parentDir := defaults.PathToK0sContainerdConfig()
	contents := RenderContainerdRegistryConfig(registry)

	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to ensure containerd directory exists: %w", err)
	}

	err := os.WriteFile(filepath.Join(parentDir, "embedded-registry.toml"), []byte(contents), 0644)
	if err != nil {
		return fmt.Errorf("failed to write embedded-registry.toml: %w", err)
	}

	return nil
}
