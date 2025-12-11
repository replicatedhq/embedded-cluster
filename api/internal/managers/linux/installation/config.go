package installation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/internal/clients"
	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kyaml "sigs.k8s.io/yaml"
)

// GetConfig returns the resolved installation configuration, with the user provided values AND defaults applied
func (m *installationManager) GetConfig(rc runtimeconfig.RuntimeConfig) (types.LinuxInstallationConfig, error) {
	config, err := m.GetConfigValues()
	if err != nil {
		return types.LinuxInstallationConfig{}, fmt.Errorf("get config: %w", err)
	}

	if err := m.setConfigDefaults(&config, rc); err != nil {
		return types.LinuxInstallationConfig{}, fmt.Errorf("set config defaults: %w", err)
	}

	if err := m.computeCIDRs(&config); err != nil {
		return types.LinuxInstallationConfig{}, fmt.Errorf("compute cidrs: %w", err)
	}

	return config, nil
}

// GetConfigValues returns the installation configuration values provided by the user
func (m *installationManager) GetConfigValues() (types.LinuxInstallationConfig, error) {
	return m.installationStore.GetConfig()
}

// SetConfigValues persists the user provided changes to the installation config
func (m *installationManager) SetConfigValues(config types.LinuxInstallationConfig) error {
	return m.installationStore.SetConfig(config)
}

// GetDefaults returns the default values for installation configuration
func (m *installationManager) GetDefaults(rc runtimeconfig.RuntimeConfig) (types.LinuxInstallationConfig, error) {
	defaults := types.LinuxInstallationConfig{
		DataDirectory:           rc.EmbeddedClusterHomeDirectory(),
		LocalArtifactMirrorPort: ecv1beta1.DefaultLocalArtifactMirrorPort,
		GlobalCIDR:              ecv1beta1.DefaultNetworkCIDR,
	}

	// Try to auto-detect network interface for defaults
	if autoInterface, err := m.netUtils.DetermineBestNetworkInterface(); err == nil {
		defaults.NetworkInterface = autoInterface
	}

	// Get the proxy default values and apply them to the defaults
	proxy := &ecv1beta1.ProxySpec{}
	newconfig.SetProxyDefaults(proxy)
	defaults.HTTPProxy = proxy.HTTPProxy
	defaults.HTTPSProxy = proxy.HTTPSProxy
	defaults.NoProxy = proxy.ProvidedNoProxy

	return defaults, nil
}

// setConfigDefaults sets default values for the installation configuration
func (m *installationManager) setConfigDefaults(config *types.LinuxInstallationConfig, rc runtimeconfig.RuntimeConfig) error {
	defaults, err := m.GetDefaults(rc)
	if err != nil {
		return fmt.Errorf("get defaults: %w", err)
	}

	if config.DataDirectory == "" {
		config.DataDirectory = defaults.DataDirectory
	}

	if config.LocalArtifactMirrorPort == 0 {
		config.LocalArtifactMirrorPort = defaults.LocalArtifactMirrorPort
	}

	if config.NetworkInterface == "" {
		config.NetworkInterface = defaults.NetworkInterface
	}

	// apply the proxy default values
	if config.HTTPProxy == "" {
		config.HTTPProxy = defaults.HTTPProxy
	}
	if config.HTTPSProxy == "" {
		config.HTTPSProxy = defaults.HTTPSProxy
	}
	if config.NoProxy == "" {
		config.NoProxy = defaults.NoProxy
	}

	// if the client has not explicitly set / used pod/service cidrs, we assume the client is using the global cidr
	// and only popluate the default for the global cidr.
	// we don't populate pod/service cidrs defaults because the client would have to explicitly
	// set them in order to use them in place of the global cidr.
	if config.PodCIDR == "" && config.ServiceCIDR == "" && config.GlobalCIDR == "" {
		config.GlobalCIDR = defaults.GlobalCIDR
	}

	return nil
}

// computeCIDRs computes PodCIDR and ServiceCIDR from GlobalCIDR if GlobalCIDR is set
func (m *installationManager) computeCIDRs(config *types.LinuxInstallationConfig) error {
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

func (m *installationManager) validateLocalArtifactMirrorPort(config types.LinuxInstallationConfig, managerPort int) error {
	if config.LocalArtifactMirrorPort == 0 {
		return errors.New("localArtifactMirrorPort is required")
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

func (m *installationManager) ConfigureHost(ctx context.Context, rc runtimeconfig.RuntimeConfig) (finalErr error) {
	opts := hostutils.InitForInstallOptions{
		License:      m.license,
		AirgapBundle: m.airgapBundle,
	}
	if err := m.hostUtils.ConfigureHost(ctx, rc, opts); err != nil {
		return fmt.Errorf("configure host: %w", err)
	}

	return nil
}

// CalculateRegistrySettings calculates registry settings for both online and airgap installations
func (m *installationManager) CalculateRegistrySettings(ctx context.Context, rc runtimeconfig.RuntimeConfig) (*types.RegistrySettings, error) {
	if m.airgapBundle == "" {
		registrySettings, err := m.getOnlineRegistrySettings()
		if err != nil {
			return nil, fmt.Errorf("failed to get online registry settings: %w", err)
		}
		return registrySettings, nil
	}

	// Get app slug for secret name
	if m.releaseData == nil || m.releaseData.ChannelRelease == nil || m.releaseData.ChannelRelease.AppSlug == "" {
		return nil, fmt.Errorf("release data with app slug is required for registry settings")
	}
	appSlug := m.releaseData.ChannelRelease.AppSlug

	// Use runtime config as the authoritative source for service CIDR
	serviceCIDR := rc.ServiceCIDR()

	registryIP, err := registry.GetRegistryClusterIP(serviceCIDR)
	if err != nil {
		return nil, fmt.Errorf("failed to get registry cluster IP: %w", err)
	}

	// Construct registry host with port
	registryHost := fmt.Sprintf("%s:5000", registryIP)

	// Construct full registry address with namespace
	appNamespace := appSlug // registry namespace is the same as the app slug in linux target
	registryAddress := fmt.Sprintf("%s/%s", registryHost, appNamespace)

	// Get registry credentials
	registryUsername := "embedded-cluster"
	registryPassword := registry.GetRegistryPassword()

	// Create image pull secret value using the same pattern as admin console
	authString := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", registryUsername, registryPassword)))
	authConfig := fmt.Sprintf(`{"auths":{"%s":{"username": "%s", "password": "%s", "auth": "%s"}}}`,
		registryHost, registryUsername, registryPassword, authString)
	imagePullSecretValue := base64.StdEncoding.EncodeToString([]byte(authConfig))

	return &types.RegistrySettings{
		HasLocalRegistry:       true,
		LocalRegistryHost:      registryHost,
		LocalRegistryAddress:   registryAddress,
		LocalRegistryNamespace: appNamespace,
		LocalRegistryUsername:  registryUsername,
		LocalRegistryPassword:  registryPassword,
		ImagePullSecretName:    fmt.Sprintf("%s-registry", appSlug),
		ImagePullSecretValue:   imagePullSecretValue,
	}, nil
}

// GetRegistrySettings reads registry settings from the cluster for airgap installations, and calculates them for online installations (should be used for upgrades)
func (m *installationManager) GetRegistrySettings(ctx context.Context, rc runtimeconfig.RuntimeConfig) (*types.RegistrySettings, error) {
	// If no airgap bundle, no registry settings needed
	if m.airgapBundle == "" {
		registrySettings, err := m.getOnlineRegistrySettings()
		if err != nil {
			return nil, fmt.Errorf("failed to get online registry settings: %w", err)
		}
		return registrySettings, nil
	}

	// Get app slug for secret name
	if m.releaseData == nil || m.releaseData.ChannelRelease == nil || m.releaseData.ChannelRelease.AppSlug == "" {
		return nil, fmt.Errorf("release data with app slug is required for registry settings")
	}
	appSlug := m.releaseData.ChannelRelease.AppSlug

	if m.kcli == nil {
		kcli, err := clients.NewKubeClient(clients.KubeClientOptions{KubeConfigPath: rc.PathToKubeConfig()})
		if err != nil {
			return nil, fmt.Errorf("create kube client: %w", err)
		}
		m.kcli = kcli
	}

	kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(ctx, m.kcli)
	if err != nil {
		return nil, fmt.Errorf("get kotsadm namespace: %w", err)
	}

	// Read the registry-creds secret from kotsadm namespace
	secret := &corev1.Secret{}
	err = m.kcli.Get(ctx, client.ObjectKey{Namespace: kotsadmNamespace, Name: "registry-creds"}, secret)
	if err != nil {
		return nil, fmt.Errorf("get registry-creds secret: %w", err)
	}

	// Verify it's a docker config secret
	if secret.Type != corev1.SecretTypeDockerConfigJson {
		return nil, fmt.Errorf("registry-creds secret is not of type kubernetes.io/dockerconfigjson")
	}

	// Parse the dockerconfigjson
	dockerJson, ok := secret.Data[".dockerconfigjson"]
	if !ok {
		return nil, fmt.Errorf("registry-creds secret missing .dockerconfigjson data")
	}

	type dockerRegistryAuth struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Auth     string `json:"auth"`
	}
	dockerConfig := struct {
		Auths map[string]dockerRegistryAuth `json:"auths"`
	}{}

	if err := json.Unmarshal(dockerJson, &dockerConfig); err != nil {
		return nil, fmt.Errorf("parse dockerconfigjson: %w", err)
	}

	// Find the embedded-cluster registry auth
	var registryHost string
	var registryUsername string
	var registryPassword string
	for host, auth := range dockerConfig.Auths {
		if auth.Username == "embedded-cluster" {
			registryHost = host
			registryUsername = auth.Username
			registryPassword = auth.Password
			break
		}
	}

	if registryHost == "" {
		return nil, fmt.Errorf("embedded-cluster username not found in registry-creds secret")
	}

	// Construct full registry address with namespace
	appNamespace := appSlug // registry namespace is the same as the app slug in linux target
	registryAddress := fmt.Sprintf("%s/%s", registryHost, appNamespace)

	// Use the full dockerconfigjson as the image pull secret value
	imagePullSecretValue := base64.StdEncoding.EncodeToString(dockerJson)

	return &types.RegistrySettings{
		HasLocalRegistry:       true,
		LocalRegistryHost:      registryHost,
		LocalRegistryAddress:   registryAddress,
		LocalRegistryNamespace: appNamespace,
		LocalRegistryUsername:  registryUsername,
		LocalRegistryPassword:  registryPassword,
		ImagePullSecretName:    fmt.Sprintf("%s-registry", appSlug),
		ImagePullSecretValue:   imagePullSecretValue,
	}, nil
}

func (m *installationManager) getOnlineRegistrySettings() (*types.RegistrySettings, error) {
	// Online mode: Use replicated proxy registry with license ID authentication
	ecDomains := utils.GetDomains(m.releaseData)

	// Parse license from bytes
	if len(m.license) == 0 {
		return nil, fmt.Errorf("license is required for online registry settings")
	}
	license := &kotsv1beta1.License{}
	if err := kyaml.Unmarshal(m.license, license); err != nil {
		return nil, fmt.Errorf("parse license: %w", err)
	}

	// Get app slug for secret name
	if m.releaseData == nil || m.releaseData.ChannelRelease == nil || m.releaseData.ChannelRelease.AppSlug == "" {
		return nil, fmt.Errorf("release data with app slug is required for registry settings")
	}
	appSlug := m.releaseData.ChannelRelease.AppSlug

	// Create auth config for both proxy and registry domains
	authConfig := fmt.Sprintf(`{"auths":{"%s":{"username": "LICENSE_ID", "password": "%s"},"%s":{"username": "LICENSE_ID", "password": "%s"}}}`,
		ecDomains.ProxyRegistryDomain, license.Spec.LicenseID, ecDomains.ReplicatedRegistryDomain, license.Spec.LicenseID)
	imagePullSecretValue := base64.StdEncoding.EncodeToString([]byte(authConfig))

	return &types.RegistrySettings{
		HasLocalRegistry:     false,
		ImagePullSecretName:  fmt.Sprintf("%s-registry", appSlug),
		ImagePullSecretValue: imagePullSecretValue,
	}, nil
}
