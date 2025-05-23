package installation

import (
	"errors"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/pkg/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
)

var _ ConfigManager = &configManager{}

// ConfigManager provides methods for validating and setting defaults for installation configuration
type ConfigManager interface {
	Read() (*types.InstallationConfig, error)
	Write(config types.InstallationConfig) error
	Validate(config *types.InstallationConfig) error
	SetDefaults(config *types.InstallationConfig) error
}

// configManager is an implementation of the ConfigManager interface
type configManager struct {
	configStore ConfigStore
	netUtils    utils.NetUtils
}

type ConfigManagerOption func(*configManager)

func WithConfigStore(configStore ConfigStore) ConfigManagerOption {
	return func(c *configManager) {
		c.configStore = configStore
	}
}

func WithNetUtils(netUtils utils.NetUtils) ConfigManagerOption {
	return func(c *configManager) {
		c.netUtils = netUtils
	}
}

// NewConfigManager creates a new ConfigManager with the provided network utilities
func NewConfigManager(opts ...ConfigManagerOption) *configManager {
	manager := &configManager{}

	for _, opt := range opts {
		opt(manager)
	}

	if manager.configStore == nil {
		manager.configStore = NewConfigMemoryStore()
	}

	if manager.netUtils == nil {
		manager.netUtils = utils.NewNetUtils()
	}

	return manager
}

func (m *configManager) Read() (*types.InstallationConfig, error) {
	return m.configStore.Read()
}

func (m *configManager) Write(config types.InstallationConfig) error {
	return m.configStore.Write(config)
}

func (m *configManager) Validate(config *types.InstallationConfig) error {
	var ve *types.APIError

	if err := m.validateGlobalCIDR(config); err != nil {
		ve = types.AppendFieldError(ve, "globalCidr", err)
	}

	if err := m.validatePodCIDR(config); err != nil {
		ve = types.AppendFieldError(ve, "podCidr", err)
	}

	if err := m.validateServiceCIDR(config); err != nil {
		ve = types.AppendFieldError(ve, "serviceCidr", err)
	}

	if err := m.validateNetworkInterface(config); err != nil {
		ve = types.AppendFieldError(ve, "networkInterface", err)
	}

	if err := m.validateAdminConsolePort(config); err != nil {
		ve = types.AppendFieldError(ve, "adminConsolePort", err)
	}

	if err := m.validateLocalArtifactMirrorPort(config); err != nil {
		ve = types.AppendFieldError(ve, "localArtifactMirrorPort", err)
	}

	if err := m.validateDataDirectory(config); err != nil {
		ve = types.AppendFieldError(ve, "dataDirectory", err)
	}

	return ve.ErrorOrNil()
}

func (m *configManager) validateGlobalCIDR(config *types.InstallationConfig) error {
	if config.GlobalCIDR == "" {
		if config.PodCIDR == "" && config.ServiceCIDR == "" {
			return errors.New("globalCidr is required")
		}
		return nil
	}

	if err := netutils.ValidateCIDR(config.GlobalCIDR, 16, true); err != nil {
		return err
	}

	podCIDR, serviceCIDR, err := newconfig.SplitCIDR(config.GlobalCIDR)
	if err != nil {
		return fmt.Errorf("split globalCidr: %w", err)
	}
	if config.PodCIDR != "" && podCIDR != config.PodCIDR {
		return errors.New("podCidr does not match globalCIDR")
	}
	if config.ServiceCIDR != "" && serviceCIDR != config.ServiceCIDR {
		return errors.New("serviceCidr does not match globalCIDR")
	}

	return nil
}

func (m *configManager) validatePodCIDR(config *types.InstallationConfig) error {
	if config.ServiceCIDR != "" && config.PodCIDR == "" {
		return errors.New("podCidr is required when serviceCidr is set")
	}
	return nil
}

func (m *configManager) validateServiceCIDR(config *types.InstallationConfig) error {
	if config.PodCIDR != "" && config.ServiceCIDR == "" {
		return errors.New("serviceCidr is required when podCidr is set")
	}
	return nil
}

func (m *configManager) validateNetworkInterface(config *types.InstallationConfig) error {
	if config.NetworkInterface == "" {
		return errors.New("networkInterface is required")
	}

	// TODO: validate the network interface exists and is up and not loopback
	return nil
}

func (m *configManager) validateAdminConsolePort(config *types.InstallationConfig) error {
	if config.AdminConsolePort == 0 {
		return errors.New("adminConsolePort is required")
	}

	lamPort := config.LocalArtifactMirrorPort
	if lamPort == 0 {
		lamPort = ecv1beta1.DefaultLocalArtifactMirrorPort
	}

	if config.AdminConsolePort == lamPort {
		return errors.New("adminConsolePort and localArtifactMirrorPort cannot be equal")
	}

	return nil
}

func (m *configManager) validateLocalArtifactMirrorPort(config *types.InstallationConfig) error {
	if config.LocalArtifactMirrorPort == 0 {
		return errors.New("localArtifactMirrorPort is required")
	}

	acPort := config.AdminConsolePort
	if acPort == 0 {
		acPort = ecv1beta1.DefaultAdminConsolePort
	}

	if config.LocalArtifactMirrorPort == acPort {
		return errors.New("adminConsolePort and localArtifactMirrorPort cannot be equal")
	}

	return nil
}

func (m *configManager) validateDataDirectory(config *types.InstallationConfig) error {
	if config.DataDirectory == "" {
		return errors.New("dataDirectory is required")
	}

	return nil
}

// SetDefaults sets default values for the installation configuration
func (m *configManager) SetDefaults(config *types.InstallationConfig) error {
	if config.AdminConsolePort == 0 {
		config.AdminConsolePort = ecv1beta1.DefaultAdminConsolePort
	}

	if config.DataDirectory == "" {
		config.DataDirectory = ecv1beta1.DefaultDataDir
	}

	if config.LocalArtifactMirrorPort == 0 {
		config.LocalArtifactMirrorPort = ecv1beta1.DefaultLocalArtifactMirrorPort
	}

	// if a network interface was not provided, attempt to discover it
	if config.NetworkInterface == "" {
		autoInterface, err := m.netUtils.DetermineBestNetworkInterface()
		if err == nil {
			config.NetworkInterface = autoInterface
		}
	}

	if err := m.setCIDRDefaults(config); err != nil {
		return fmt.Errorf("unable to set cidr defaults: %w", err)
	}

	m.setProxyDefaults(config)

	return nil
}

func (m *configManager) setProxyDefaults(config *types.InstallationConfig) {
	proxy := &ecv1beta1.ProxySpec{
		HTTPProxy:       config.HTTPProxy,
		HTTPSProxy:      config.HTTPSProxy,
		ProvidedNoProxy: config.NoProxy,
	}
	newconfig.SetProxyDefaults(proxy)

	config.HTTPProxy = proxy.HTTPProxy
	config.HTTPSProxy = proxy.HTTPSProxy
	config.NoProxy = proxy.ProvidedNoProxy
}

func (m *configManager) setCIDRDefaults(config *types.InstallationConfig) error {
	if config.PodCIDR == "" && config.ServiceCIDR == "" {
		if config.GlobalCIDR == "" {
			config.GlobalCIDR = ecv1beta1.DefaultNetworkCIDR
		}

		podCIDR, serviceCIDR, err := netutils.SplitNetworkCIDR(config.GlobalCIDR)
		if err != nil {
			return fmt.Errorf("split network cidr: %w", err)
		}
		config.PodCIDR = podCIDR
		config.ServiceCIDR = serviceCIDR

		return nil
	}

	return nil
}
