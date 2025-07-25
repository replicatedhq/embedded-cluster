package installation

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

func (m *installationManager) GetConfig() (types.LinuxInstallationConfig, error) {
	return m.installationStore.GetConfig()
}

func (m *installationManager) SetConfig(config types.LinuxInstallationConfig) error {
	return m.installationStore.SetConfig(config)
}

func (m *installationManager) ValidateConfig(config types.LinuxInstallationConfig, managerPort int) error {
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

	if err := m.validateAdminConsolePort(config, managerPort); err != nil {
		ve = types.AppendFieldError(ve, "adminConsolePort", err)
	}

	if err := m.validateLocalArtifactMirrorPort(config, managerPort); err != nil {
		ve = types.AppendFieldError(ve, "localArtifactMirrorPort", err)
	}

	if err := m.validateDataDirectory(config); err != nil {
		ve = types.AppendFieldError(ve, "dataDirectory", err)
	}

	return ve.ErrorOrNil()
}

func (m *installationManager) validateGlobalCIDR(config types.LinuxInstallationConfig) error {
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

func (m *installationManager) validatePodCIDR(config types.LinuxInstallationConfig) error {
	if config.GlobalCIDR != "" {
		return nil
	}
	if config.PodCIDR == "" {
		return errors.New("podCidr is required when globalCidr is not set")
	}
	return nil
}

func (m *installationManager) validateServiceCIDR(config types.LinuxInstallationConfig) error {
	if config.GlobalCIDR != "" {
		return nil
	}
	if config.ServiceCIDR == "" {
		return errors.New("serviceCidr is required when globalCidr is not set")
	}
	return nil
}

func (m *installationManager) validateNetworkInterface(config types.LinuxInstallationConfig) error {
	if config.NetworkInterface == "" {
		return errors.New("networkInterface is required")
	}

	// TODO: validate the network interface exists and is up and not loopback
	return nil
}

func (m *installationManager) validateAdminConsolePort(config types.LinuxInstallationConfig, managerPort int) error {
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

	if config.AdminConsolePort == managerPort {
		return errors.New("adminConsolePort cannot be the same as the manager port")
	}

	return nil
}

func (m *installationManager) validateLocalArtifactMirrorPort(config types.LinuxInstallationConfig, managerPort int) error {
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

	if config.LocalArtifactMirrorPort == managerPort {
		return errors.New("localArtifactMirrorPort cannot be the same as the manager port")
	}

	return nil
}

func (m *installationManager) validateDataDirectory(config types.LinuxInstallationConfig) error {
	if config.DataDirectory == "" {
		return errors.New("dataDirectory is required")
	}

	return nil
}

// SetConfigDefaults sets default values for the installation configuration
func (m *installationManager) SetConfigDefaults(config *types.LinuxInstallationConfig, rc runtimeconfig.RuntimeConfig) error {
	if config.AdminConsolePort == 0 {
		config.AdminConsolePort = ecv1beta1.DefaultAdminConsolePort
	}

	if config.DataDirectory == "" {
		config.DataDirectory = rc.EmbeddedClusterHomeDirectory()
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

func (m *installationManager) setProxyDefaults(config *types.LinuxInstallationConfig) {
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

func (m *installationManager) setCIDRDefaults(config *types.LinuxInstallationConfig) error {
	// if the client has not explicitly set / used pod/service cidrs, we assume the client is using the global cidr
	// and only popluate the default for the global cidr.
	// we don't populate pod/service cidrs defaults because the client would have to explicitly
	// set them in order to use them in place of the global cidr.
	if config.PodCIDR == "" && config.ServiceCIDR == "" && config.GlobalCIDR == "" {
		config.GlobalCIDR = ecv1beta1.DefaultNetworkCIDR
	}
	return nil
}

func (m *installationManager) ConfigureHost(ctx context.Context, rc runtimeconfig.RuntimeConfig) (finalErr error) {
	if err := m.setRunningStatus("Configuring installation"); err != nil {
		return fmt.Errorf("set running status: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v: %s", r, string(debug.Stack()))
		}
		if finalErr != nil {
			if err := m.setFailedStatus(finalErr.Error()); err != nil {
				m.logger.WithError(err).Error("set failed status")
			}
		} else {
			if err := m.setCompletedStatus(types.StateSucceeded, "Installation configured"); err != nil {
				m.logger.WithError(err).Error("set succeeded status")
			}
		}
	}()

	opts := hostutils.InitForInstallOptions{
		License:      m.license,
		AirgapBundle: m.airgapBundle,
	}
	if err := m.hostUtils.ConfigureHost(ctx, rc, opts); err != nil {
		return fmt.Errorf("configure host: %w", err)
	}

	return nil
}
