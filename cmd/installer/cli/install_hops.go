package cli

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/goods"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/replicatedhq/embedded-cluster/pkg-new/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg-new/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg-new/tlsutils"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/configutils"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/replicatedhq/embedded-cluster/web"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

// Hop: buildInstallFlags maps cobra command flags to install flags
func buildInstallFlags(cmd *cobra.Command, flags *installFlags) error {
	// Target defaulting (if not V3)
	if !isV3Enabled() {
		flags.target = "linux"
	}

	// Target validation
	if flags.target != "linux" && flags.target != "kubernetes" {
		return fmt.Errorf(`invalid --target (must be one of: "linux", "kubernetes")`)
	}

	// If only one of cert or key is provided, return an error
	if (flags.tlsCertFile != "" && flags.tlsKeyFile == "") || (flags.tlsCertFile == "" && flags.tlsKeyFile != "") {
		return fmt.Errorf("both --tls-cert and --tls-key must be provided together")
	}

	// Skip host preflights from env var (if flag not explicitly set)
	if !cmd.Flags().Changed("skip-host-preflights") {
		if os.Getenv("SKIP_HOST_PREFLIGHTS") == "1" || os.Getenv("SKIP_HOST_PREFLIGHTS") == "true" {
			flags.skipHostPreflights = true
		}
	}

	// Network interface auto-detection (if not provided)
	if flags.networkInterface == "" && flags.target == "linux" {
		autoInterface, err := newconfig.DetermineBestNetworkInterface()
		if err == nil {
			flags.networkInterface = autoInterface
		}
		// If error, leave empty and validation will catch it later
	}

	// Port conflict validations
	if flags.managerPort != 0 && flags.adminConsolePort != 0 {
		if flags.managerPort == flags.adminConsolePort {
			return fmt.Errorf("manager port cannot be the same as admin console port")
		}
	}

	if flags.localArtifactMirrorPort != 0 && flags.adminConsolePort != 0 {
		if flags.localArtifactMirrorPort == flags.adminConsolePort {
			return fmt.Errorf("local artifact mirror port cannot be the same as admin console port")
		}
	}

	// CIDR configuration
	cidrCfg, err := cidrConfigFromCmd(cmd)
	if err != nil {
		return err
	}
	flags.cidrConfig = cidrCfg

	// Proxy configuration
	proxy, err := parseProxyFlags(cmd, flags.networkInterface, flags.cidrConfig)
	if err != nil {
		return err
	}
	flags.proxySpec = proxy

	// Headless installation validation
	if isV3Enabled() && flags.headless {
		if err := validateHeadlessInstallFlags(flags); err != nil {
			return err
		}
	}

	return nil
}

func validateHeadlessInstallFlags(flags *installFlags) error {
	if flags.configValues == "" {
		return fmt.Errorf("--config-values flag is required for headless installation")
	}

	if flags.adminConsolePassword == "" {
		return fmt.Errorf("--admin-console-password flag is required for headless installation")
	}

	if flags.target != string(apitypes.InstallTargetLinux) {
		return fmt.Errorf("headless installation only supports --target=linux (got: %s)", flags.target)
	}

	return nil
}

// Hop: buildInstallConfig builds the install config from install flags
func buildInstallConfig(flags *installFlags) (*installConfig, error) {
	installCfg := &installConfig{
		clusterID:               uuid.New().String(),
		enableManagerExperience: isV3Enabled(),
	}

	// License file reading
	if flags.licenseFile != "" {
		b, err := os.ReadFile(flags.licenseFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read license file: %w", err)
		}
		installCfg.licenseBytes = b

		// validate the license is indeed a license file
		l, err := helpers.ParseLicenseFromBytes(b)
		if err != nil {
			var notALicenseFileErr helpers.ErrNotALicenseFile
			if errors.As(err, &notALicenseFileErr) {
				return nil, fmt.Errorf("failed to parse the license file at %q, please ensure it is not corrupt: %w", flags.licenseFile, err)
			}

			return nil, fmt.Errorf("failed to parse license file: %w", err)
		}
		installCfg.license = l
	}

	// Config values validation
	if flags.configValues != "" {
		err := configutils.ValidateKotsConfigValues(flags.configValues)
		if err != nil {
			return nil, fmt.Errorf("config values file is not valid: %w", err)
		}

		// Parse the config values file
		cv, err := helpers.ParseConfigValues(flags.configValues)
		if err != nil {
			return nil, fmt.Errorf("failed to parse config values file: %w", err)
		}
		installCfg.configValues = cv
	}

	// Airgap detection and metadata
	installCfg.isAirgap = flags.airgapBundle != ""
	if flags.airgapBundle != "" {
		metadata, err := airgap.AirgapMetadataFromPath(flags.airgapBundle)
		if err != nil {
			return nil, fmt.Errorf("failed to get airgap info: %w", err)
		}
		installCfg.airgapMetadata = metadata
	}

	// Embedded assets size
	size, err := goods.SizeOfEmbeddedAssets()
	if err != nil {
		return nil, fmt.Errorf("failed to get size of embedded files: %w", err)
	}
	installCfg.embeddedAssetsSize = size

	// End user config (overrides file)
	eucfg, err := helpers.ParseEndUserConfig(flags.overrides)
	if err != nil {
		return nil, fmt.Errorf("process overrides file: %w", err)
	}
	installCfg.endUserConfig = eucfg

	// TLS Certificate Processing
	if err := processTLSConfig(flags, installCfg); err != nil {
		return nil, fmt.Errorf("process TLS config: %w", err)
	}

	return installCfg, nil
}

func cidrConfigFromCmd(cmd *cobra.Command) (*newconfig.CIDRConfig, error) {
	if err := validateCIDRFlags(cmd); err != nil {
		return nil, err
	}

	// parse the various cidr flags to make sure we have exactly what we want
	cidrCfg, err := getCIDRConfig(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to determine pod and service CIDRs: %w", err)
	}

	return cidrCfg, nil
}

func processTLSConfig(flags *installFlags, installCfg *installConfig) error {
	// If both cert and key are provided, load them
	if flags.tlsCertFile != "" && flags.tlsKeyFile != "" {
		certBytes, err := os.ReadFile(flags.tlsCertFile)
		if err != nil {
			return fmt.Errorf("failed to read TLS certificate: %w", err)
		}
		keyBytes, err := os.ReadFile(flags.tlsKeyFile)
		if err != nil {
			return fmt.Errorf("failed to read TLS key: %w", err)
		}

		cert, err := tls.X509KeyPair(certBytes, keyBytes)
		if err != nil {
			return fmt.Errorf("failed to parse TLS certificate: %w", err)
		}

		installCfg.tlsCert = cert
		installCfg.tlsCertBytes = certBytes
		installCfg.tlsKeyBytes = keyBytes
	} else if installCfg.enableManagerExperience {
		// For manager experience, generate self-signed cert if none provided, with user confirmation
		logrus.Warn("\nNo certificate files provided. A self-signed certificate will be used, and your browser will show a security warning.")
		logrus.Info("To use your own certificate, provide both --tls-key and --tls-cert flags.")

		if !flags.assumeYes {
			logrus.Info("") // newline so the prompt is separated from the warning
			confirmed, err := prompts.New().Confirm("Do you want to continue with a self-signed certificate?", false)
			if err != nil {
				return fmt.Errorf("failed to get confirmation: %w", err)
			}
			if !confirmed {
				logrus.Info("Installation cancelled. Please run the command again with the --tls-key and --tls-cert flags or use the --yes flag to continue with a self-signed certificate.\n")
				return fmt.Errorf("installation cancelled by user")
			}
		}

		// Get all IP addresses for the certificate
		ipAddresses, err := netutils.ListAllValidIPAddresses()
		if err != nil {
			return fmt.Errorf("failed to list all valid IP addresses: %w", err)
		}

		// Determine the namespace for the certificate
		kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(context.Background(), nil)
		if err != nil {
			return fmt.Errorf("get kotsadm namespace: %w", err)
		}

		// Generate self-signed certificate
		cert, certData, keyData, err := tlsutils.GenerateCertificate(flags.hostname, ipAddresses, kotsadmNamespace)
		if err != nil {
			return fmt.Errorf("generate tls certificate: %w", err)
		}
		installCfg.tlsCert = cert
		installCfg.tlsCertBytes = certData
		installCfg.tlsKeyBytes = keyData
	}

	return nil
}

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
