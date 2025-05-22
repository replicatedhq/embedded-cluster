package installation

import (
	"errors"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
)

func ConfigValidate(config *types.InstallationConfig) error {
	var ve *types.APIError

	if err := configValidateGlobalCIDR(config); err != nil {
		ve = types.AppendFieldError(ve, "globalCidr", err)
	}

	if err := configValidatePodCIDR(config); err != nil {
		ve = types.AppendFieldError(ve, "podCidr", err)
	}

	if err := configValidateServiceCIDR(config); err != nil {
		ve = types.AppendFieldError(ve, "serviceCidr", err)
	}

	if err := configValidateNetworkInterface(config); err != nil {
		ve = types.AppendFieldError(ve, "networkInterface", err)
	}

	if err := configValidateAdminConsolePort(config); err != nil {
		ve = types.AppendFieldError(ve, "adminConsolePort", err)
	}

	if err := configValidateLocalArtifactMirrorPort(config); err != nil {
		ve = types.AppendFieldError(ve, "localArtifactMirrorPort", err)
	}

	if err := configValidateDataDirectory(config); err != nil {
		ve = types.AppendFieldError(ve, "dataDirectory", err)
	}

	return ve.ErrorOrNil()
}

func configValidateGlobalCIDR(config *types.InstallationConfig) error {
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

func configValidatePodCIDR(config *types.InstallationConfig) error {
	if config.ServiceCIDR != "" && config.PodCIDR == "" {
		return errors.New("podCidr is required when serviceCidr is set")
	}
	return nil
}

func configValidateServiceCIDR(config *types.InstallationConfig) error {
	if config.PodCIDR != "" && config.ServiceCIDR == "" {
		return errors.New("serviceCidr is required when podCidr is set")
	}
	return nil
}

func configValidateNetworkInterface(config *types.InstallationConfig) error {
	if config.NetworkInterface == "" {
		return errors.New("networkInterface is required")
	}

	// TODO: validate the network interface exists and is up and not loopback
	return nil
}

func configValidateAdminConsolePort(config *types.InstallationConfig) error {
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

func configValidateLocalArtifactMirrorPort(config *types.InstallationConfig) error {
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

func configValidateDataDirectory(config *types.InstallationConfig) error {
	if config.DataDirectory == "" {
		return errors.New("dataDirectory is required")
	}

	return nil
}

func ConfigSetDefaults(config *types.InstallationConfig) error {
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
		autoInterface, err := newconfig.DetermineBestNetworkInterface()
		if err == nil {
			config.NetworkInterface = autoInterface
		}
	}

	if err := configSetCIDRDefaults(config); err != nil {
		return fmt.Errorf("unable to set cidr defaults: %w", err)
	}

	configSetProxyDefaults(config)

	return nil
}

func configSetProxyDefaults(config *types.InstallationConfig) {
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

func configSetCIDRDefaults(config *types.InstallationConfig) error {
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
