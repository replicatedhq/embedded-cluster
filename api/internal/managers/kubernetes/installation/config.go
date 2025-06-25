package installation

import (
	"errors"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
)

func (m *installationManager) GetConfig() (types.KubernetesInstallationConfig, error) {
	return m.installationStore.GetConfig()
}

func (m *installationManager) SetConfig(config types.KubernetesInstallationConfig) error {
	return m.installationStore.SetConfig(config)
}

func (m *installationManager) ValidateConfig(config types.KubernetesInstallationConfig, managerPort int) error {
	var ve *types.APIError

	if err := m.validateAdminConsolePort(config, managerPort); err != nil {
		ve = types.AppendFieldError(ve, "adminConsolePort", err)
	}

	return ve.ErrorOrNil()
}

func (m *installationManager) validateAdminConsolePort(config types.KubernetesInstallationConfig, managerPort int) error {
	if config.AdminConsolePort == 0 {
		return errors.New("adminConsolePort is required")
	}

	if config.AdminConsolePort == managerPort {
		return errors.New("adminConsolePort cannot be the same as the manager port")
	}

	return nil
}

// SetConfigDefaults sets default values for the installation configuration
func (m *installationManager) SetConfigDefaults(config *types.KubernetesInstallationConfig) error {
	if config.AdminConsolePort == 0 {
		config.AdminConsolePort = ecv1beta1.DefaultAdminConsolePort
	}

	m.setProxyDefaults(config)

	return nil
}

func (m *installationManager) setProxyDefaults(config *types.KubernetesInstallationConfig) {
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
