package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2"
	"github.com/replicatedhq/embedded-cluster/pkg/configutils"
	"github.com/replicatedhq/embedded-cluster/pkg/extensions"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/embedded-cluster/pkg/support"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Install2CmdFlags struct {
	adminConsolePassword    string
	adminConsolePort        int
	airgapBundle            string
	dataDir                 string
	licenseFile             string
	localArtifactMirrorPort int
	assumeYes               bool
	overrides               string
	privateCAs              []string
	skipHostPreflights      bool
	ignoreHostPreflights    bool
	configValues            string

	networkInterface               string
	isAutoSelectedNetworkInterface bool
	autoSelectNetworkInterfaceErr  error

	license *kotsv1beta1.License
	proxy   *ecv1beta1.ProxySpec
	cidrCfg *CIDRConfig
}

// Install2Cmd returns a cobra command for installing the embedded cluster.
// This is the upcoming version of install without the operator and where
// install does all of the work. This is a hidden command until it's tested
// and ready.
func Install2Cmd(ctx context.Context, name string) *cobra.Command {
	var flags Install2CmdFlags

	cmd := &cobra.Command{
		Use:           "install2",
		Short:         fmt.Sprintf("Experimental installer for %s", name),
		Hidden:        true,
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("install command must be run as root")
			}

			if flags.skipHostPreflights {
				logrus.Warnf("Warning: --skip-host-preflights is deprecated and will be removed in a later version. Use --ignore-host-preflights instead.")
			}

			p, err := parseProxyFlags(cmd)
			if err != nil {
				return err
			}
			flags.proxy = p

			if err := validateCIDRFlags(cmd); err != nil {
				return err
			}

			// parse the various cidr flags to make sure we have exactly what we want
			cidrCfg, err := getCIDRConfig(cmd)
			if err != nil {
				return fmt.Errorf("unable to determine pod and service CIDRs: %w", err)
			}
			flags.cidrCfg = cidrCfg

			// if a network interface flag was not provided, attempt to discover it
			if flags.networkInterface == "" {
				autoInterface, err := determineBestNetworkInterface()
				if err != nil {
					flags.autoSelectNetworkInterfaceErr = err
				} else {
					flags.isAutoSelectedNetworkInterface = true
					flags.networkInterface = autoInterface
				}
			}

			// validate the the license is indeed a license file
			l, err := helpers.ParseLicense(flags.licenseFile)
			if err != nil {
				if err == helpers.ErrNotALicenseFile {
					return fmt.Errorf("license file is not a valid license file")
				}

				return fmt.Errorf("unable to parse license file: %w", err)
			}
			flags.license = l

			runtimeconfig.ApplyFlags(cmd.Flags())
			os.Setenv("TMPDIR", runtimeconfig.EmbeddedClusterTmpSubDir())

			if err := runtimeconfig.WriteToDisk(); err != nil {
				return fmt.Errorf("unable to write runtime config to disk: %w", err)
			}

			if os.Getenv("DISABLE_TELEMETRY") != "" {
				metrics.DisableMetrics()
			}

			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			runtimeconfig.Cleanup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runInstall2(cmd, args, name, flags); err != nil {
				return err
			}

			return nil

		},
	}

	cmd.Flags().StringVar(&flags.adminConsolePassword, "admin-console-password", "", "Password for the Admin Console")
	cmd.Flags().IntVar(&flags.adminConsolePort, "admin-console-port", ecv1beta1.DefaultAdminConsolePort, "Port on which the Admin Console will be served")
	cmd.Flags().StringVar(&flags.airgapBundle, "airgap-bundle", "", "Path to the air gap bundle. If set, the installation will complete without internet access.")
	cmd.Flags().StringVar(&flags.dataDir, "data-dir", ecv1beta1.DefaultDataDir, "Path to the data directory")
	cmd.Flags().StringVarP(&flags.licenseFile, "license", "l", "", "Path to the license file")
	cmd.Flags().IntVar(&flags.localArtifactMirrorPort, "local-artifact-mirror-port", ecv1beta1.DefaultLocalArtifactMirrorPort, "Port on which the Local Artifact Mirror will be served")
	cmd.Flags().StringVar(&flags.networkInterface, "network-interface", "", "The network interface to use for the cluster")
	cmd.Flags().BoolVarP(&flags.assumeYes, "yes", "y", false, "Assume yes to all prompts.")

	cmd.Flags().StringVar(&flags.overrides, "overrides", "", "File with an EmbeddedClusterConfig object to override the default configuration")
	cmd.Flags().MarkHidden("overrides")

	cmd.Flags().StringSliceVar(&flags.privateCAs, "private-ca", []string{}, "Path to a trusted private CA certificate file")

	cmd.Flags().BoolVar(&flags.skipHostPreflights, "skip-host-preflights", false, "Skip host preflight checks. This is not recommended and has been deprecated.")
	cmd.Flags().MarkHidden("skip-host-preflights")
	cmd.Flags().MarkDeprecated("skip-host-preflights", "This flag is deprecated and will be removed in a future version. Use --ignore-host-preflights instead.")

	cmd.Flags().BoolVar(&flags.ignoreHostPreflights, "ignore-host-preflights", false, "Allow bypassing host preflight failures")
	cmd.Flags().StringVar(&flags.configValues, "config-values", "", "Path to the config values to use when installing")

	addProxyFlags(cmd)
	addCIDRFlags(cmd)
	cmd.Flags().SetNormalizeFunc(normalizeNoPromptToYes)

	return cmd
}

func runInstall2(cmd *cobra.Command, args []string, name string, flags Install2CmdFlags) error {
	if err := runInstallVerifyAndPrompt(cmd.Context(), name, &flags); err != nil {
		return err
	}

	logrus.Debugf("materializing binaries")
	if err := materializeFiles(flags.airgapBundle); err != nil {
		metrics.ReportApplyFinished(cmd.Context(), "", flags.license, err)
		return err
	}

	logrus.Debugf("configuring sysctl")
	if err := configutils.ConfigureSysctl(); err != nil {
		return fmt.Errorf("unable to configure sysctl: %w", err)
	}

	logrus.Debugf("configuring network manager")
	if err := configureNetworkManager(cmd.Context()); err != nil {
		return fmt.Errorf("unable to configure network manager: %w", err)
	}

	var replicatedAPIURL, proxyRegistryURL string
	if flags.license != nil {
		replicatedAPIURL = flags.license.Spec.Endpoint
		proxyRegistryURL = fmt.Sprintf("https://%s", runtimeconfig.ProxyRegistryAddress)
	}

	logrus.Debugf("running host preflights")
	if err := preflights.PrepareAndRun(cmd.Context(), preflights.PrepareAndRunOptions{
		ReplicatedAPIURL:     replicatedAPIURL,
		ProxyRegistryURL:     proxyRegistryURL,
		Proxy:                flags.proxy,
		PodCIDR:              flags.cidrCfg.PodCIDR,
		ServiceCIDR:          flags.cidrCfg.ServiceCIDR,
		GlobalCIDR:           flags.cidrCfg.GlobalCIDR,
		PrivateCAs:           flags.privateCAs,
		IsAirgap:             flags.airgapBundle != "",
		SkipHostPreflights:   flags.skipHostPreflights,
		IgnoreHostPreflights: flags.ignoreHostPreflights,
		AssumeYes:            flags.assumeYes,
	}); err != nil {
		if err == preflights.ErrPreflightsHaveFail {
			// we exit and not return an error to prevent the error from being printed to stderr
			// we already handled the output
			os.Exit(1)
			return nil
		}
		return fmt.Errorf("unable to prepare and run preflights: %w", err)
	}

	k0sCfg, err := installAndStartCluster(cmd.Context(), flags.networkInterface, flags.airgapBundle, flags.license, flags.proxy, flags.cidrCfg, flags.overrides)
	if err != nil {
		metrics.ReportApplyFinished(cmd.Context(), "", flags.license, err)
		return err
	}

	disasterRecoveryEnabled, err := helpers.DisasterRecoveryEnabled(flags.license)
	if err != nil {
		return fmt.Errorf("unable to check if disaster recovery is enabled: %w", err)
	}

	installObject, err := recordInstallation(cmd.Context(), flags, k0sCfg, disasterRecoveryEnabled)
	if err != nil {
		metrics.ReportApplyFinished(cmd.Context(), "", flags.license, err)
		return err
	}

	// TODO (@salah): update installation status to reflect what's happening

	logrus.Debugf("installing addons")
	if err := addons2.Install(cmd.Context(), addons2.InstallOptions{
		AdminConsolePwd:         flags.adminConsolePassword,
		License:                 flags.license,
		LicenseFile:             flags.licenseFile,
		AirgapBundle:            flags.airgapBundle,
		Proxy:                   flags.proxy,
		PrivateCAs:              flags.privateCAs,
		ConfigValuesFile:        flags.configValues,
		ServiceCIDR:             flags.cidrCfg.ServiceCIDR,
		DisasterRecoveryEnabled: disasterRecoveryEnabled,
		KotsInstaller: func(msg *spinner.MessageWriter) error {
			opts := kotscli.InstallOptions{
				AppSlug:          flags.license.Spec.AppSlug,
				LicenseFile:      flags.licenseFile,
				Namespace:        runtimeconfig.KotsadmNamespace,
				AirgapBundle:     flags.airgapBundle,
				ConfigValuesFile: flags.configValues,
			}
			return kotscli.Install(opts, msg)
		},
	}); err != nil {
		metrics.ReportApplyFinished(cmd.Context(), "", flags.license, err)
		return err
	}

	logrus.Debugf("installing extensions")
	if err := extensions.Install(cmd.Context(), flags.airgapBundle != ""); err != nil {
		metrics.ReportApplyFinished(cmd.Context(), "", flags.license, err)
		return err
	}

	logrus.Debugf("installing manager")
	if err := installAndEnableManager(cmd.Context()); err != nil {
		metrics.ReportApplyFinished(cmd.Context(), "", flags.license, err)
		return err
	}

	// mark that the installation is installed as everything has been applied
	installObject.Status.State = ecv1beta1.InstallationStateInstalled
	if err := updateInstallation(cmd.Context(), installObject); err != nil {
		metrics.ReportApplyFinished(cmd.Context(), "", flags.license, err)
		return err
	}

	if err = support.CreateHostSupportBundle(); err != nil {
		logrus.Warnf("unable to create host support bundle: %v", err)
	}

	if err := printSuccessMessage(flags.license, flags.networkInterface); err != nil {
		metrics.ReportApplyFinished(cmd.Context(), "", flags.license, err)
		return err
	}

	return nil
}

func runInstallVerifyAndPrompt(ctx context.Context, name string, flags *Install2CmdFlags) error {
	logrus.Debugf("checking if %s is already installed", name)
	installed, err := k0s.IsInstalled()
	if err != nil {
		return err
	}
	if installed {
		logrus.Errorf("An installation has been detected on this machine.")
		logrus.Infof("If you want to reinstall, you need to remove the existing installation first.")
		logrus.Infof("You can do this by running the following command:")
		logrus.Infof("\n  sudo ./%s reset\n", name)
		os.Exit(1)
	}

	channelRelease, err := release.GetChannelRelease()
	if err != nil {
		return fmt.Errorf("unable to read channel release data: %w", err)
	}

	if channelRelease != nil && channelRelease.Airgap && flags.airgapBundle == "" && !flags.assumeYes {
		logrus.Warnf("You downloaded an air gap bundle but didn't provide it with --airgap-bundle.")
		logrus.Warnf("If you continue, the installation will not use an air gap bundle and will connect to the internet.")
		if !prompts.New().Confirm("Do you want to proceed with an online installation?", false) {
			return ErrNothingElseToAdd
		}
	}

	metrics.ReportApplyStarted(ctx, flags.licenseFile)

	logrus.Debugf("checking license matches")
	license, err := getLicenseFromFilepath(flags.licenseFile)
	if err != nil {
		metricErr := fmt.Errorf("unable to get license: %w", err)
		metrics.ReportApplyFinished(ctx, "", flags.license, metricErr)
		return err // do not return the metricErr, as we want the user to see the error message without a prefix
	}
	isAirgap := false
	if flags.airgapBundle != "" {
		isAirgap = true
	}
	if isAirgap {
		logrus.Debugf("checking airgap bundle matches binary")
		if err := checkAirgapMatches(flags.airgapBundle); err != nil {
			return err // we want the user to see the error message without a prefix
		}
	}

	if !isAirgap {
		if err := maybePromptForAppUpdate(ctx, prompts.New(), license, flags.assumeYes); err != nil {
			if errors.Is(err, ErrNothingElseToAdd) {
				metrics.ReportApplyFinished(ctx, "", flags.license, err)
				return err
			}
			// If we get an error other than ErrNothingElseToAdd, we warn and continue as
			// this check is not critical.
			logrus.Debugf("WARNING: Failed to check for newer app versions: %v", err)
		}
	}

	if err := preflights.ValidateApp(); err != nil {
		metrics.ReportApplyFinished(ctx, "", flags.license, err)
		return err
	}

	if flags.adminConsolePassword != "" {
		if !validateAdminConsolePassword(flags.adminConsolePassword, flags.adminConsolePassword) {
			return fmt.Errorf("unable to set the Admin Console password")
		}
	} else {
		// no password was provided
		if flags.assumeYes {
			logrus.Infof("The Admin Console password is set to %s", "password")
			flags.adminConsolePassword = "password"
		} else {
			maxTries := 3
			for i := 0; i < maxTries; i++ {
				promptA := prompts.New().Password(fmt.Sprintf("Set the Admin Console password (minimum %d characters):", minAdminPasswordLength))
				promptB := prompts.New().Password("Confirm the Admin Console password:")

				if validateAdminConsolePassword(promptA, promptB) {
					flags.adminConsolePassword = promptA
					break
				}
			}
		}
	}
	if flags.adminConsolePassword == "" {
		err := fmt.Errorf("no admin console password")
		metrics.ReportApplyFinished(ctx, "", flags.license, err)
		return err
	}

	return nil
}

func installAndStartCluster(ctx context.Context, networkInterface string, airgapBundle string, license *kotsv1beta1.License, proxy *ecv1beta1.ProxySpec, cidrCfg *CIDRConfig, overrides string) (*k0sconfig.ClusterConfig, error) {
	loading := spinner.Start()
	defer loading.Close()
	loading.Infof("Installing %s node", runtimeconfig.BinaryName())
	logrus.Debugf("creating k0s configuration file")

	cfg, err := k0s.WriteK0sConfig(ctx, networkInterface, airgapBundle, cidrCfg.PodCIDR, cidrCfg.ServiceCIDR, overrides)
	if err != nil {
		err := fmt.Errorf("unable to create config file: %w", err)
		metrics.ReportApplyFinished(ctx, "", license, err)
		return nil, err
	}
	logrus.Debugf("creating systemd unit files")
	if err := createSystemdUnitFiles(false, proxy); err != nil {
		err := fmt.Errorf("unable to create systemd unit files: %w", err)
		metrics.ReportApplyFinished(ctx, "", license, err)
		return nil, err
	}

	logrus.Debugf("installing k0s")
	if err := k0s.Install(networkInterface); err != nil {
		err := fmt.Errorf("unable to install cluster: %w", err)
		metrics.ReportApplyFinished(ctx, "", license, err)
		return nil, err
	}
	loading.Infof("Waiting for %s node to be ready", runtimeconfig.BinaryName())
	logrus.Debugf("waiting for k0s to be ready")
	if err := waitForK0s(); err != nil {
		err := fmt.Errorf("unable to wait for node: %w", err)
		metrics.ReportApplyFinished(ctx, "", license, err)
		return nil, err
	}

	// init the kubeconfig
	os.Setenv("KUBECONFIG", runtimeconfig.PathToKubeConfig())

	loading.Infof("Node installation finished!")
	return cfg, nil
}

func recordInstallation(ctx context.Context, flags Install2CmdFlags, k0sCfg *k0sv1beta1.ClusterConfig, disasterRecoveryEnabled bool) (*ecv1beta1.Installation, error) {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return nil, fmt.Errorf("create kube client: %w", err)
	}

	if err := createECNamespace(ctx, kcli); err != nil {
		return nil, fmt.Errorf("create embedded-cluster namespace: %w", err)
	}

	cfg, err := release.GetEmbeddedClusterConfig()
	if err != nil {
		return nil, err
	}
	var cfgspec *ecv1beta1.ConfigSpec
	if cfg != nil {
		cfgspec = &cfg.Spec
	}

	var euOverrides string
	if flags.overrides != "" {
		eucfg, err := helpers.ParseEndUserConfig(flags.overrides)
		if err != nil {
			return nil, fmt.Errorf("process overrides file: %w", err)
		}
		if eucfg != nil {
			euOverrides = eucfg.Spec.UnsupportedOverrides.K0s
		}
	}

	installation := ecv1beta1.Installation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: ecv1beta1.GroupVersion.String(),
			Kind:       "Installation",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("embedded-cluster-installation-%s", time.Now().Format("20060102150405")),
		},
		Spec: ecv1beta1.InstallationSpec{
			ClusterID:                 metrics.ClusterID().String(),
			MetricsBaseURL:            metrics.BaseURL(flags.license),
			AirGap:                    flags.airgapBundle != "",
			Proxy:                     flags.proxy,
			Network:                   networkSpecFromK0sConfig(k0sCfg),
			Config:                    cfgspec,
			RuntimeConfig:             runtimeconfig.Get(),
			EndUserK0sConfigOverrides: euOverrides,
			BinaryName:                runtimeconfig.BinaryName(),
			SourceType:                ecv1beta1.InstallationSourceTypeConfigMap,
			LicenseInfo: &ecv1beta1.LicenseInfo{
				IsDisasterRecoverySupported: disasterRecoveryEnabled,
			},
		},
		Status: ecv1beta1.InstallationStatus{
			State: ecv1beta1.InstallationStateKubernetesInstalled,
		},
	}
	if err := kubeutils.CreateInstallation(ctx, kcli, &installation); err != nil {
		return nil, fmt.Errorf("create installation: %w", err)
	}

	return &installation, nil
}

func updateInstallation(ctx context.Context, install *ecv1beta1.Installation) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("create kube client: %w", err)
	}

	if err := kubeutils.UpdateInstallation(ctx, kcli, install); err != nil {
		return fmt.Errorf("Failed to update installation")
	}
	return nil
}

func createECNamespace(ctx context.Context, kcli client.Client) error {
	ns := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "embedded-cluster",
		},
	}
	if err := kcli.Create(ctx, &ns); err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func networkSpecFromK0sConfig(k0sCfg *k0sv1beta1.ClusterConfig) *ecv1beta1.NetworkSpec {
	network := &ecv1beta1.NetworkSpec{}

	if k0sCfg.Spec != nil && k0sCfg.Spec.Network != nil {
		network.PodCIDR = k0sCfg.Spec.Network.PodCIDR
		network.ServiceCIDR = k0sCfg.Spec.Network.ServiceCIDR
	}

	if k0sCfg.Spec.API != nil {
		if val, ok := k0sCfg.Spec.API.ExtraArgs["service-node-port-range"]; ok {
			network.NodePortRange = val
		}
	}

	return network
}

func printSuccessMessage(license *kotsv1beta1.License, networkInterface string) error {
	adminConsoleURL := getAdminConsoleURL(networkInterface, runtimeconfig.AdminConsolePort())

	successColor := "\033[32m"
	colorReset := "\033[0m"
	var successMessage string
	if license != nil {
		successMessage = fmt.Sprintf("Visit the Admin Console to configure and install %s: %s%s%s",
			license.Spec.AppSlug, successColor, adminConsoleURL, colorReset,
		)
	} else {
		successMessage = fmt.Sprintf("Visit the Admin Console to configure and install your application: %s%s%s",
			successColor, adminConsoleURL, colorReset,
		)
	}
	logrus.Info(successMessage)

	return nil
}

func getAdminConsoleURL(networkInterface string, port int) string {
	ipaddr := runtimeconfig.TryDiscoverPublicIP()
	if ipaddr == "" {
		var err error
		ipaddr, err = netutils.FirstValidAddress(networkInterface)
		if err != nil {
			logrus.Errorf("unable to determine node IP address: %v", err)
			ipaddr = "NODE-IP-ADDRESS"
		}
	}
	return fmt.Sprintf("http://%s:%v", ipaddr, port)
}
