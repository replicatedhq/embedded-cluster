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

func (m *installationManager) GetConfig() (types.KubernetesInstallationConfig, error) {
	return m.installationStore.GetConfig()
}

func (m *installationManager) SetConfig(config types.KubernetesInstallationConfig) error {
	return m.installationStore.SetConfig(config)
}

// GetDefaults returns the default values for Kubernetes installation configuration
func (m *installationManager) GetDefaults() (types.KubernetesInstallationConfig, error) {
	defaults := types.KubernetesInstallationConfig{
		AdminConsolePort: ecv1beta1.DefaultAdminConsolePort,
	}

	return defaults, nil
}

// SetConfigDefaults sets default values for the installation configuration
func (m *installationManager) SetConfigDefaults(config *types.KubernetesInstallationConfig) error {
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

	if err := m.ValidateConfig(config, ki.ManagerPort()); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	if err := m.SetConfig(config); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	// update the kubernetes installation
	ki.SetAdminConsolePort(config.AdminConsolePort)

	if config.HTTPProxy != "" || config.HTTPSProxy != "" || config.NoProxy != "" {
		ki.SetProxySpec(&ecv1beta1.ProxySpec{
			HTTPProxy:       config.HTTPProxy,
			HTTPSProxy:      config.HTTPSProxy,
			NoProxy:         config.NoProxy,
			ProvidedNoProxy: config.NoProxy,
		})
	} else {
		ki.SetProxySpec(nil)
	}

	return nil
}
