package models

import (
	"fmt"
	"sync"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

type InstallationConfigStore interface {
	Read() (*InstallationConfig, error)
	Write(cfg InstallationConfig) error
}

var _ InstallationConfigStore = &InstallationConfigMemoryStore{}
var _ InstallationConfigStore = &InstallationConfigRuntimeConfigStore{}

type InstallationConfigMemoryStore struct {
	mu  sync.RWMutex
	cfg *InstallationConfig
}

func NewInstallationConfigMemoryStore() *InstallationConfigMemoryStore {
	return &InstallationConfigMemoryStore{
		cfg: &InstallationConfig{},
	}
}

func (s *InstallationConfigMemoryStore) Read() (*InstallationConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.cfg, nil
}

func (s *InstallationConfigMemoryStore) Write(cfg InstallationConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = &cfg

	return nil
}

type InstallationConfigRuntimeConfigStore struct {
	mu sync.RWMutex
}

func NewInstallationConfigRuntimeConfigStore() *InstallationConfigRuntimeConfigStore {
	return &InstallationConfigRuntimeConfigStore{}
}

func (s *InstallationConfigRuntimeConfigStore) Read() (*InstallationConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return runtimeConfigToInstallationConfig()
}

func (s *InstallationConfigRuntimeConfigStore) Write(cfg InstallationConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := applyInstallationConfigToRuntimeConfig(cfg)
	if err != nil {
		return fmt.Errorf("apply config to runtime config: %w", err)
	}

	err = runtimeconfig.WriteToDisk()
	if err != nil {
		return fmt.Errorf("write runtime config to disk: %w", err)
	}

	return nil
}

func runtimeConfigToInstallationConfig() (*InstallationConfig, error) {
	cfg := &InstallationConfig{}

	cfg.AdminConsolePort = runtimeconfig.AdminConsolePort()
	cfg.AdminConsolePassword = runtimeconfig.AdminConsolePassword()

	cfg.DataDirectory = runtimeconfig.EmbeddedClusterHomeDirectory()
	cfg.HostCABundlePath = runtimeconfig.HostCABundlePath()
	cfg.LocalArtifactMirrorPort = runtimeconfig.LocalArtifactMirrorPort()

	proxySpec := runtimeconfig.ProxySpec()
	networkSpec := runtimeconfig.NetworkSpec()

	cfg.HTTPProxy = proxySpec.HTTPProxy
	cfg.HTTPSProxy = proxySpec.HTTPSProxy
	cfg.NoProxy = proxySpec.ProvidedNoProxy

	cfg.NetworkInterface = networkSpec.NetworkInterface
	cfg.PodCIDR = networkSpec.PodCIDR
	cfg.ServiceCIDR = networkSpec.ServiceCIDR
	cfg.GlobalCIDR = networkSpec.GlobalCIDR

	cfg.Overrides = runtimeconfig.EndUserK0sConfigOverrides()

	return cfg, nil
}

func applyInstallationConfigToRuntimeConfig(config InstallationConfig) error {
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

	networkSpec := getNetworkSpecFromConfig(config)
	runtimeconfig.SetNetworkSpec(networkSpec)

	euOverrides, err := getEndUserK0sConfigOverridesFromConfig(config)
	if err != nil {
		return fmt.Errorf("get end user k0s config overrides from config: %w", err)
	}
	runtimeconfig.SetEndUserK0sConfigOverrides(euOverrides)

	return nil
}

func getProxySpecFromConfig(config InstallationConfig) (*ecv1beta1.ProxySpec, error) {
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

func getNetworkSpecFromConfig(config InstallationConfig) *ecv1beta1.NetworkSpec {
	return &ecv1beta1.NetworkSpec{
		NetworkInterface: config.NetworkInterface,
		PodCIDR:          config.PodCIDR,
		ServiceCIDR:      config.ServiceCIDR,
		GlobalCIDR:       config.GlobalCIDR,
		// TODO: NodePortRange from k0s config
	}
}

func getEndUserK0sConfigOverridesFromConfig(config InstallationConfig) (string, error) {
	if config.Overrides == "" {
		return "", nil
	}

	eucfg, err := helpers.ParseEndUserConfig(config.Overrides)
	if err != nil {
		return "", fmt.Errorf("parse end user config: %w", err)
	}
	if eucfg != nil {
		return eucfg.Spec.UnsupportedOverrides.K0s, nil
	}

	return "", nil
}
