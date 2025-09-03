package installation

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

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
	m.setConfigDefaults(&config)
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

func (m *installationManager) ConfigureInstallation(ctx context.Context, ki kubernetesinstallation.Installation, config types.KubernetesInstallationConfig) (finalErr error) {
	if err := m.setStatus(types.StateRunning, ""); err != nil {
		return fmt.Errorf("set status: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v: %s", r, string(debug.Stack()))
		}
		if finalErr != nil {
			if err := m.setStatus(types.StateFailed, finalErr.Error()); err != nil {
				m.logger.WithError(err).Error("set failed status")
			}
		} else {
			if err := m.setStatus(types.StateSucceeded, "Installation configured"); err != nil {
				m.logger.WithError(err).Error("set succeeded status")
			}
		}
	}()

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
