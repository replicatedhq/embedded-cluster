package install

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	states "github.com/replicatedhq/embedded-cluster/api/internal/states/install"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
)

func (c *InstallController) GetInstallationConfig(ctx context.Context) (types.LinuxInstallationConfig, error) {
	config, err := c.installationManager.GetConfig()
	if err != nil {
		return types.LinuxInstallationConfig{}, err
	}

	if err := c.installationManager.SetConfigDefaults(&config, c.rc); err != nil {
		return types.LinuxInstallationConfig{}, fmt.Errorf("set defaults: %w", err)
	}

	if err := c.installationManager.ValidateConfig(config, c.rc.ManagerPort()); err != nil {
		return types.LinuxInstallationConfig{}, fmt.Errorf("validate: %w", err)
	}

	return config, nil
}

func (c *InstallController) ConfigureInstallation(ctx context.Context, config types.LinuxInstallationConfig) error {
	err := c.configureInstallation(ctx, config)
	if err != nil {
		return err
	}

	go func() {
		// Background context is used to avoid canceling the operation if the context is canceled
		ctx := context.Background()

		lock, err := c.stateMachine.AcquireLock()
		if err != nil {
			c.logger.WithError(err).Error("failed to acquire lock")
			return
		}
		defer lock.Release()

		err = c.stateMachine.Transition(lock, states.StateHostConfiguring)
		if err != nil {
			c.logger.WithError(err).Error("failed to transition states")
			return
		}

		err = c.installationManager.ConfigureHost(ctx, c.rc)

		if err != nil {
			c.logger.WithError(err).Error("failed to configure host")
			err = c.stateMachine.Transition(lock, states.StateHostConfigurationFailed)
			if err != nil {
				c.logger.WithError(err).Error("failed to transition states")
			}
		} else {
			err = c.stateMachine.Transition(lock, states.StateHostConfigured)
			if err != nil {
				c.logger.WithError(err).Error("failed to transition states")
			}
		}
	}()

	return nil
}

func (c *InstallController) configureInstallation(_ context.Context, config types.LinuxInstallationConfig) (finalErr error) {
	lock, err := c.stateMachine.AcquireLock()
	if err != nil {
		return types.NewConflictError(err)
	}
	defer lock.Release()

	if err := c.stateMachine.ValidateTransition(lock, states.StateInstallationConfiguring, states.StateInstallationConfigured); err != nil {
		return types.NewConflictError(err)
	}

	err = c.stateMachine.Transition(lock, states.StateInstallationConfiguring)
	if err != nil {
		return fmt.Errorf("failed to transition states: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v: %s", r, string(debug.Stack()))
		}
		if finalErr != nil {
			failureStatus := types.Status{
				State:       types.StateFailed,
				Description: finalErr.Error(),
				LastUpdated: time.Now(),
			}

			if err = c.store.LinuxInstallationStore().SetStatus(failureStatus); err != nil {
				c.logger.Errorf("failed to update status: %w", err)
			}

			if err := c.stateMachine.Transition(lock, states.StateInstallationConfigurationFailed); err != nil {
				c.logger.Errorf("failed to transition states: %w", err)
			}
		}
	}()

	if err := c.installationManager.ValidateConfig(config, c.rc.ManagerPort()); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	if err := c.computeCIDRs(&config); err != nil {
		return fmt.Errorf("compute cidrs: %w", err)
	}

	if err := c.installationManager.SetConfig(config); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	proxy, err := newconfig.GetProxySpec(config.HTTPProxy, config.HTTPSProxy, config.NoProxy, config.PodCIDR, config.ServiceCIDR, config.NetworkInterface, c.netUtils)
	if err != nil {
		return fmt.Errorf("get proxy spec: %w", err)
	}

	networkSpec := ecv1beta1.NetworkSpec{
		NetworkInterface: config.NetworkInterface,
		GlobalCIDR:       config.GlobalCIDR,
		PodCIDR:          config.PodCIDR,
		ServiceCIDR:      config.ServiceCIDR,
		NodePortRange:    c.rc.NodePortRange(),
	}

	// TODO (@team): discuss the distinction between the runtime config and the installation config
	// update the runtime config
	c.rc.SetDataDir(config.DataDirectory)
	c.rc.SetLocalArtifactMirrorPort(config.LocalArtifactMirrorPort)
	c.rc.SetAdminConsolePort(config.AdminConsolePort)
	c.rc.SetProxySpec(proxy)
	c.rc.SetNetworkSpec(networkSpec)

	// update process env vars from the runtime config
	if err := c.rc.SetEnv(); err != nil {
		return fmt.Errorf("set env vars: %w", err)
	}

	err = c.stateMachine.Transition(lock, states.StateInstallationConfigured)
	if err != nil {
		return fmt.Errorf("failed to transition states: %w", err)
	}

	return nil
}

func (c *InstallController) computeCIDRs(config *types.LinuxInstallationConfig) error {
	if config.GlobalCIDR != "" {
		podCIDR, serviceCIDR, err := netutils.SplitNetworkCIDR(config.GlobalCIDR)
		if err != nil {
			return fmt.Errorf("split network cidr: %w", err)
		}
		config.PodCIDR = podCIDR
		config.ServiceCIDR = serviceCIDR
	}

	return nil
}

func (c *InstallController) GetInstallationStatus(ctx context.Context) (types.Status, error) {
	return c.installationManager.GetStatus()
}

func (c *InstallController) CalculateRegistrySettings(ctx context.Context) (*types.RegistrySettings, error) {
	return c.installationManager.CalculateRegistrySettings(ctx, c.rc)
}
