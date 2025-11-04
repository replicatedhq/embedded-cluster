package cli

import (
	"fmt"
	"path/filepath"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg-new/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/replicatedhq/embedded-cluster/web"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

// Hop: buildRuntimeConfig sets runtime values from install flags and install config
func buildRuntimeConfig(flags *installFlags, installCfg *installConfig, rc runtimeconfig.RuntimeConfig) error {
	hostCABundlePath, err := findHostCABundle()
	if err != nil {
		return fmt.Errorf("find host CA bundle: %w", err)
	}
	logrus.Debugf("using host CA bundle: %s", hostCABundlePath)

	networkSpec, err := buildNetworkSpec(flags, installCfg)
	if err != nil {
		return fmt.Errorf("build network spec: %w", err)
	}

	// TODO: validate that a single port isn't used for multiple services
	// resolve datadir to absolute path
	absoluteDataDir, err := filepath.Abs(flags.dataDir)
	if err != nil {
		return fmt.Errorf("construct path for directory: %w", err)
	}

	rc.SetAdminConsolePort(flags.adminConsolePort)
	rc.SetManagerPort(flags.managerPort)
	rc.SetProxySpec(flags.proxySpec)
	rc.SetDataDir(absoluteDataDir)
	rc.SetLocalArtifactMirrorPort(flags.localArtifactMirrorPort)
	rc.SetHostCABundlePath(hostCABundlePath)
	rc.SetNetworkSpec(networkSpec)

	return nil
}

func buildNetworkSpec(flags *installFlags, installCfg *installConfig) (ecv1beta1.NetworkSpec, error) {
	k0sCfg, err := buildK0sConfig(flags, installCfg)
	if err != nil {
		return ecv1beta1.NetworkSpec{}, fmt.Errorf("create k0s config: %w", err)
	}
	networkSpec := helpers.NetworkSpecFromK0sConfig(k0sCfg)
	networkSpec.NetworkInterface = flags.networkInterface
	if flags.cidrConfig.GlobalCIDR != nil {
		networkSpec.GlobalCIDR = *flags.cidrConfig.GlobalCIDR
	}
	return networkSpec, nil
}

// Hop: buildKubernetesInstallation sets kubernetes installation values from install flags
func buildKubernetesInstallation(flags *installFlags, ki kubernetesinstallation.Installation) error {
	ki.SetAdminConsolePort(flags.adminConsolePort)
	ki.SetManagerPort(flags.managerPort)
	ki.SetProxySpec(flags.proxySpec)
	ki.SetKubernetesEnvSettings(flags.kubernetesEnvSettings)

	return nil
}

// Hop: buildMetricsReporter builds the metrics reporter for installation tracking
func buildMetricsReporter(cmd *cobra.Command, installCfg *installConfig) *installReporter {
	return newInstallReporter(
		replicatedAppURL(), cmd.CalledAs(), flagsToStringSlice(cmd.Flags()),
		installCfg.license.Spec.LicenseID, installCfg.clusterID, installCfg.license.Spec.AppSlug,
	)
}

// Hop: buildAPIOptions builds API server options from install flags, config, and other dependencies
func buildAPIOptions(flags installFlags, installCfg *installConfig, rc runtimeconfig.RuntimeConfig, ki kubernetesinstallation.Installation, metricsReporter metrics.ReporterInterface) (apiOptions, error) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(flags.adminConsolePassword), 10)
	if err != nil {
		return apiOptions{}, fmt.Errorf("generate password hash: %w", err)
	}

	var configValues apitypes.AppConfigValues
	if installCfg.configValues != nil {
		configValues = apitypes.ConvertToAppConfigValues(installCfg.configValues)
	}

	return apiOptions{
		APIConfig: apitypes.APIConfig{
			InstallTarget: apitypes.InstallTarget(flags.target),
			Password:      flags.adminConsolePassword,
			PasswordHash:  passwordHash,
			TLSConfig: apitypes.TLSConfig{
				CertBytes: installCfg.tlsCertBytes,
				KeyBytes:  installCfg.tlsKeyBytes,
				Hostname:  flags.hostname,
			},
			License:              installCfg.licenseBytes,
			AirgapBundle:         flags.airgapBundle,
			AirgapMetadata:       installCfg.airgapMetadata,
			EmbeddedAssetsSize:   installCfg.embeddedAssetsSize,
			ConfigValues:         configValues,
			ReleaseData:          release.GetReleaseData(),
			EndUserConfig:        installCfg.endUserConfig,
			ClusterID:            installCfg.clusterID,
			Mode:                 apitypes.ModeInstall,
			RequiresInfraUpgrade: false, // Always false for install

			LinuxConfig: apitypes.LinuxConfig{
				RuntimeConfig:             rc,
				AllowIgnoreHostPreflights: flags.ignoreHostPreflights,
			},
			KubernetesConfig: apitypes.KubernetesConfig{
				Installation: ki,
			},
		},

		ManagerPort:     flags.managerPort,
		Headless:        flags.headless,
		WebMode:         web.ModeInstall,
		MetricsReporter: metricsReporter,
	}, nil
}

// Hop: buildHelmClientOptions builds helm client options from install config and runtime config
func buildHelmClientOptions(installCfg *installConfig, rc runtimeconfig.RuntimeConfig) helm.HelmOptions {
	airgapChartsPath := ""
	if installCfg.isAirgap {
		airgapChartsPath = rc.EmbeddedClusterChartsSubDir()
	}

	return helm.HelmOptions{
		HelmPath:              rc.PathToEmbeddedClusterBinary("helm"),
		KubernetesEnvSettings: rc.GetKubernetesEnvSettings(),
		K8sVersion:            versions.K0sVersion,
		AirgapPath:            airgapChartsPath,
	}
}

// Hop: buildKotsInstallOptions builds kots install options from config and flags
func buildKotsInstallOptions(installCfg *installConfig, flags installFlags, kotsadmNamespace string, loading *spinner.MessageWriter) kotscli.InstallOptions {
	return kotscli.InstallOptions{
		AppSlug:               installCfg.license.Spec.AppSlug,
		License:               installCfg.licenseBytes,
		Namespace:             kotsadmNamespace,
		ClusterID:             installCfg.clusterID,
		AirgapBundle:          flags.airgapBundle,
		ConfigValuesFile:      flags.configValues,
		ReplicatedAppEndpoint: replicatedAppURL(),
		SkipPreflights:        flags.ignoreAppPreflights,
		Stdout:                loading,
	}
}

// Hop: buildAddonInstallOpts builds addon installation options from config, flags, and runtime config
func buildAddonInstallOpts(flags installFlags, installCfg *installConfig, rc runtimeconfig.RuntimeConfig, kotsadmNamespace string, loading **spinner.MessageWriter) *addons.InstallOptions {
	var embCfgSpec *ecv1beta1.ConfigSpec
	if embCfg := release.GetEmbeddedClusterConfig(); embCfg != nil {
		embCfgSpec = &embCfg.Spec
	}

	var euCfgSpec *ecv1beta1.ConfigSpec
	if installCfg.endUserConfig != nil {
		euCfgSpec = &installCfg.endUserConfig.Spec
	}

	return &addons.InstallOptions{
		ClusterID:               installCfg.clusterID,
		AdminConsolePwd:         flags.adminConsolePassword,
		AdminConsolePort:        rc.AdminConsolePort(),
		License:                 installCfg.license,
		IsAirgap:                flags.airgapBundle != "",
		TLSCertBytes:            installCfg.tlsCertBytes,
		TLSKeyBytes:             installCfg.tlsKeyBytes,
		Hostname:                flags.hostname,
		DisasterRecoveryEnabled: installCfg.license.Spec.IsDisasterRecoverySupported,
		IsMultiNodeEnabled:      installCfg.license.Spec.IsEmbeddedClusterMultiNodeEnabled,
		EmbeddedConfigSpec:      embCfgSpec,
		EndUserConfigSpec:       euCfgSpec,
		ProxySpec:               rc.ProxySpec(),
		HostCABundlePath:        rc.HostCABundlePath(),
		KotsadmNamespace:        kotsadmNamespace,
		DataDir:                 rc.EmbeddedClusterHomeDirectory(),
		K0sDataDir:              rc.EmbeddedClusterK0sSubDir(),
		OpenEBSDataDir:          rc.EmbeddedClusterOpenEBSLocalSubDir(),
		ServiceCIDR:             rc.ServiceCIDR(),
		KotsInstaller: func() error {
			opts := buildKotsInstallOptions(installCfg, flags, kotsadmNamespace, *loading)
			return kotscli.Install(opts)
		},
	}
}

// Hop: buildK0sConfig builds k0s cluster configuration from install flags and config
func buildK0sConfig(flags *installFlags, installCfg *installConfig) (*k0sv1beta1.ClusterConfig, error) {
	return k0s.NewK0sConfig(flags.networkInterface, installCfg.isAirgap, flags.cidrConfig.PodCIDR, flags.cidrConfig.ServiceCIDR, installCfg.endUserConfig, nil)
}

// Hop: buildRecordInstallationOptions builds the options for recording an installation
func buildRecordInstallationOptions(installCfg *installConfig, rc runtimeconfig.RuntimeConfig) kubeutils.RecordInstallationOptions {
	// get the embedded cluster config
	cfg := release.GetEmbeddedClusterConfig()
	var cfgspec *ecv1beta1.ConfigSpec
	if cfg != nil {
		cfgspec = &cfg.Spec
	}

	// extract airgap uncompressed size if airgap info is provided
	var airgapUncompressedSize int64
	if installCfg.airgapMetadata != nil && installCfg.airgapMetadata.AirgapInfo != nil {
		airgapUncompressedSize = installCfg.airgapMetadata.AirgapInfo.Spec.UncompressedSize
	}

	return kubeutils.RecordInstallationOptions{
		ClusterID:              installCfg.clusterID,
		IsAirgap:               installCfg.isAirgap,
		License:                installCfg.license,
		ConfigSpec:             cfgspec,
		MetricsBaseURL:         replicatedAppURL(),
		RuntimeConfig:          rc.Get(),
		EndUserConfig:          installCfg.endUserConfig,
		AirgapUncompressedSize: airgapUncompressedSize,
	}
}
