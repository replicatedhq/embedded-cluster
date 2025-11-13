package install

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/sirupsen/logrus"
)

func (c *InstallController) GetInstallationConfig(ctx context.Context) (types.LinuxInstallationConfigResponse, error) {
	// Get stored config (user values only)
	values, err := c.installationManager.GetConfigValues()
	if err != nil {
		return types.LinuxInstallationConfigResponse{}, fmt.Errorf("get config values: %w", err)
	}

	// Get defaults separately
	defaults, err := c.installationManager.GetDefaults(c.rc)
	if err != nil {
		return types.LinuxInstallationConfigResponse{}, fmt.Errorf("get defaults: %w", err)
	}

	// Get the final "resolved" config with the user values and defaults applied
	config, err := c.installationManager.GetConfig(c.rc)
	if err != nil {
		return types.LinuxInstallationConfigResponse{}, fmt.Errorf("get config: %w", err)
	}

	return types.LinuxInstallationConfigResponse{
		Values:   values,
		Defaults: defaults,
		Resolved: config,
	}, nil
}

func (c *InstallController) ConfigureInstallation(ctx context.Context, config types.LinuxInstallationConfig) error {
	logger := c.logger.WithField("operation", "configure-installation")

	err := c.configureInstallation(ctx, logger, config)
	if err != nil {
		return err
	}

	go func() {
		// Background context is used to avoid canceling the operation if the context is canceled
		ctx := context.Background()

		err := c.configureHost(ctx, logger)
		if err != nil {
			logger.WithError(err).Error("failed to configure host")
		}
	}()

	return nil
}

func (c *InstallController) configureInstallation(_ context.Context, logger logrus.FieldLogger, config types.LinuxInstallationConfig) (finalErr error) {
	lock, err := c.stateMachine.AcquireLock()
	if err != nil {
		return types.NewConflictError(err)
	}
	defer lock.Release()

	if err := c.stateMachine.ValidateTransition(lock, states.StateInstallationConfiguring, states.StateInstallationConfigured); err != nil {
		return types.NewConflictError(err)
	}

	err = c.stateMachine.Transition(lock, states.StateInstallationConfiguring, nil)
	if err != nil {
		return fmt.Errorf("failed to transition states: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v: %s", r, string(debug.Stack()))
		}
		if finalErr != nil {
			logger.Error(finalErr)

			if err := c.stateMachine.Transition(lock, states.StateInstallationConfigurationFailed, finalErr); err != nil {
				logger.WithError(err).Error("failed to transition states")
			}

			if err = c.setInstallationStatus(types.StateFailed, finalErr.Error()); err != nil {
				logger.WithError(err).Error("failed to set status to failed")
			}
		}
	}()

	if err := c.setInstallationStatus(types.StateRunning, "Configuring installation"); err != nil {
		return fmt.Errorf("set status to running: %w", err)
	}

	// Store the user provided values
	if err := c.installationManager.SetConfigValues(config); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	// Get the resolved config with defaults applied and CIDRs computed
	resolvedConfig, err := c.installationManager.GetConfig(c.rc)
	if err != nil {
		return fmt.Errorf("get resolved config: %w", err)
	}

	if err := c.installationManager.ValidateConfig(resolvedConfig, c.rc.ManagerPort()); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	proxy, err := newconfig.GetProxySpec(resolvedConfig.HTTPProxy, resolvedConfig.HTTPSProxy, resolvedConfig.NoProxy, resolvedConfig.PodCIDR, resolvedConfig.ServiceCIDR, resolvedConfig.NetworkInterface, c.netUtils)
	if err != nil {
		return fmt.Errorf("get proxy spec: %w", err)
	}

	networkSpec := ecv1beta1.NetworkSpec{
		NetworkInterface: resolvedConfig.NetworkInterface,
		GlobalCIDR:       resolvedConfig.GlobalCIDR,
		PodCIDR:          resolvedConfig.PodCIDR,
		ServiceCIDR:      resolvedConfig.ServiceCIDR,
		NodePortRange:    c.rc.NodePortRange(),
	}

	// TODO (@team): discuss the distinction between the runtime config and the installation config
	// update the runtime config
	c.rc.SetDataDir(resolvedConfig.DataDirectory)
	c.rc.SetLocalArtifactMirrorPort(resolvedConfig.LocalArtifactMirrorPort)
	c.rc.SetAdminConsolePort(resolvedConfig.AdminConsolePort)
	c.rc.SetProxySpec(proxy)
	c.rc.SetNetworkSpec(networkSpec)

	// update process env vars from the runtime config
	if err := c.rc.SetEnv(); err != nil {
		return fmt.Errorf("set env vars: %w", err)
	}

	err = c.stateMachine.Transition(lock, states.StateInstallationConfigured, nil)
	if err != nil {
		return fmt.Errorf("failed to transition states: %w", err)
	}

	return nil
}

func (c *InstallController) configureHost(ctx context.Context, logger logrus.FieldLogger) (finalErr error) {
	lock, err := c.stateMachine.AcquireLock()
	if err != nil {
		return types.NewConflictError(err)
	}
	defer lock.Release()

	err = c.stateMachine.Transition(lock, states.StateHostConfiguring, nil)
	if err != nil {
		return fmt.Errorf("failed to transition states: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v: %s", r, string(debug.Stack()))
		}
		if finalErr != nil {
			logger.Error(finalErr)

			if err := c.stateMachine.Transition(lock, states.StateHostConfigurationFailed, finalErr); err != nil {
				logger.WithError(err).Error("failed to transition states")
			}

			if err = c.setInstallationStatus(types.StateFailed, finalErr.Error()); err != nil {
				logger.WithError(err).Error("failed to set status to failed")
			}
		}
	}()

	err = c.installationManager.ConfigureHost(ctx, c.rc)
	if err != nil {
		return fmt.Errorf("configure host: %w", err)
	}

	err = c.stateMachine.Transition(lock, states.StateHostConfigured, nil)
	if err != nil {
		return fmt.Errorf("failed to transition states: %w", err)
	}

	if err := c.setInstallationStatus(types.StateSucceeded, "Installation configured"); err != nil {
		logger.WithError(err).Error("failed to set status to succeeded")
	}

	return nil
}

func (c *InstallController) GetInstallationStatus(ctx context.Context) (types.Status, error) {
	return c.store.LinuxInstallationStore().GetStatus()
}

func (c *InstallController) CalculateRegistrySettings(ctx context.Context) (*types.RegistrySettings, error) {
	return c.installationManager.CalculateRegistrySettings(ctx, c.rc)
}

func (c *InstallController) setInstallationStatus(state types.State, description string) error {
	return c.store.LinuxInstallationStore().SetStatus(types.Status{
		State:       state,
		Description: description,
		LastUpdated: time.Now(),
	})
}
