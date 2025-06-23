package cli

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/AlecAivazis/survey/v2/terminal"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/cloudutils"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg-new/k0s"
	ecmetadata "github.com/replicatedhq/embedded-cluster/pkg-new/metadata"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg-new/tlsutils"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	addontypes "github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/configutils"
	"github.com/replicatedhq/embedded-cluster/pkg/extensions"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/embedded-cluster/pkg/support"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type InstallCmdFlags struct {
	adminConsolePassword    string
	adminConsolePort        int
	airgapBundle            string
	isAirgap                bool
	dataDir                 string
	licenseFile             string
	localArtifactMirrorPort int
	assumeYes               bool
	overrides               string
	skipHostPreflights      bool
	ignoreHostPreflights    bool
	configValues            string
	networkInterface        string

	// guided UI flags
	enableManagerExperience bool
	managerPort             int
	tlsCertFile             string
	tlsKeyFile              string
	hostname                string

	// TODO: move to substruct
	license      *kotsv1beta1.License
	licenseBytes []byte
	tlsCert      tls.Certificate
	tlsCertBytes []byte
	tlsKeyBytes  []byte
}

// webAssetsFS is the filesystem to be used by the web component. Defaults to nil allowing the web server to use the default assets embedded in the binary. Useful for testing.
var webAssetsFS fs.FS = nil

// InstallCmd returns a cobra command for installing the embedded cluster.
func InstallCmd(ctx context.Context, name string) *cobra.Command {
	var flags InstallCmdFlags

	ctx, cancel := context.WithCancel(ctx)
	rc := runtimeconfig.New(nil)

	cmd := &cobra.Command{
		Use:   "install",
		Short: fmt.Sprintf("Install %s", name),
		PostRun: func(cmd *cobra.Command, args []string) {
			rc.Cleanup()
			cancel() // Cancel context when command completes
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var airgapInfo *kotsv1beta1.Airgap
			if flags.airgapBundle != "" {
				var err error
				airgapInfo, err = airgap.AirgapInfoFromPath(flags.airgapBundle)
				if err != nil {
					return fmt.Errorf("failed to get airgap info: %w", err)
				}
			}

			if err := verifyAndPrompt(ctx, name, flags, prompts.New(), airgapInfo); err != nil {
				return err
			}
			if err := preRunInstall(cmd, &flags, rc); err != nil {
				return err
			}

			if flags.enableManagerExperience {
				return runManagerExperienceInstall(ctx, flags, rc, airgapInfo)
			}

			_ = rc.SetEnv()

			clusterID := metrics.ClusterID()
			installReporter := newInstallReporter(
				replicatedAppURL(), clusterID, cmd.CalledAs(), flagsToStringSlice(cmd.Flags()),
				flags.license.Spec.LicenseID, flags.license.Spec.AppSlug,
			)
			installReporter.ReportInstallationStarted(ctx)

			// Setup signal handler with the metrics reporter cleanup function
			signalHandler(ctx, cancel, func(ctx context.Context, sig os.Signal) {
				installReporter.ReportSignalAborted(ctx, sig)
			})

			if err := runInstall(cmd.Context(), flags, rc, installReporter, airgapInfo); err != nil {
				// Check if this is an interrupt error from the terminal
				if errors.Is(err, terminal.InterruptErr) {
					installReporter.ReportSignalAborted(ctx, syscall.SIGINT)
				} else {
					installReporter.ReportInstallationFailed(ctx, err)
				}
				return err
			}
			installReporter.ReportInstallationSucceeded(ctx)

			return nil
		},
	}

	if err := addInstallFlags(cmd, &flags); err != nil {
		panic(err)
	}
	if err := addInstallAdminConsoleFlags(cmd, &flags); err != nil {
		panic(err)
	}
	if err := addManagerExperienceFlags(cmd, &flags); err != nil {
		panic(err)
	}

	cmd.AddCommand(InstallRunPreflightsCmd(ctx, name))

	return cmd
}

func addInstallFlags(cmd *cobra.Command, flags *InstallCmdFlags) error {
	cmd.Flags().StringVar(&flags.airgapBundle, "airgap-bundle", "", "Path to the air gap bundle. If set, the installation will complete without internet access.")
	cmd.Flags().StringVar(&flags.dataDir, "data-dir", ecv1beta1.DefaultDataDir, "Path to the data directory")
	cmd.Flags().IntVar(&flags.localArtifactMirrorPort, "local-artifact-mirror-port", ecv1beta1.DefaultLocalArtifactMirrorPort, "Port on which the Local Artifact Mirror will be served")
	cmd.Flags().StringVar(&flags.networkInterface, "network-interface", "", "The network interface to use for the cluster")
	cmd.Flags().BoolVarP(&flags.assumeYes, "yes", "y", false, "Assume yes to all prompts.")
	cmd.Flags().SetNormalizeFunc(normalizeNoPromptToYes)

	cmd.Flags().StringVar(&flags.overrides, "overrides", "", "File with an EmbeddedClusterConfig object to override the default configuration")
	if err := cmd.Flags().MarkHidden("overrides"); err != nil {
		return err
	}

	cmd.Flags().StringSlice("private-ca", []string{}, "Path to a trusted private CA certificate file")
	if err := cmd.Flags().MarkHidden("private-ca"); err != nil {
		return err
	}
	if err := cmd.Flags().MarkDeprecated("private-ca", "This flag is no longer used and will be removed in a future version. The CA bundle will be automatically detected from the host."); err != nil {
		return err
	}

	if err := addProxyFlags(cmd); err != nil {
		return err
	}
	if err := addCIDRFlags(cmd); err != nil {
		return err
	}

	cmd.Flags().BoolVar(&flags.skipHostPreflights, "skip-host-preflights", false, "Skip host preflight checks. This is not recommended and has been deprecated.")
	if err := cmd.Flags().MarkHidden("skip-host-preflights"); err != nil {
		return err
	}
	if err := cmd.Flags().MarkDeprecated("skip-host-preflights", "This flag is deprecated and will be removed in a future version. Use --ignore-host-preflights instead."); err != nil {
		return err
	}
	cmd.Flags().BoolVar(&flags.ignoreHostPreflights, "ignore-host-preflights", false, "Allow bypassing host preflight failures")

	return nil
}

func addInstallAdminConsoleFlags(cmd *cobra.Command, flags *InstallCmdFlags) error {
	cmd.Flags().StringVar(&flags.adminConsolePassword, "admin-console-password", "", "Password for the Admin Console")
	cmd.Flags().IntVar(&flags.adminConsolePort, "admin-console-port", ecv1beta1.DefaultAdminConsolePort, "Port on which the Admin Console will be served")
	cmd.Flags().StringVarP(&flags.licenseFile, "license", "l", "", "Path to the license file")
	if err := cmd.MarkFlagRequired("license"); err != nil {
		panic(err)
	}
	cmd.Flags().StringVar(&flags.configValues, "config-values", "", "Path to the config values to use when installing")

	return nil
}

func addManagerExperienceFlags(cmd *cobra.Command, flags *InstallCmdFlags) error {
	cmd.Flags().BoolVar(&flags.enableManagerExperience, "manager-experience", false, "Run the browser-based installation experience.")
	cmd.Flags().IntVar(&flags.managerPort, "manager-port", ecv1beta1.DefaultManagerPort, "Port on which the Manager will be served")
	cmd.Flags().StringVar(&flags.tlsCertFile, "tls-cert", "", "Path to the TLS certificate file")
	cmd.Flags().StringVar(&flags.tlsKeyFile, "tls-key", "", "Path to the TLS key file")
	cmd.Flags().StringVar(&flags.hostname, "hostname", "", "Hostname to use for TLS configuration")

	if err := cmd.Flags().MarkHidden("manager-experience"); err != nil {
		return err
	}
	if err := cmd.Flags().MarkHidden("manager-port"); err != nil {
		return err
	}
	if err := cmd.Flags().MarkHidden("tls-cert"); err != nil {
		return err
	}
	if err := cmd.Flags().MarkHidden("tls-key"); err != nil {
		return err
	}
	if err := cmd.Flags().MarkHidden("hostname"); err != nil {
		return err
	}

	return nil
}

func preRunInstall(cmd *cobra.Command, flags *InstallCmdFlags, rc runtimeconfig.RuntimeConfig) error {
	if os.Getuid() != 0 {
		return fmt.Errorf("install command must be run as root")
	}

	// set the umask to 022 so that we can create files/directories with 755 permissions
	// this does not return an error - it returns the previous umask
	_ = syscall.Umask(0o022)

	// license file can be empty for restore
	if flags.licenseFile != "" {
		b, err := os.ReadFile(flags.licenseFile)
		if err != nil {
			return fmt.Errorf("unable to read license file: %w", err)
		}
		flags.licenseBytes = b

		// validate the the license is indeed a license file
		l, err := helpers.ParseLicense(flags.licenseFile)
		if err != nil {
			if err == helpers.ErrNotALicenseFile {
				return fmt.Errorf("license file is not a valid license file")
			}

			return fmt.Errorf("unable to parse license file: %w", err)
		}
		flags.license = l
	}

	if flags.configValues != "" {
		err := configutils.ValidateKotsConfigValues(flags.configValues)
		if err != nil {
			return fmt.Errorf("config values file is not valid: %w", err)
		}
	}

	flags.isAirgap = flags.airgapBundle != ""

	hostCABundlePath, err := findHostCABundle()
	if err != nil {
		return fmt.Errorf("unable to find host CA bundle: %w", err)
	}
	logrus.Debugf("using host CA bundle: %s", hostCABundlePath)

	// if a network interface flag was not provided, attempt to discover it
	if flags.networkInterface == "" {
		autoInterface, err := newconfig.DetermineBestNetworkInterface()
		if err == nil {
			flags.networkInterface = autoInterface
		}
	}

	if flags.localArtifactMirrorPort != 0 && flags.adminConsolePort != 0 {
		if flags.localArtifactMirrorPort == flags.adminConsolePort {
			return fmt.Errorf("local artifact mirror port cannot be the same as admin console port")
		}
	}

	eucfg, err := helpers.ParseEndUserConfig(flags.overrides)
	if err != nil {
		return fmt.Errorf("process overrides file: %w", err)
	}

	proxy, err := proxyConfigFromCmd(cmd, flags.assumeYes)
	if err != nil {
		return err
	}

	cidrCfg, err := cidrConfigFromCmd(cmd)
	if err != nil {
		return err
	}

	k0sCfg, err := k0s.NewK0sConfig(flags.networkInterface, flags.isAirgap, cidrCfg.PodCIDR, cidrCfg.ServiceCIDR, eucfg, nil)
	if err != nil {
		return fmt.Errorf("unable to create k0s config: %w", err)
	}
	networkSpec := helpers.NetworkSpecFromK0sConfig(k0sCfg)
	networkSpec.NetworkInterface = flags.networkInterface
	if cidrCfg.GlobalCIDR != nil {
		networkSpec.GlobalCIDR = *cidrCfg.GlobalCIDR
	}

	// TODO: validate that a single port isn't used for multiple services
	rc.SetDataDir(flags.dataDir)
	rc.SetLocalArtifactMirrorPort(flags.localArtifactMirrorPort)
	rc.SetAdminConsolePort(flags.adminConsolePort)
	rc.SetHostCABundlePath(hostCABundlePath)
	rc.SetNetworkSpec(networkSpec)
	rc.SetProxySpec(proxy)

	// restore command doesn't have a password flag
	if cmd.Flags().Lookup("admin-console-password") != nil {
		if err := ensureAdminConsolePassword(flags); err != nil {
			return err
		}
	}

	return nil
}

func proxyConfigFromCmd(cmd *cobra.Command, assumeYes bool) (*ecv1beta1.ProxySpec, error) {
	proxy, err := parseProxyFlags(cmd)
	if err != nil {
		return nil, err
	}

	if err := verifyProxyConfig(proxy, prompts.New(), assumeYes); err != nil {
		return nil, err
	}

	return proxy, nil
}

func cidrConfigFromCmd(cmd *cobra.Command) (*newconfig.CIDRConfig, error) {
	if err := validateCIDRFlags(cmd); err != nil {
		return nil, err
	}

	// parse the various cidr flags to make sure we have exactly what we want
	cidrCfg, err := getCIDRConfig(cmd)
	if err != nil {
		return nil, fmt.Errorf("unable to determine pod and service CIDRs: %w", err)
	}

	return cidrCfg, nil
}

func runManagerExperienceInstall(ctx context.Context, flags InstallCmdFlags, rc runtimeconfig.RuntimeConfig, airgapInfo *kotsv1beta1.Airgap) (finalErr error) {
	// this is necessary because the api listens on all interfaces,
	// and we only know the interface to use when the user selects it in the ui
	ipAddresses, err := netutils.ListAllValidIPAddresses()
	if err != nil {
		return fmt.Errorf("unable to list all valid IP addresses: %w", err)
	}

	if flags.tlsCertFile == "" || flags.tlsKeyFile == "" {
		logrus.Warn("\nNo certificate files provided. A self-signed certificate will be used, and your browser will show a security warning.")
		logrus.Info("To use your own certificate, provide both --tls-key and --tls-cert flags.")

		if !flags.assumeYes {
			logrus.Info("") // newline so the prompt is separated from the warning
			confirmed, err := prompts.New().Confirm("Do you want to continue with a self-signed certificate?", false)
			if err != nil {
				return fmt.Errorf("failed to get confirmation: %w", err)
			}
			if !confirmed {
				logrus.Infof("\nInstallation cancelled. Please run the command again with the --tls-key and --tls-cert flags.\n")
				return nil
			}
		}
	}

	if flags.tlsCertFile != "" && flags.tlsKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(flags.tlsCertFile, flags.tlsKeyFile)
		if err != nil {
			return fmt.Errorf("load tls certificate: %w", err)
		}
		certData, err := os.ReadFile(flags.tlsCertFile)
		if err != nil {
			return fmt.Errorf("unable to read tls cert file: %w", err)
		}
		keyData, err := os.ReadFile(flags.tlsKeyFile)
		if err != nil {
			return fmt.Errorf("unable to read tls key file: %w", err)
		}
		flags.tlsCert = cert
		flags.tlsCertBytes = certData
		flags.tlsKeyBytes = keyData
	} else {
		cert, certData, keyData, err := tlsutils.GenerateCertificate(flags.hostname, ipAddresses)
		if err != nil {
			return fmt.Errorf("generate tls certificate: %w", err)
		}
		flags.tlsCert = cert
		flags.tlsCertBytes = certData
		flags.tlsKeyBytes = keyData
	}

	eucfg, err := helpers.ParseEndUserConfig(flags.overrides)
	if err != nil {
		return fmt.Errorf("process overrides file: %w", err)
	}

	apiConfig := apiConfig{
		// TODO (@salah): implement reporting in api
		// MetricsReporter: installReporter,
		RuntimeConfig: rc,
		Password:      flags.adminConsolePassword,
		TLSConfig: apitypes.TLSConfig{
			CertBytes: flags.tlsCertBytes,
			KeyBytes:  flags.tlsKeyBytes,
			Hostname:  flags.hostname,
		},
		ManagerPort:   flags.managerPort,
		License:       flags.licenseBytes,
		AirgapBundle:  flags.airgapBundle,
		AirgapInfo:    airgapInfo,
		ConfigValues:  flags.configValues,
		ReleaseData:   release.GetReleaseData(),
		EndUserConfig: eucfg,
	}

	if err := startAPI(ctx, flags.tlsCert, apiConfig); err != nil {
		return fmt.Errorf("unable to start api: %w", err)
	}

	// TODO: add app name to this message (e.g., App Name manager)
	logrus.Infof("\nVisit the manager to continue: %s\n", getManagerURL(flags.hostname, flags.managerPort))
	<-ctx.Done()

	return nil
}

func runInstall(ctx context.Context, flags InstallCmdFlags, rc runtimeconfig.RuntimeConfig, installReporter *InstallReporter, airgapInfo *kotsv1beta1.Airgap) (finalErr error) {
	if flags.enableManagerExperience {
		return nil
	}

	logrus.Debug("initializing install")
	if err := initializeInstall(ctx, flags, rc); err != nil {
		return fmt.Errorf("unable to initialize install: %w", err)
	}

	logrus.Debugf("running install preflights")
	if err := runInstallPreflights(ctx, flags, rc, installReporter.reporter, airgapInfo); err != nil {
		if errors.Is(err, preflights.ErrPreflightsHaveFail) {
			return NewErrorNothingElseToAdd(err)
		}
		return fmt.Errorf("unable to run install preflights: %w", err)
	}

	if _, err := installAndStartCluster(ctx, flags, rc, nil); err != nil {
		return fmt.Errorf("unable to install cluster: %w", err)
	}

	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	mcli, err := kubeutils.MetadataClient()
	if err != nil {
		return fmt.Errorf("unable to create metadata client: %w", err)
	}

	errCh := kubeutils.WaitForKubernetes(ctx, kcli)
	defer logKubernetesErrors(errCh)

	in, err := recordInstallation(ctx, kcli, flags, rc, flags.license, airgapInfo)
	if err != nil {
		return fmt.Errorf("unable to record installation: %w", err)
	}

	if err := ecmetadata.CreateVersionMetadataConfigmap(ctx, kcli); err != nil {
		return fmt.Errorf("unable to create version metadata configmap: %w", err)
	}

	// TODO (@salah): update installation status to reflect what's happening

	logrus.Debugf("adding insecure registry")
	registryIP, err := registry.GetRegistryClusterIP(rc.ServiceCIDR())
	if err != nil {
		return fmt.Errorf("unable to get registry cluster IP: %w", err)
	}
	if err := hostutils.AddInsecureRegistry(fmt.Sprintf("%s:5000", registryIP)); err != nil {
		return fmt.Errorf("unable to add insecure registry: %w", err)
	}

	airgapChartsPath := ""
	if flags.isAirgap {
		airgapChartsPath = rc.EmbeddedClusterChartsSubDir()
	}

	hcli, err := helm.NewClient(helm.HelmOptions{
		KubeConfig: rc.PathToKubeConfig(),
		K0sVersion: versions.K0sVersion,
		AirgapPath: airgapChartsPath,
	})
	if err != nil {
		return fmt.Errorf("unable to create helm client: %w", err)
	}
	defer hcli.Close()

	logrus.Debugf("installing addons")
	if err := installAddons(ctx, kcli, mcli, hcli, rc, flags); err != nil {
		return err
	}

	logrus.Debugf("installing extensions")
	if err := installExtensions(ctx, hcli); err != nil {
		return fmt.Errorf("unable to install extensions: %w", err)
	}

	if err := kubeutils.SetInstallationState(ctx, kcli, in, ecv1beta1.InstallationStateInstalled, "Installed"); err != nil {
		return fmt.Errorf("unable to update installation: %w", err)
	}

	if err = support.CreateHostSupportBundle(ctx, kcli); err != nil {
		logrus.Warnf("Unable to create host support bundle: %v", err)
	}

	printSuccessMessage(flags.license, flags.hostname, flags.networkInterface, rc)

	return nil
}

func getAddonInstallOpts(flags InstallCmdFlags, rc runtimeconfig.RuntimeConfig, loading **spinner.MessageWriter) (*addons.InstallOptions, error) {
	var embCfgSpec *ecv1beta1.ConfigSpec
	if embCfg := release.GetEmbeddedClusterConfig(); embCfg != nil {
		embCfgSpec = &embCfg.Spec
	}

	euCfg, err := helpers.ParseEndUserConfig(flags.overrides)
	if err != nil {
		return nil, fmt.Errorf("unable to process overrides file: %w", err)
	}
	var euCfgSpec *ecv1beta1.ConfigSpec
	if euCfg != nil {
		euCfgSpec = &euCfg.Spec
	}

	opts := &addons.InstallOptions{
		AdminConsolePwd:         flags.adminConsolePassword,
		License:                 flags.license,
		IsAirgap:                flags.airgapBundle != "",
		TLSCertBytes:            flags.tlsCertBytes,
		TLSKeyBytes:             flags.tlsKeyBytes,
		Hostname:                flags.hostname,
		DisasterRecoveryEnabled: flags.license.Spec.IsDisasterRecoverySupported,
		IsMultiNodeEnabled:      flags.license.Spec.IsEmbeddedClusterMultiNodeEnabled,
		EmbeddedConfigSpec:      embCfgSpec,
		EndUserConfigSpec:       euCfgSpec,
		KotsInstaller: func() error {
			opts := kotscli.InstallOptions{
				RuntimeConfig:         rc,
				AppSlug:               flags.license.Spec.AppSlug,
				License:               flags.licenseBytes,
				Namespace:             runtimeconfig.KotsadmNamespace,
				AirgapBundle:          flags.airgapBundle,
				ConfigValuesFile:      flags.configValues,
				ReplicatedAppEndpoint: replicatedAppURL(),
				Stdout:                *loading,
			}
			return kotscli.Install(opts)
		},
	}
	return opts, nil
}

func verifyAndPrompt(ctx context.Context, name string, flags InstallCmdFlags, prompt prompts.Prompt, airgapInfo *kotsv1beta1.Airgap) error {
	logrus.Debugf("checking if k0s is already installed")
	err := verifyNoInstallation(name, "reinstall")
	if err != nil {
		return err
	}

	err = verifyChannelRelease("installation", flags.isAirgap, flags.assumeYes)
	if err != nil {
		return err
	}

	logrus.Debugf("checking license matches")
	license, err := getLicenseFromFilepath(flags.licenseFile)
	if err != nil {
		return err
	}
	if airgapInfo != nil {
		logrus.Debugf("checking airgap bundle matches binary")
		if err := checkAirgapMatches(airgapInfo); err != nil {
			return err // we want the user to see the error message without a prefix
		}
	}

	if !flags.isAirgap {
		if err := maybePromptForAppUpdate(ctx, prompt, license, flags.assumeYes); err != nil {
			if errors.As(err, &ErrorNothingElseToAdd{}) {
				return err
			}
			// If we get an error other than ErrorNothingElseToAdd, we warn and continue as this
			// check is not critical.
			logrus.Debugf("WARNING: Failed to check for newer app versions: %v", err)
		}
	}

	if err := release.ValidateECConfig(); err != nil {
		return err
	}

	return nil
}

func ensureAdminConsolePassword(flags *InstallCmdFlags) error {
	if flags.adminConsolePassword == "" {
		// no password was provided
		if flags.assumeYes {
			logrus.Infof("\nThe Admin Console password is set to %q.", "password")
			flags.adminConsolePassword = "password"
		} else {
			logrus.Info("")
			maxTries := 3
			for i := 0; i < maxTries; i++ {
				promptA, err := prompts.New().Password(fmt.Sprintf("Set the Admin Console password (minimum %d characters):", minAdminPasswordLength))
				if err != nil {
					return fmt.Errorf("failed to get password: %w", err)
				}

				promptB, err := prompts.New().Password("Confirm the Admin Console password:")
				if err != nil {
					return fmt.Errorf("failed to get password confirmation: %w", err)
				}

				if validateAdminConsolePassword(promptA, promptB) {
					flags.adminConsolePassword = promptA
					return nil
				}
			}
			return NewErrorNothingElseToAdd(errors.New("password is not valid"))
		}
	}

	if !validateAdminConsolePassword(flags.adminConsolePassword, flags.adminConsolePassword) {
		return NewErrorNothingElseToAdd(errors.New("password is not valid"))
	}

	return nil
}

func getLicenseFromFilepath(licenseFile string) (*kotsv1beta1.License, error) {
	rel := release.GetChannelRelease()

	// handle the three cases that do not require parsing the license file
	// 1. no release and no license, which is OK
	// 2. no license and a release, which is not OK
	// 3. a license and no release, which is not OK
	if rel == nil && licenseFile == "" {
		// no license and no release, this is OK
		return nil, nil
	} else if rel == nil && licenseFile != "" {
		// license is present but no release, this means we would install without vendor charts and k0s overrides
		return nil, fmt.Errorf("a license was provided but no release was found in binary, please rerun without the license flag")
	} else if rel != nil && licenseFile == "" {
		// release is present but no license, this is not OK
		return nil, fmt.Errorf("no license was provided for %s and one is required, please rerun with '--license <path to license file>'", rel.AppSlug)
	}

	license, err := helpers.ParseLicense(licenseFile)
	if err != nil {
		return nil, fmt.Errorf("unable to parse the license file at %q, please ensure it is not corrupt: %w", licenseFile, err)
	}

	// Check if the license matches the application version data
	if rel.AppSlug != license.Spec.AppSlug {
		// if the app is different, we will not be able to provide the correct vendor supplied charts and k0s overrides
		return nil, fmt.Errorf("license app %s does not match binary app %s, please provide the correct license", license.Spec.AppSlug, rel.AppSlug)
	}

	// Ensure the binary channel actually is present in the supplied license
	if err := checkChannelExistence(license, rel); err != nil {
		return nil, err
	}

	if license.Spec.Entitlements["expires_at"].Value.StrVal != "" {
		// read the expiration date, and check it against the current date
		expiration, err := time.Parse(time.RFC3339, license.Spec.Entitlements["expires_at"].Value.StrVal)
		if err != nil {
			return nil, fmt.Errorf("parse expiration date: %w", err)
		}
		if time.Now().After(expiration) {
			return nil, fmt.Errorf("license expired on %s, please provide a valid license", expiration)
		}
	}

	if !license.Spec.IsEmbeddedClusterDownloadEnabled {
		return nil, fmt.Errorf("license does not have embedded cluster enabled, please provide a valid license")
	}

	return license, nil
}

// checkChannelExistence verifies that a channel exists in a supplied license, returning a user-friendly
// error message actually listing available channels, if it does not.
func checkChannelExistence(license *kotsv1beta1.License, rel *release.ChannelRelease) error {
	var allowedChannels []string
	channelExists := false

	if len(license.Spec.Channels) == 0 { // support pre-multichannel licenses
		allowedChannels = append(allowedChannels, fmt.Sprintf("%s (%s)", license.Spec.ChannelName, license.Spec.ChannelID))
		channelExists = license.Spec.ChannelID == rel.ChannelID
	} else {
		for _, channel := range license.Spec.Channels {
			allowedChannels = append(allowedChannels, fmt.Sprintf("%s (%s)", channel.ChannelSlug, channel.ChannelID))
			if channel.ChannelID == rel.ChannelID {
				channelExists = true
			}
		}
	}

	if !channelExists {
		return fmt.Errorf("binary channel %s (%s) not present in license, channels allowed by license are: %s",
			rel.ChannelID, rel.ChannelSlug, strings.Join(allowedChannels, ", "))
	}

	return nil
}

func verifyChannelRelease(cmdName string, isAirgap bool, assumeYes bool) error {
	channelRelease := release.GetChannelRelease()

	if channelRelease != nil && channelRelease.Airgap && !isAirgap && !assumeYes {
		logrus.Warnf("\nYou downloaded an air gap bundle but didn't provide it with --airgap-bundle.")
		logrus.Warnf("If you continue, the %s will not use an air gap bundle and will connect to the internet.\n", cmdName)
		confirmed, err := prompts.New().Confirm(fmt.Sprintf("Do you want to proceed with an online %s?", cmdName), false)
		if err != nil {
			return fmt.Errorf("failed to get confirmation: %w", err)
		}
		if !confirmed {
			// TODO: send aborted metrics event
			return NewErrorNothingElseToAdd(errors.New("user aborted: air gap bundle downloaded but flag not provided"))
		}
	}
	return nil
}

func verifyNoInstallation(name string, cmdName string) error {
	installed, err := k0s.IsInstalled()
	if err != nil {
		return err
	}
	if installed {
		logrus.Errorf("\nAn installation is detected on this machine.")
		logrus.Infof("To %s, you must first remove the existing installation.", cmdName)
		logrus.Infof("You can do this by running the following command:")
		logrus.Infof("\n  sudo ./%s reset\n", name)
		return NewErrorNothingElseToAdd(errors.New("previous installation detected"))
	}
	return nil
}

func initializeInstall(ctx context.Context, flags InstallCmdFlags, rc runtimeconfig.RuntimeConfig) error {
	logrus.Info("")
	spinner := spinner.Start()
	spinner.Infof("Initializing")

	licenseBytes, err := os.ReadFile(flags.licenseFile)
	if err != nil {
		return fmt.Errorf("unable to read license file: %w", err)
	}

	if err := hostutils.ConfigureHost(ctx, rc, hostutils.InitForInstallOptions{
		License:      licenseBytes,
		AirgapBundle: flags.airgapBundle,
	}); err != nil {
		spinner.ErrorClosef("Initialization failed")
		return fmt.Errorf("configure host: %w", err)
	}

	spinner.Closef("Initialization complete")
	return nil
}

func installAndStartCluster(ctx context.Context, flags InstallCmdFlags, rc runtimeconfig.RuntimeConfig, mutate func(*k0sv1beta1.ClusterConfig) error) (*k0sv1beta1.ClusterConfig, error) {
	loading := spinner.Start()
	loading.Infof("Installing node")
	logrus.Debugf("creating k0s configuration file")

	eucfg, err := helpers.ParseEndUserConfig(flags.overrides)
	if err != nil {
		return nil, fmt.Errorf("process overrides file: %w", err)
	}

	cfg, err := k0s.WriteK0sConfig(ctx, flags.networkInterface, flags.airgapBundle, rc.PodCIDR(), rc.ServiceCIDR(), eucfg, mutate)
	if err != nil {
		loading.ErrorClosef("Failed to install node")
		return nil, fmt.Errorf("create config file: %w", err)
	}

	logrus.Debugf("creating systemd unit files")
	if err := hostutils.CreateSystemdUnitFiles(ctx, logrus.StandardLogger(), rc, false); err != nil {
		loading.ErrorClosef("Failed to install node")
		return nil, fmt.Errorf("create systemd unit files: %w", err)
	}

	logrus.Debugf("installing k0s")
	if err := k0s.Install(rc); err != nil {
		loading.ErrorClosef("Failed to install node")
		return nil, fmt.Errorf("install cluster: %w", err)
	}

	logrus.Debugf("waiting for k0s to be ready")
	if err := k0s.WaitForK0s(); err != nil {
		loading.ErrorClosef("Failed to install node")
		return nil, fmt.Errorf("wait for k0s: %w", err)
	}

	loading.Infof("Waiting for node")
	logrus.Debugf("waiting for node to be ready")
	if err := waitForNode(ctx); err != nil {
		loading.ErrorClosef("Node failed to become ready")
		return nil, fmt.Errorf("wait for node: %w", err)
	}

	loading.Closef("Node is ready")
	return cfg, nil
}

func installAddons(ctx context.Context, kcli client.Client, mcli metadata.Interface, hcli helm.Client, rc runtimeconfig.RuntimeConfig, flags InstallCmdFlags) error {
	progressChan := make(chan addontypes.AddOnProgress)
	defer close(progressChan)

	var loading *spinner.MessageWriter
	go func() {
		for progress := range progressChan {
			switch progress.Status.State {
			case apitypes.StateRunning:
				loading = spinner.Start()
				loading.Infof("Installing %s", progress.Name)
			case apitypes.StateSucceeded:
				loading.Closef("%s is ready", progress.Name)
			case apitypes.StateFailed:
				loading.ErrorClosef("Failed to install %s", progress.Name)
			}
		}
	}()

	addOns := addons.New(
		addons.WithLogFunc(logrus.Debugf),
		addons.WithKubernetesClient(kcli),
		addons.WithMetadataClient(mcli),
		addons.WithHelmClient(hcli),
		addons.WithRuntimeConfig(rc),
		addons.WithProgressChannel(progressChan),
	)

	opts, err := getAddonInstallOpts(flags, rc, &loading)
	if err != nil {
		return fmt.Errorf("get addon install opts: %w", err)
	}

	if err := addOns.Install(ctx, *opts); err != nil {
		return fmt.Errorf("install addons: %w", err)
	}

	return nil
}

func installExtensions(ctx context.Context, hcli helm.Client) error {
	progressChan := make(chan extensions.ExtensionsProgress)
	defer close(progressChan)

	loading := spinner.Start()
	loading.Infof("Installing additional components")

	go func() {
		for progress := range progressChan {
			loading.Infof("Installing additional components (%d/%d)", progress.Current, progress.Total)
		}
	}()

	if err := extensions.Install(ctx, hcli, progressChan); err != nil {
		loading.ErrorClosef("Failed to install additional components")
		return fmt.Errorf("unable to install extensions: %w", err)
	}

	loading.Closef("Additional components are ready")

	return nil
}

func checkAirgapMatches(airgapInfo *kotsv1beta1.Airgap) error {
	rel := release.GetChannelRelease()
	if rel == nil {
		return fmt.Errorf("airgap bundle provided but no release was found in binary, please rerun without the airgap-bundle flag")
	}

	appSlug := airgapInfo.Spec.AppSlug
	channelID := airgapInfo.Spec.ChannelID
	airgapVersion := airgapInfo.Spec.VersionLabel

	// Check if the airgap bundle matches the application version data
	if rel.AppSlug != appSlug {
		// if the app is different, we will not be able to provide the correct vendor supplied charts and k0s overrides
		return fmt.Errorf("airgap bundle app %s does not match binary app %s, please provide the correct bundle", appSlug, rel.AppSlug)
	}
	if rel.ChannelID != channelID {
		// if the channel is different, we will not be able to install the pinned vendor application version within kots
		return fmt.Errorf("airgap bundle channel %s does not match binary channel %s, please provide the correct bundle", channelID, rel.ChannelID)
	}
	if rel.VersionLabel != airgapVersion {
		// if the version is different, who knows what might be different
		return fmt.Errorf("airgap bundle version %s does not match binary version %s, please provide the correct bundle", airgapVersion, rel.VersionLabel)
	}

	return nil
}

// maybePromptForAppUpdate warns the user if the embedded release is not the latest for the current
// channel. If stdout is a terminal, it will prompt the user to continue installing the out-of-date
// release and return an error if the user chooses not to continue.
func maybePromptForAppUpdate(ctx context.Context, prompt prompts.Prompt, license *kotsv1beta1.License, assumeYes bool) error {
	channelRelease := release.GetChannelRelease()
	if channelRelease == nil {
		// It is possible to install without embedding the release data. In this case, we cannot
		// check for app updates.
		return nil
	}

	if license == nil {
		return errors.New("license required")
	}

	logrus.Debugf("Checking for pending app releases")

	currentRelease, err := getCurrentAppChannelRelease(ctx, license, channelRelease.ChannelID)
	if err != nil {
		return fmt.Errorf("get current app channel release: %w", err)
	}

	// In the dev and test environments, the channelSequence is set to 0 for all releases.
	if channelRelease.VersionLabel == currentRelease.VersionLabel {
		logrus.Debugf("Current app release is up-to-date")
		return nil
	}
	logrus.Debugf("Current app release is out-of-date")

	apiURL := replicatedAppURL()
	releaseURL := fmt.Sprintf("%s/embedded/%s/%s", apiURL, channelRelease.AppSlug, channelRelease.ChannelSlug)
	logrus.Warnf("\nA newer version %s is available.", currentRelease.VersionLabel)
	logrus.Infof(
		"To download it, run:\n  curl -fL \"%s\" \\\n    -H \"Authorization: %s\" \\\n    -o %s-%s.tgz\n",
		releaseURL,
		license.Spec.LicenseID,
		channelRelease.AppSlug,
		channelRelease.ChannelSlug,
	)

	// if the assumeYes flag is set, we don't prompt the user and continue by default.
	if assumeYes {
		return nil
	}

	text := fmt.Sprintf("Do you want to continue installing %s anyway?", channelRelease.VersionLabel)
	confirmed, err := prompt.Confirm(text, true)
	if err != nil {
		return fmt.Errorf("failed to get confirmation: %w", err)
	}
	if !confirmed {
		// TODO: send aborted metrics event
		return NewErrorNothingElseToAdd(errors.New("user aborted: app not up-to-date"))
	}

	logrus.Debug("User confirmed prompt to continue installing out-of-date release")

	return nil
}

// verifyProxyConfig prompts for confirmation when HTTP proxy is set without HTTPS proxy,
// returning an error if the user declines to proceed.
func verifyProxyConfig(proxy *ecv1beta1.ProxySpec, prompt prompts.Prompt, assumeYes bool) error {
	if proxy != nil && proxy.HTTPProxy != "" && proxy.HTTPSProxy == "" && !assumeYes {
		message := "Typically --https-proxy should be set if --http-proxy is set. Installation failures are likely otherwise. Do you want to continue anyway?"
		confirmed, err := prompt.Confirm(message, false)
		if err != nil {
			return fmt.Errorf("failed to confirm proxy settings: %w", err)
		}
		if !confirmed {
			return NewErrorNothingElseToAdd(errors.New("user aborted: HTTP proxy configured without HTTPS proxy"))
		}
		logrus.Debug("User confirmed prompt to proceed installing with `http_proxy` set and `https_proxy` unset")
	}
	return nil
}

// Minimum character length for the Admin Console password
const minAdminPasswordLength = 6

func validateAdminConsolePassword(password, passwordCheck string) bool {
	if password != passwordCheck {
		logrus.Errorf("Passwords don't match. Please try again.\n")
		return false
	}
	if len(password) < minAdminPasswordLength {
		logrus.Errorf("Password must have more than %d characters. Please try again.\n", minAdminPasswordLength)
		return false
	}
	return true
}

func replicatedAppURL() string {
	var embCfgSpec *ecv1beta1.ConfigSpec
	if embCfg := release.GetEmbeddedClusterConfig(); embCfg != nil {
		embCfgSpec = &embCfg.Spec
	}
	domains := runtimeconfig.GetDomains(embCfgSpec)
	return netutils.MaybeAddHTTPS(domains.ReplicatedAppDomain)
}

func proxyRegistryURL() string {
	var embCfgSpec *ecv1beta1.ConfigSpec
	if embCfg := release.GetEmbeddedClusterConfig(); embCfg != nil {
		embCfgSpec = &embCfg.Spec
	}
	domains := runtimeconfig.GetDomains(embCfgSpec)
	return netutils.MaybeAddHTTPS(domains.ProxyRegistryDomain)
}

func waitForNode(ctx context.Context) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("create kube client: %w", err)
	}
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("get hostname: %w", err)
	}
	nodename := strings.ToLower(hostname)
	if err := kubeutils.WaitForNode(ctx, kcli, nodename, false); err != nil {
		return fmt.Errorf("wait for node: %w", err)
	}
	return nil
}

func recordInstallation(
	ctx context.Context, kcli client.Client, flags InstallCmdFlags, rc runtimeconfig.RuntimeConfig, license *kotsv1beta1.License, airgapInfo *kotsv1beta1.Airgap,
) (*ecv1beta1.Installation, error) {
	// get the embedded cluster config
	cfg := release.GetEmbeddedClusterConfig()
	var cfgspec *ecv1beta1.ConfigSpec
	if cfg != nil {
		cfgspec = &cfg.Spec
	}

	// parse the end user config
	eucfg, err := helpers.ParseEndUserConfig(flags.overrides)
	if err != nil {
		return nil, fmt.Errorf("process overrides file: %w", err)
	}

	// extract airgap uncompressed size if airgap info is provided
	var airgapUncompressedSize int64
	if airgapInfo != nil {
		airgapUncompressedSize = airgapInfo.Spec.UncompressedSize
	}

	// record the installation
	installation, err := kubeutils.RecordInstallation(ctx, kcli, kubeutils.RecordInstallationOptions{
		IsAirgap:               flags.isAirgap,
		License:                license,
		ConfigSpec:             cfgspec,
		MetricsBaseURL:         replicatedAppURL(),
		RuntimeConfig:          rc.Get(),
		EndUserConfig:          eucfg,
		AirgapUncompressedSize: airgapUncompressedSize,
	})
	if err != nil {
		return nil, fmt.Errorf("record installation: %w", err)
	}

	return installation, nil
}

func normalizeNoPromptToYes(f *pflag.FlagSet, name string) pflag.NormalizedName {
	switch name {
	case "no-prompt":
		name = "yes"
	}
	return pflag.NormalizedName(name)
}

func printSuccessMessage(license *kotsv1beta1.License, hostname string, networkInterface string, rc runtimeconfig.RuntimeConfig) {
	adminConsoleURL := getAdminConsoleURL(hostname, networkInterface, rc.AdminConsolePort())

	// Create the message content
	message := fmt.Sprintf("Visit the Admin Console to configure and install %s:", license.Spec.AppSlug)

	// Determine the length of the longest line
	longestLine := len(message)
	if len(adminConsoleURL) > longestLine {
		longestLine = len(adminConsoleURL)
	}

	// Create the divider line
	divider := strings.Repeat("-", longestLine)

	// ANSI escape codes
	boldStart := "\033[1m"
	boldEnd := "\033[0m"
	greenStart := "\033[32m"
	greenEnd := "\033[0m"

	// Print the box in bold
	logrus.Infof("\n%s%s%s", boldStart, divider, boldEnd)
	logrus.Infof("%s%s%s", boldStart, message, boldEnd)
	logrus.Infof("%s%s%s", boldStart, "", boldEnd)
	logrus.Infof("%s%s%s%s%s", boldStart, greenStart, adminConsoleURL, greenEnd, boldEnd)
	logrus.Infof("%s%s%s\n", boldStart, divider, boldEnd)
}

func getAdminConsoleURL(hostname string, networkInterface string, port int) string {
	if hostname != "" {
		return fmt.Sprintf("http://%s:%v", hostname, port)
	}
	ipaddr := cloudutils.TryDiscoverPublicIP()
	if ipaddr == "" {
		if addr := os.Getenv("EC_PUBLIC_ADDRESS"); addr != "" {
			ipaddr = addr
		} else {
			var err error
			ipaddr, err = netutils.FirstValidAddress(networkInterface)
			if err != nil {
				logrus.Errorf("Unable to determine node IP address: %v", err)
				ipaddr = "NODE-IP-ADDRESS"
			}
		}
	}
	return fmt.Sprintf("http://%s:%v", ipaddr, port)
}

// logKubernetesErrors prints errors that may be related to k8s not coming up that manifest as
// addons failing to install. We run this in the background as waiting for kubernetes can take
// minutes and we can install addons in parallel.
func logKubernetesErrors(errCh <-chan error) {
	for {
		select {
		case err, ok := <-errCh:
			if !ok {
				return
			}
			logrus.Errorf("Infrastructure failed to become ready: %v", err)
		default:
			return
		}
	}
}
