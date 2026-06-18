package hostutils

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Masterminds/semver/v3"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/sirupsen/logrus"
)

// registryConfigTemplateV2 skips TLS verification for the airgap registry on
// containerd 1.7.x (k0s 1.34/1.35), using the legacy io.containerd.grpc.v1.cri path.
// TODO(k0s-1.36-oldest): drop the v2 templates and useContainerdV3Schema.
const registryConfigTemplateV2 = `
[plugins."io.containerd.grpc.v1.cri".registry]
  [plugins."io.containerd.grpc.v1.cri".registry.configs]
    [plugins."io.containerd.grpc.v1.cri".registry.configs."%s".tls]
      insecure_skip_verify = true
`

// registryConfigTemplateV3 is the containerd 2.x (k0s 1.36+) drop-in. The .tls
// sub-block was removed in 2.x, so TLS settings move to a hosts.toml under
// config_path (see hostsTomlTemplateV3); this drop-in only sets config_path.
const registryConfigTemplateV3 = `version = 3

[plugins."io.containerd.cri.v1.images".registry]
  config_path = "%s"
`

// hostsTomlTemplateV3 carries skip_verify for the airgap registry on containerd
// 2.x. Both %s placeholders are the registry host[:port].
const hostsTomlTemplateV3 = `server = "https://%s"

[host."https://%s"]
  skip_verify = true
`

// useContainerdV3Schema reports whether the embedded k0s ships containerd 2.x
// (k0s 1.36+), which needs the v3 drop-in schema. Falls back to v2 if the
// version is unset (e.g. "0.0.0" in tests) or malformed.
func useContainerdV3Schema() bool {
	sv, err := semver.NewVersion(versions.K0sVersion)
	if err != nil {
		return false
	}
	return sv.Major() == 1 && sv.Minor() >= 36
}

// AddInsecureRegistry adds a registry to the list of registries that
// are allowed to be accessed over HTTPS without verifying the certificate.
// The drop-in schema depends on the embedded k0s/containerd version.
func (h *HostUtils) AddInsecureRegistry(registry string) error {
	parentDir := runtimeconfig.K0sContainerdConfigPath
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to ensure containerd directory exists: %w", err)
	}

	if useContainerdV3Schema() {
		return h.addInsecureRegistryV3(registry)
	}

	contents := fmt.Sprintf(registryConfigTemplateV2, registry)
	if err := os.WriteFile(filepath.Join(parentDir, "embedded-registry.toml"), []byte(contents), 0644); err != nil {
		return fmt.Errorf("failed to write embedded-registry.toml: %w", err)
	}
	return nil
}

// addInsecureRegistryV3 writes the containerd 2.x (k0s 1.36+) configuration: a
// config_path drop-in plus a hosts.toml carrying skip_verify for the registry.
func (h *HostUtils) addInsecureRegistryV3(registry string) error {
	dropIn := fmt.Sprintf(registryConfigTemplateV3, runtimeconfig.K0sContainerdCertsDir)
	if err := os.WriteFile(filepath.Join(runtimeconfig.K0sContainerdConfigPath, "embedded-registry.toml"), []byte(dropIn), 0644); err != nil {
		return fmt.Errorf("failed to write embedded-registry.toml: %w", err)
	}

	hostDir := filepath.Join(runtimeconfig.K0sContainerdCertsDir, registry)
	if err := os.MkdirAll(hostDir, 0755); err != nil {
		return fmt.Errorf("failed to ensure containerd certs.d directory exists: %w", err)
	}
	hostsToml := fmt.Sprintf(hostsTomlTemplateV3, registry, registry)
	if err := os.WriteFile(filepath.Join(hostDir, "hosts.toml"), []byte(hostsToml), 0644); err != nil {
		return fmt.Errorf("failed to write hosts.toml: %w", err)
	}
	return nil
}

// v2RegistryHostRegex extracts the registry host from a legacy (v2) embedded-registry.toml,
// matching the configs."<host>".tls line written by registryConfigTemplateV2.
var v2RegistryHostRegex = regexp.MustCompile(`io\.containerd\.grpc\.v1\.cri"\.registry\.configs\."([^"]+)"\.tls`)

// MigrateContainerdConfigToV3 rewrites a stale v2 embedded-registry.toml to the
// v3 schema before k0s 1.36 starts (which would otherwise reject it). Run per
// node during an airgap upgrade. Idempotent; no-op for k0s < 1.36 or if the
// drop-in is absent or already v3.
func (h *HostUtils) MigrateContainerdConfigToV3() error {
	if !useContainerdV3Schema() {
		return nil
	}

	path := filepath.Join(runtimeconfig.K0sContainerdConfigPath, "embedded-registry.toml")
	contents, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read embedded-registry.toml: %w", err)
	}

	match := v2RegistryHostRegex.FindStringSubmatch(string(contents))
	if match == nil {
		// Not a legacy v2 drop-in (already v3 or unrecognized); leave it alone.
		return nil
	}
	registry := match[1]

	logrus.Infof("migrating containerd registry config for %s to v3 schema", registry)
	if err := h.addInsecureRegistryV3(registry); err != nil {
		return fmt.Errorf("failed to migrate containerd registry config to v3: %w", err)
	}
	return nil
}
