package installation

import (
	"fmt"
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	k0sconfig "github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

type ConfigStore interface {
	Read() (*types.InstallationConfig, error)
	Write(cfg types.InstallationConfig) error
}

var _ ConfigStore = &ConfigMemoryStore{}
var _ ConfigStore = &ConfigRuntimeConfigStore{}

type ConfigMemoryStore struct {
	mu  sync.RWMutex
	cfg *types.InstallationConfig
}

func NewConfigMemoryStore() *ConfigMemoryStore {
	return &ConfigMemoryStore{
		cfg: &types.InstallationConfig{},
	}
}

func (s *ConfigMemoryStore) Read() (*types.InstallationConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.cfg, nil
}

func (s *ConfigMemoryStore) Write(cfg types.InstallationConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = &cfg

	return nil
}

type ConfigRuntimeConfigStore struct {
	mu sync.RWMutex
}

func NewConfigRuntimeConfigStore() (*ConfigRuntimeConfigStore, error) {
	runtimeConfig, err := runtimeconfig.ReadFromDisk()
	if err != nil {
		return nil, fmt.Errorf("read runtime config from disk: %w", err)
	} else if runtimeConfig != nil {
		runtimeconfig.Set(runtimeConfig)
	}

	return &ConfigRuntimeConfigStore{}, nil
}

func (s *ConfigRuntimeConfigStore) Read() (*types.InstallationConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return runtimeConfigToConfig()
}

func (s *ConfigRuntimeConfigStore) Write(cfg types.InstallationConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := applyConfigToRuntimeConfig(cfg)
	if err != nil {
		return fmt.Errorf("apply config to runtime config: %w", err)
	}

	err = runtimeconfig.WriteToDisk()
	if err != nil {
		return fmt.Errorf("write runtime config to disk: %w", err)
	}

	return nil
}

func runtimeConfigToConfig() (*types.InstallationConfig, error) {
	cfg := &types.InstallationConfig{}

	cfg.AdminConsolePort = runtimeconfig.AdminConsolePort()
	cfg.AdminConsolePassword = runtimeconfig.AdminConsolePassword()

	cfg.DataDirectory = runtimeconfig.EmbeddedClusterHomeDirectory()
	cfg.HostCABundlePath = runtimeconfig.HostCABundlePath()
	cfg.LocalArtifactMirrorPort = runtimeconfig.LocalArtifactMirrorPort()

	proxySpec := runtimeconfig.ProxySpec()
	networkSpec := runtimeconfig.NetworkSpec()

	if proxySpec != nil {
		cfg.HTTPProxy = proxySpec.HTTPProxy
		cfg.HTTPSProxy = proxySpec.HTTPSProxy
		cfg.NoProxy = proxySpec.ProvidedNoProxy
	}

	if networkSpec != nil {
		cfg.NetworkInterface = networkSpec.NetworkInterface
		cfg.PodCIDR = networkSpec.PodCIDR
		cfg.ServiceCIDR = networkSpec.ServiceCIDR
		cfg.GlobalCIDR = networkSpec.GlobalCIDR
	}

	cfg.EndUserConfigOverrides = runtimeconfig.EndUserK0sConfigOverrides()

	return cfg, nil
}

func applyConfigToRuntimeConfig(config types.InstallationConfig) error {
	if config.DataDirectory != "" {
		runtimeconfig.SetDataDir(config.DataDirectory)
	}

	if config.HostCABundlePath != "" {
		runtimeconfig.SetHostCABundlePath(config.HostCABundlePath)
	}

	if config.LocalArtifactMirrorPort != 0 {
		runtimeconfig.SetLocalArtifactMirrorPort(config.LocalArtifactMirrorPort)
	}

	if config.AdminConsolePort != 0 {
		runtimeconfig.SetAdminConsolePort(config.AdminConsolePort)
	}

	if config.AdminConsolePassword != "" {
		runtimeconfig.SetAdminConsolePassword(config.AdminConsolePassword)
	}

	proxySpec, err := getProxySpecFromConfig(config)
	if err != nil {
		return fmt.Errorf("get proxy spec from config: %w", err)
	}
	runtimeconfig.SetProxySpec(proxySpec)

	networkSpec, err := getNetworkSpecFromConfig(config)
	if err != nil {
		return fmt.Errorf("get network spec from config: %w", err)
	}
	runtimeconfig.SetNetworkSpec(networkSpec)

	euOverrides, err := getEndUserK0sConfigOverridesFromConfig(config)
	if err != nil {
		return fmt.Errorf("get end user k0s config overrides from config: %w", err)
	}
	runtimeconfig.SetEndUserK0sConfigOverrides(euOverrides)

	return nil
}

func getProxySpecFromConfig(config types.InstallationConfig) (*ecv1beta1.ProxySpec, error) {
	if config.HTTPProxy == "" && config.HTTPSProxy == "" && config.NoProxy == "" {
		return nil, nil
	}

	proxySpec := ecv1beta1.ProxySpec{
		HTTPProxy:       config.HTTPProxy,
		HTTPSProxy:      config.HTTPSProxy,
		ProvidedNoProxy: config.NoProxy,
	}

	// Now that we have all no-proxy entries (from flags/env), merge in defaults
	noProxy, err := combineNoProxySuppliedValuesAndDefaults(config, proxySpec, nil)
	if err != nil {
		return nil, fmt.Errorf("combine no-proxy supplied values and defaults: %w", err)
	}
	proxySpec.NoProxy = noProxy

	return &proxySpec, nil
}

func getNetworkSpecFromConfig(config types.InstallationConfig) (*ecv1beta1.NetworkSpec, error) {
	nodePortRange, err := getNodePortRangeFromConfig(config)
	if err != nil {
		return nil, fmt.Errorf("get node port range: %w", err)
	}

	return &ecv1beta1.NetworkSpec{
		NetworkInterface: config.NetworkInterface,
		PodCIDR:          config.PodCIDR,
		ServiceCIDR:      config.ServiceCIDR,
		GlobalCIDR:       config.GlobalCIDR,
		NodePortRange:    nodePortRange,
	}, nil
}

func getNodePortRangeFromConfig(config types.InstallationConfig) (string, error) {
	cfg := k0sconfig.RenderK0sConfig("")

	embcfg := release.GetEmbeddedClusterConfig()
	if embcfg != nil {
		// Apply vendor k0s overrides
		vendorOverrides := embcfg.Spec.UnsupportedOverrides.K0s
		var err error
		cfg, err = k0sconfig.PatchK0sConfig(cfg, vendorOverrides, false)
		if err != nil {
			return "", fmt.Errorf("patch vendor overrides: %w", err)
		}
	}

	endUserOverrides, err := getEndUserK0sConfigOverridesFromConfig(config)
	if err != nil {
		return "", fmt.Errorf("get end user k0s config overrides from config: %w", err)
	}

	cfg, err = k0sconfig.PatchK0sConfig(cfg, endUserOverrides, false)
	if err != nil {
		return "", fmt.Errorf("patch end user overrides: %w", err)
	}

	if cfg.Spec.API != nil {
		if val, ok := cfg.Spec.API.ExtraArgs["service-node-port-range"]; ok {
			return val, nil
		}
	}

	return k0sconfig.DefaultServiceNodePortRange, nil
}

func getEndUserK0sConfigOverridesFromConfig(config types.InstallationConfig) (string, error) {
	if config.EndUserConfigOverrides == "" {
		return "", nil
	}

	eucfg, err := helpers.ParseEndUserConfig(config.EndUserConfigOverrides)
	if err != nil {
		return "", fmt.Errorf("parse end user config: %w", err)
	}
	if eucfg != nil {
		return eucfg.Spec.UnsupportedOverrides.K0s, nil
	}

	return "", nil
}
