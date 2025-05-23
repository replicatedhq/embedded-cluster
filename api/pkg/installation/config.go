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

var _ InstallationManager = &installationManager{}

// InstallationManager provides methods for validating and setting defaults for installation configuration
type InstallationManager interface {
	ReadConfig() (*types.InstallationConfig, error)
	WriteConfig(config types.InstallationConfig) error
	ReadStatus() (*types.InstallationStatus, error)
	WriteStatus(status types.InstallationStatus) error
	ValidateConfig(config *types.InstallationConfig) error
	SetDefaults(config *types.InstallationConfig) error
}

// installationManager is an implementation of the InstallationManager interface
type installationManager struct {
	installationStore InstallationStore
	netUtils          utils.NetUtils
}

type InstallationManagerOption func(*installationManager)

func WithInstallationStore(installationStore InstallationStore) InstallationManagerOption {
	return func(c *installationManager) {
		c.installationStore = installationStore
	}
}

func WithNetUtils(netUtils utils.NetUtils) InstallationManagerOption {
	return func(c *installationManager) {
		c.netUtils = netUtils
	}
}

// NewInstallationManager creates a new InstallationManager with the provided network utilities
func NewInstallationManager(opts ...InstallationManagerOption) *installationManager {
	manager := &installationManager{}

	for _, opt := range opts {
		opt(manager)
	}

	if manager.installationStore == nil {
		manager.installationStore = NewMemoryStore()
	}

	if manager.netUtils == nil {
		manager.netUtils = utils.NewNetUtils()
	}

	return manager
}

func (m *installationManager) ReadConfig() (*types.InstallationConfig, error) {
	return m.installationStore.ReadConfig()
}

func (m *installationManager) WriteConfig(config types.InstallationConfig) error {
	return m.installationStore.WriteConfig(config)
}

func (m *installationManager) ReadStatus() (*types.InstallationStatus, error) {
	return m.installationStore.ReadStatus()
}

func (m *installationManager) WriteStatus(status types.InstallationStatus) error {
	return m.installationStore.WriteStatus(status)
}

func (m *installationManager) ValidateConfig(config *types.InstallationConfig) error {
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

func (m *installationManager) validateGlobalCIDR(config *types.InstallationConfig) error {
	if config.GlobalCIDR != "" {
		if err := netutils.ValidateCIDR(config.GlobalCIDR, 16, true); err != nil {
			return err
		}
	} else {
		if config.PodCIDR == "" && config.ServiceCIDR == "" {
			return errors.New("globalCidr is required")
		}
	}
	return nil
}

func (m *installationManager) validatePodCIDR(config *types.InstallationConfig) error {
	if config.GlobalCIDR != "" {
		return nil
	}
	if config.PodCIDR == "" {
		return errors.New("podCidr is required when globalCidr is not set")
	}
	return nil
}

func (m *installationManager) validateServiceCIDR(config *types.InstallationConfig) error {
	if config.GlobalCIDR != "" {
		return nil
	}
	if config.ServiceCIDR == "" {
		return errors.New("serviceCidr is required when globalCidr is not set")
	}
	return nil
}

func (m *installationManager) validateNetworkInterface(config *types.InstallationConfig) error {
	if config.NetworkInterface == "" {
		return errors.New("networkInterface is required")
	}

	// TODO: validate the network interface exists and is up and not loopback
	return nil
}

func (m *installationManager) validateAdminConsolePort(config *types.InstallationConfig) error {
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

func (m *installationManager) validateLocalArtifactMirrorPort(config *types.InstallationConfig) error {
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

func (m *installationManager) validateDataDirectory(config *types.InstallationConfig) error {
	if config.DataDirectory == "" {
		return errors.New("dataDirectory is required")
	}

	return nil
}

// SetDefaults sets default values for the installation configuration
func (m *installationManager) SetDefaults(config *types.InstallationConfig) error {
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

func (m *installationManager) setProxyDefaults(config *types.InstallationConfig) {
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

func (m *installationManager) setCIDRDefaults(config *types.InstallationConfig) error {
	// if the client has not explicitly set / used pod/service cidrs, we assume the client is using the global cidr
	// and only popluate the default for the global cidr.
	// we don't populate pod/service cidrs defaults because the client would have to explicitly
	// set them in order to use them in place of the global cidr.
	if config.PodCIDR == "" && config.ServiceCIDR == "" && config.GlobalCIDR == "" {
		config.GlobalCIDR = ecv1beta1.DefaultNetworkCIDR
	}
	return nil
}
