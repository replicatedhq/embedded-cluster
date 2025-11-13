package installation

import (
	"context"
	"errors"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/kubernetesinstallation"
)

// GetConfig returns the resolved installation configuration, with the user provided values AND defaults applied
func (m *installationManager) GetConfig() (types.KubernetesInstallationConfig, error) {
	config, err := m.GetConfigValues()
	if err != nil {
		return types.KubernetesInstallationConfig{}, fmt.Errorf("get config: %w", err)
	}

	if err := m.setConfigDefaults(&config); err != nil {
		return types.KubernetesInstallationConfig{}, fmt.Errorf("set config defaults: %w", err)
	}

	return config, nil
}

// GetConfigValues returns the installation configuration values provided by the user
func (m *installationManager) GetConfigValues() (types.KubernetesInstallationConfig, error) {
	return m.installationStore.GetConfig()
}

// SetConfigValues persists the user provided changes to the installation config
func (m *installationManager) SetConfigValues(config types.KubernetesInstallationConfig) error {
	return m.installationStore.SetConfig(config)
}

// GetDefaults returns the default values for Kubernetes installation configuration
func (m *installationManager) GetDefaults() (types.KubernetesInstallationConfig, error) {
	defaults := types.KubernetesInstallationConfig{
		AdminConsolePort: ecv1beta1.DefaultAdminConsolePort,
	}

	return defaults, nil
}

// setConfigDefaults sets default values for the installation configuration
func (m *installationManager) setConfigDefaults(config *types.KubernetesInstallationConfig) error {
	defaults, err := m.GetDefaults()
	if err != nil {
		return fmt.Errorf("get defaults: %w", err)
	}

	//Note: we don't set defaults for HTTPProxy, HTTPSProxy, NoProxy (like we do for the linux target) since the settings on the host (such as env vars, etc) are likely different form what we want to run on the k8s cluster

	if config.AdminConsolePort == 0 {
		config.AdminConsolePort = defaults.AdminConsolePort
	}

	return nil
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

func (m *installationManager) ConfigureInstallation(ctx context.Context, ki kubernetesinstallation.Installation, config types.KubernetesInstallationConfig) error {
	// Store the user provided values before applying the defaults
	if err := m.SetConfigValues(config); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	// Get the resolved config with defaults applied
	resolvedConfig, err := m.GetConfig()
	if err != nil {
		return fmt.Errorf("get resolved config: %w", err)
	}

	if err := m.ValidateConfig(resolvedConfig, ki.ManagerPort()); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	// update the kubernetes installation
	ki.SetAdminConsolePort(resolvedConfig.AdminConsolePort)

	if resolvedConfig.HTTPProxy != "" || resolvedConfig.HTTPSProxy != "" || resolvedConfig.NoProxy != "" {
		ki.SetProxySpec(&ecv1beta1.ProxySpec{
			HTTPProxy:       resolvedConfig.HTTPProxy,
			HTTPSProxy:      resolvedConfig.HTTPSProxy,
			NoProxy:         resolvedConfig.NoProxy,
			ProvidedNoProxy: resolvedConfig.NoProxy,
		})
	} else {
		ki.SetProxySpec(nil)
	}

	return nil
}
