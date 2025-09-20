package cli

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/AlecAivazis/survey/v2/terminal"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/goods"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
	"github.com/replicatedhq/embedded-cluster/pkg-new/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/configutils"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	rcutil "github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig/util"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	helmcli "helm.sh/helm/v3/pkg/cli"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type UpgradeCmdFlags struct {
	adminConsolePort   int
	airgapBundle       string
	airgapMetadata     *airgap.AirgapMetadata
	embeddedAssetsSize int64
	isAirgap           bool
	licenseFile        string
	assumeYes          bool
	overrides          string
	configValues       string

	// linux flags
	dataDir              string
	ignoreHostPreflights bool
	ignoreAppPreflights  bool
	networkInterface     string

	// kubernetes flags
	kubernetesEnvSettings *helmcli.EnvSettings

	// guided UI flags
	target      string
	managerPort int
	tlsCertFile string
	tlsKeyFile  string
	hostname    string

	upgradeConfig
}

type upgradeConfig struct {
	clusterID    string
	license      *kotsv1beta1.License
	licenseBytes []byte
	tlsCert      tls.Certificate
	tlsCertBytes []byte
	tlsKeyBytes  []byte

	kubernetesRESTClientGetter genericclioptions.RESTClientGetter
}

// UpgradeCmd returns a cobra command for upgrading the embedded cluster application.
func UpgradeCmd(ctx context.Context, appSlug, appTitle string) *cobra.Command {
	var flags UpgradeCmdFlags

	ctx, cancel := context.WithCancel(ctx)

	rc := runtimeconfig.New(nil)
	ki := kubernetesinstallation.New(nil)

	short := fmt.Sprintf("Upgrade %s onto Linux or Kubernetes", appTitle)

	cmd := &cobra.Command{
		Use:     "upgrade",
		Short:   short,
		Example: upgradeCmdExample(appSlug),
		PostRun: func(cmd *cobra.Command, args []string) {
			rc.Cleanup()
			cancel() // Cancel context when command completes
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := preRunUpgrade(cmd, &flags, rc, ki); err != nil {
				return err
			}
			if err := verifyAndPromptUpgrade(ctx, &flags, prompts.New()); err != nil {
				return err
			}

			metricsReporter := newUpgradeReporter(
				replicatedAppURL(), cmd.CalledAs(), flagsToStringSlice(cmd.Flags()),
				flags.license.Spec.LicenseID, flags.clusterID, flags.license.Spec.AppSlug,
			)
			metricsReporter.ReportUpgradeStarted(ctx)

			// Setup signal handler with the metrics reporter cleanup function
			signalHandler(ctx, cancel, func(ctx context.Context, sig os.Signal) {
				metricsReporter.ReportSignalAborted(ctx, sig)
			})

			if err := runManagerExperienceUpgrade(ctx, flags, rc, ki, metricsReporter.reporter, appTitle); err != nil {
				// Check if this is an interrupt error from the terminal
				if errors.Is(err, terminal.InterruptErr) {
					metricsReporter.ReportSignalAborted(ctx, syscall.SIGINT)
				} else {
					metricsReporter.ReportUpgradeFailed(ctx, err)
				}
				return err
			}
			metricsReporter.ReportUpgradeSucceeded(ctx)

			return nil
		},
	}

	cmd.SetUsageTemplate(defaultUsageTemplateV3)

	mustAddUpgradeFlags(cmd, &flags)

	if err := addUpgradeAdminConsoleFlags(cmd, &flags); err != nil {
		panic(err)
	}
	if err := addUpgradeTLSFlags(cmd, &flags); err != nil {
		panic(err)
	}
	if err := addUpgradeManagementConsoleFlags(cmd, &flags); err != nil {
		panic(err)
	}

	return cmd
}

const (
	upgradeCmdExampleText = `
  # Upgrade on a Linux host
  %s upgrade \
      --target linux \
      --license ./license.yaml \
      --yes

  # Upgrade in a Kubernetes cluster
  %s upgrade \
      --target kubernetes \
      --kubeconfig ./kubeconfig \
      --airgap-bundle ./replicated.airgap \
      --license ./license.yaml
`
)

func upgradeCmdExample(appSlug string) string {
	return fmt.Sprintf(upgradeCmdExampleText, appSlug, appSlug)
}

func mustAddUpgradeFlags(cmd *cobra.Command, flags *UpgradeCmdFlags) {
	normalizeFuncs := []func(f *pflag.FlagSet, name string) pflag.NormalizedName{}

	commonFlagSet := newCommonUpgradeFlags(flags)
	cmd.Flags().AddFlagSet(commonFlagSet)
	if fn := commonFlagSet.GetNormalizeFunc(); fn != nil {
		normalizeFuncs = append(normalizeFuncs, fn)
	}

	linuxFlagSet := newLinuxUpgradeFlags(flags)
	cmd.Flags().AddFlagSet(linuxFlagSet)
	if fn := linuxFlagSet.GetNormalizeFunc(); fn != nil {
		normalizeFuncs = append(normalizeFuncs, fn)
	}

	kubernetesFlagSet := newKubernetesUpgradeFlags(flags)
	cmd.Flags().AddFlagSet(kubernetesFlagSet)
	if fn := kubernetesFlagSet.GetNormalizeFunc(); fn != nil {
		normalizeFuncs = append(normalizeFuncs, fn)
	}

	cmd.Flags().SetNormalizeFunc(func(f *pflag.FlagSet, name string) pflag.NormalizedName {
		result := pflag.NormalizedName(strings.ToLower(name))
		for _, fn := range normalizeFuncs {
			if fn != nil {
				result = fn(f, string(result))
			}
		}
		return result
	})
}

func newCommonUpgradeFlags(flags *UpgradeCmdFlags) *pflag.FlagSet {
	flagSet := pflag.NewFlagSet("common", pflag.ContinueOnError)

	flagSet.StringVar(&flags.target, "target", "", "The target platform to upgrade. Valid options are 'linux' or 'kubernetes'.")
	mustMarkFlagRequired(flagSet, "target")

	flagSet.StringVar(&flags.airgapBundle, "airgap-bundle", "", "Path to the air gap bundle. If set, the upgrade will complete without internet access.")

	flagSet.StringVar(&flags.overrides, "overrides", "", "File with an EmbeddedClusterConfig object to override the default configuration")
	mustMarkFlagHidden(flagSet, "overrides")

	mustAddProxyFlags(flagSet)

	flagSet.BoolVarP(&flags.assumeYes, "yes", "y", false, "Assume yes to all prompts.")
	flagSet.SetNormalizeFunc(normalizeNoPromptToYes)

	return flagSet
}

func newLinuxUpgradeFlags(flags *UpgradeCmdFlags) *pflag.FlagSet {
	flagSet := pflag.NewFlagSet("linux", pflag.ContinueOnError)

	// For upgrade, we'll detect the existing data directory
	flagSet.StringVar(&flags.dataDir, "data-dir", "", "Path to the existing data directory (will be auto-detected if not provided)")
	flagSet.StringVar(&flags.networkInterface, "network-interface", "", "The network interface to use for the cluster")

	flagSet.BoolVar(&flags.ignoreHostPreflights, "ignore-host-preflights", false, "Allow bypassing host preflight failures")
	flagSet.BoolVar(&flags.ignoreAppPreflights, "ignore-app-preflights", false, "Allow bypassing app preflight failures")

	mustAddCIDRFlags(flagSet)

	flagSet.VisitAll(func(flag *pflag.Flag) {
		mustSetFlagTargetLinux(flagSet, flag.Name)
	})

	return flagSet
}

func newKubernetesUpgradeFlags(flags *UpgradeCmdFlags) *pflag.FlagSet {
	flagSet := pflag.NewFlagSet("kubernetes", pflag.ContinueOnError)

	addUpgradeKubernetesCLIFlags(flagSet, flags)

	flagSet.VisitAll(func(flag *pflag.Flag) {
		mustSetFlagTargetKubernetes(flagSet, flag.Name)
	})

	return flagSet
}

func addUpgradeAdminConsoleFlags(cmd *cobra.Command, flags *UpgradeCmdFlags) error {
	cmd.Flags().StringVarP(&flags.licenseFile, "license", "l", "", "Path to the license file")
	mustMarkFlagRequired(cmd.Flags(), "license")
	cmd.Flags().StringVar(&flags.configValues, "config-values", "", "Path to the config values to use when upgrading")

	return nil
}

func addUpgradeTLSFlags(cmd *cobra.Command, flags *UpgradeCmdFlags) error {
	managerName := "Manager"

	cmd.Flags().StringVar(&flags.tlsCertFile, "tls-cert", "", fmt.Sprintf("Path to the TLS certificate file for the %s", managerName))
	cmd.Flags().StringVar(&flags.tlsKeyFile, "tls-key", "", fmt.Sprintf("Path to the TLS key file for the %s", managerName))
	cmd.Flags().StringVar(&flags.hostname, "hostname", "", fmt.Sprintf("Hostname to use for accessing the %s", managerName))

	return nil
}

func addUpgradeManagementConsoleFlags(cmd *cobra.Command, flags *UpgradeCmdFlags) error {
	cmd.Flags().IntVar(&flags.managerPort, "manager-port", ecv1beta1.DefaultManagerPort, "Port on which the Manager will be served")
	return nil
}

func addUpgradeKubernetesCLIFlags(flagSet *pflag.FlagSet, flags *UpgradeCmdFlags) {
	// From helm
	// https://github.com/helm/helm/blob/v3.18.3/pkg/cli/environment.go#L145-L163

	s := helmcli.New()

	flagSet.StringVar(&s.KubeConfig, "kubeconfig", "", "Path to the kubeconfig file")
	flagSet.StringVar(&s.KubeContext, "kube-context", s.KubeContext, "Name of the kubeconfig context to use")
	flagSet.StringVar(&s.KubeToken, "kube-token", s.KubeToken, "Bearer token used for authentication")
	flagSet.StringVar(&s.KubeAsUser, "kube-as-user", s.KubeAsUser, "Username to impersonate for the operation")
	flagSet.StringArrayVar(&s.KubeAsGroups, "kube-as-group", s.KubeAsGroups, "Group to impersonate for the operation, this flag can be repeated to specify multiple groups.")
	flagSet.StringVar(&s.KubeAPIServer, "kube-apiserver", s.KubeAPIServer, "The address and the port for the Kubernetes API server")
	flagSet.StringVar(&s.KubeCaFile, "kube-ca-file", s.KubeCaFile, "The certificate authority file for the Kubernetes API server connection")
	flagSet.StringVar(&s.KubeTLSServerName, "kube-tls-server-name", s.KubeTLSServerName, "Server name to use for Kubernetes API server certificate validation. If it is not provided, the hostname used to contact the server is used")
	flagSet.BoolVar(&s.KubeInsecureSkipTLSVerify, "kube-insecure-skip-tls-verify", s.KubeInsecureSkipTLSVerify, "If true, the Kubernetes API server's certificate will not be checked for validity. This will make your HTTPS connections insecure")
	flagSet.IntVar(&s.BurstLimit, "burst-limit", s.BurstLimit, "Kubernetes API client-side default throttling limit")
	flagSet.Float32Var(&s.QPS, "qps", s.QPS, "Queries per second used when communicating with the Kubernetes API, not including bursting")

	flags.kubernetesEnvSettings = s
}

func processUpgradeTLSConfig(flags *UpgradeCmdFlags) error {
	// If both cert and key are provided, validate and load them
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

		return nil
	}

	// If only one of cert or key is provided, return an error
	if flags.tlsCertFile != "" || flags.tlsKeyFile != "" {
		return fmt.Errorf("both --tls-cert and --tls-key must be provided together")
	}

	// If neither is provided, no TLS configuration (will use default behavior)
	return nil
}

func preRunUpgrade(cmd *cobra.Command, flags *UpgradeCmdFlags, rc runtimeconfig.RuntimeConfig, ki kubernetesinstallation.Installation) error {
	if !slices.Contains([]string{"linux", "kubernetes"}, flags.target) {
		return fmt.Errorf(`invalid --target (must be one of: "linux", "kubernetes")`)
	}

	// For Linux upgrades, set up kubeconfig from existing runtime config
	if flags.target == "linux" {
		existingRC, err := rcutil.GetRuntimeConfigFromCluster(context.Background())
		if err != nil {
			return fmt.Errorf("failed to get runtime config from cluster: %w", err)
		}
		kubeconfig := existingRC.PathToKubeConfig()
		if kubeconfig != "" {
			os.Setenv("KUBECONFIG", kubeconfig)
			logrus.Debugf("set KUBECONFIG to %s", kubeconfig)
		}
	}

	// Get cluster ID from existing installation instead of generating new one
	installation, err := getExistingInstallation(context.Background())
	if err != nil {
		return fmt.Errorf("could not get existing installation: %w", err)
	}
	if installation.Spec.ClusterID == "" {
		return fmt.Errorf("existing installation has empty cluster ID")
	}
	flags.clusterID = installation.Spec.ClusterID

	if err := preRunUpgradeCommon(cmd, flags, rc, ki); err != nil {
		return err
	}

	switch flags.target {
	case "linux":
		return preRunUpgradeLinux(cmd, flags, rc)
	case "kubernetes":
		return preRunUpgradeKubernetes(cmd, flags, ki)
	}

	return nil
}

func preRunUpgradeCommon(cmd *cobra.Command, flags *UpgradeCmdFlags, rc runtimeconfig.RuntimeConfig, ki kubernetesinstallation.Installation) error {
	// license file is required for upgrade
	if flags.licenseFile != "" {
		b, err := os.ReadFile(flags.licenseFile)
		if err != nil {
			return fmt.Errorf("unable to read license file: %w", err)
		}
		flags.licenseBytes = b

		// validate the license is indeed a license file
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
	if flags.airgapBundle != "" {
		metadata, err := airgap.AirgapMetadataFromPath(flags.airgapBundle)
		if err != nil {
			return fmt.Errorf("failed to get airgap info: %w", err)
		}
		flags.airgapMetadata = metadata
	}

	var err error
	flags.embeddedAssetsSize, err = goods.SizeOfEmbeddedAssets()
	if err != nil {
		return fmt.Errorf("failed to get size of embedded files: %w", err)
	}

	// Read existing TLS certificates from cluster secrets
	// This is required for upgrades - we must use certificates from the existing installation
	if err := readKotsadmTLSSecret(flags); err != nil {
		return fmt.Errorf("upgrade requires TLS certificates from existing installation: %w", err)
	}

	// Read existing admin console configuration from cluster secrets
	if err := readKotsadmConfigMap(flags); err != nil {
		return fmt.Errorf("failed to read admin console configuration from existing installation: %w", err)
	}

	if flags.managerPort != 0 && flags.adminConsolePort != 0 {
		if flags.managerPort == flags.adminConsolePort {
			return fmt.Errorf("manager port cannot be the same as admin console port")
		}
	}

	proxy, err := proxyConfigFromCmd(cmd, flags.assumeYes)
	if err != nil {
		return err
	}

	rc.SetAdminConsolePort(flags.adminConsolePort)
	ki.SetAdminConsolePort(flags.adminConsolePort)

	rc.SetManagerPort(flags.managerPort)
	ki.SetManagerPort(flags.managerPort)

	rc.SetProxySpec(proxy)
	ki.SetProxySpec(proxy)

	// Process TLS certificate configuration if provided
	if err := processUpgradeTLSConfig(flags); err != nil {
		return fmt.Errorf("process TLS configuration: %w", err)
	}

	return nil
}

func preRunUpgradeLinux(cmd *cobra.Command, flags *UpgradeCmdFlags, rc runtimeconfig.RuntimeConfig) error {
	if !cmd.Flags().Changed("ignore-host-preflights") && (os.Getenv("SKIP_HOST_PREFLIGHTS") == "1" || os.Getenv("SKIP_HOST_PREFLIGHTS") == "true") {
		flags.ignoreHostPreflights = true
	}

	if os.Getuid() != 0 {
		return fmt.Errorf("upgrade command must be run as root")
	}

	// set the umask to 022 so that we can create files/directories with 755 permissions
	// this does not return an error - it returns the previous umask
	_ = syscall.Umask(0o022)

	// If data directory wasn't provided, try to detect it from existing installation
	if flags.dataDir == "" {
		// Try to get from existing runtime config
		existingRC, err := rcutil.GetRuntimeConfigFromCluster(context.Background())
		if err == nil && existingRC.EmbeddedClusterHomeDirectory() != "" {
			flags.dataDir = existingRC.EmbeddedClusterHomeDirectory()
		} else {
			// Fall back to default locations
			if _, err := os.Stat(ecv1beta1.DefaultDataDir); err == nil {
				flags.dataDir = ecv1beta1.DefaultDataDir
			} else {
				// Try app-specific location
				appSpecificDir := filepath.Join("/var/lib", runtimeconfig.AppSlug())
				if _, err := os.Stat(appSpecificDir); err == nil {
					flags.dataDir = appSpecificDir
				} else {
					return fmt.Errorf("upgrade requires existing data directory from previous installation: could not detect data directory, please specify with --data-dir")
				}
			}
		}
	}

	// Validate that data directory exists
	if _, err := os.Stat(flags.dataDir); os.IsNotExist(err) {
		return fmt.Errorf("upgrade requires existing data directory from previous installation: directory does not exist: %s", flags.dataDir)
	}

	hostCABundlePath, err := findHostCABundle()
	if err != nil {
		return fmt.Errorf("unable to find host CA bundle: %w", err)
	}
	logrus.Debugf("using host CA bundle: %s", hostCABundlePath)

	// Get existing runtime config for network interface detection and network spec
	existingRC, err := rcutil.GetRuntimeConfigFromCluster(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get existing runtime config: %w", err)
	}

	// if a network interface flag was not provided, attempt to discover it
	if flags.networkInterface == "" {
		if existingRC.Get() != nil {
			flags.networkInterface = existingRC.NetworkInterface()
		}
	}

	// Use existing network spec as-is for upgrades
	networkSpec := existingRC.Get().Network

	// TODO: validate that a single port isn't used for multiple services
	// resolve datadir to absolute path
	absoluteDataDir, err := filepath.Abs(flags.dataDir)
	if err != nil {
		return fmt.Errorf("unable to construct path for directory: %w", err)
	}
	rc.SetDataDir(absoluteDataDir)
	rc.SetHostCABundlePath(hostCABundlePath)
	rc.SetNetworkSpec(networkSpec)

	return nil
}

func preRunUpgradeKubernetes(_ *cobra.Command, flags *UpgradeCmdFlags, _ kubernetesinstallation.Installation) error {
	// TODO: we only support amd64 clusters for target=kubernetes upgrades
	helpers.SetClusterArch("amd64")

	// If set, validate that the kubeconfig file exists and can be read
	if flags.kubernetesEnvSettings.KubeConfig != "" {
		if _, err := os.Stat(flags.kubernetesEnvSettings.KubeConfig); os.IsNotExist(err) {
			return fmt.Errorf("kubeconfig file does not exist: %s", flags.kubernetesEnvSettings.KubeConfig)
		} else if err != nil {
			return fmt.Errorf("unable to stat kubeconfig file: %w", err)
		}
	}

	restConfig, err := flags.kubernetesEnvSettings.RESTClientGetter().ToRESTConfig()
	if err != nil {
		return fmt.Errorf("failed to discover kubeconfig: %w", err)
	}

	// Check that we have a valid kubeconfig
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create discovery client: %w", err)
	}
	_, err = discoveryClient.ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to connect to kubernetes api server: %w", err)
	}

	flags.kubernetesRESTClientGetter = flags.kubernetesEnvSettings.RESTClientGetter()

	return nil
}

func verifyAndPromptUpgrade(ctx context.Context, flags *UpgradeCmdFlags, prompt prompts.Prompt) error {
	logrus.Debugf("checking if existing installation is present")
	installation, err := getExistingInstallation(context.Background())
	if err != nil || installation == nil {
		return NewErrorNothingElseToAdd(errors.New("no existing installation detected"))
	}

	err = verifyChannelRelease("upgrade", flags.isAirgap, flags.assumeYes)
	if err != nil {
		return err
	}

	logrus.Debugf("checking license matches")
	license, err := getLicenseFromFilepath(flags.licenseFile)
	if err != nil {
		return err
	}
	if flags.airgapMetadata != nil && flags.airgapMetadata.AirgapInfo != nil {
		logrus.Debugf("checking airgap bundle matches binary")
		if err := checkAirgapMatches(flags.airgapMetadata.AirgapInfo); err != nil {
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

func getExistingInstallation(ctx context.Context) (*ecv1beta1.Installation, error) {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	installation, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest installation: %w", err)
	}

	return installation, nil
}

func readKotsadmTLSSecret(flags *UpgradeCmdFlags) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Read TLS certificate from kotsadm-tls secret
	tlsSecret := &corev1.Secret{}
	err = kcli.Get(context.Background(), client.ObjectKey{
		Namespace: constants.KotsadmNamespace,
		Name:      "kotsadm-tls",
	}, tlsSecret)
	if err != nil {
		return fmt.Errorf("failed to read kotsadm-tls secret from cluster: %w", err)
	}

	certData, hasCert := tlsSecret.Data["tls.crt"]
	keyData, hasKey := tlsSecret.Data["tls.key"]

	if !hasCert || !hasKey {
		return fmt.Errorf("kotsadm-tls secret is missing required tls.crt or tls.key data")
	}

	cert, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		return fmt.Errorf("failed to load TLS certificate from kotsadm-tls secret: %w", err)
	}

	flags.tlsCert = cert
	flags.tlsCertBytes = certData
	flags.tlsKeyBytes = keyData

	return nil
}

func readKotsadmConfigMap(flags *UpgradeCmdFlags) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Read admin console port from existing configuration
	configMap := &corev1.ConfigMap{}
	err = kcli.Get(context.Background(), client.ObjectKey{
		Namespace: constants.KotsadmNamespace,
		Name:      "kotsadm-confg", // Correct name as defined in kots/pkg/kotsadm/types/constants.go
	}, configMap)
	if err == nil {
		// Parse admin console port from config if available
		// This would depend on the actual structure of the config
		flags.adminConsolePort = ecv1beta1.DefaultAdminConsolePort // fallback
	} else {
		flags.adminConsolePort = ecv1beta1.DefaultAdminConsolePort
	}

	return nil
}

// readKotsadmPasswordSecret reads the bcrypt password hash from the kotsadm-password secret
func readKotsadmPasswordSecret() ([]byte, error) {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	passwordSecret := &corev1.Secret{}
	err = kcli.Get(context.Background(), client.ObjectKey{
		Namespace: constants.KotsadmNamespace,
		Name:      "kotsadm-password",
	}, passwordSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to read kotsadm-password secret from cluster: %w", err)
	}

	passwordBcryptData, hasPasswordBcrypt := passwordSecret.Data["passwordBcrypt"]
	if !hasPasswordBcrypt {
		return nil, fmt.Errorf("kotsadm-password secret is missing required passwordBcrypt data")
	}

	return passwordBcryptData, nil
}

func runManagerExperienceUpgrade(
	ctx context.Context, flags UpgradeCmdFlags, rc runtimeconfig.RuntimeConfig, ki kubernetesinstallation.Installation,
	metricsReporter metrics.ReporterInterface, appTitle string,
) (finalErr error) {
	// Verify we have TLS certificates (should have been loaded from cluster secrets)
	if len(flags.tlsCertBytes) == 0 || len(flags.tlsKeyBytes) == 0 {
		return fmt.Errorf("TLS certificates are required for upgrade but were not found in the kotsadm-tls secret")
	}

	// Read password hash from the kotsadm-password secret
	passwordHash, err := readKotsadmPasswordSecret()
	if err != nil {
		return fmt.Errorf("failed to read password hash from cluster: %w", err)
	}

	eucfg, err := helpers.ParseEndUserConfig(flags.overrides)
	if err != nil {
		return fmt.Errorf("process overrides file: %w", err)
	}

	var configValues apitypes.AppConfigValues
	if flags.configValues != "" {
		configValues = make(apitypes.AppConfigValues)
		kotsConfigValues, err := helpers.ParseConfigValues(flags.configValues)
		if err != nil {
			return fmt.Errorf("parse config values file: %w", err)
		}
		if kotsConfigValues != nil {
			for key, value := range kotsConfigValues.Spec.Values {
				configValues[key] = apitypes.AppConfigValue{Value: value.Value}
			}
		}
	}

	apiConfig := apiOptions{
		APIConfig: apitypes.APIConfig{
			Password:     "", // Only PasswordHash is necessary for upgrades because the kotsadm-password secret has been created already
			PasswordHash: passwordHash,
			TLSConfig: apitypes.TLSConfig{
				CertBytes: flags.tlsCertBytes,
				KeyBytes:  flags.tlsKeyBytes,
				Hostname:  flags.hostname,
			},
			License:            flags.licenseBytes,
			AirgapBundle:       flags.airgapBundle,
			AirgapMetadata:     flags.airgapMetadata,
			EmbeddedAssetsSize: flags.embeddedAssetsSize,
			ConfigValues:       configValues,
			ReleaseData:        release.GetReleaseData(),
			EndUserConfig:      eucfg,
			ClusterID:          flags.clusterID,

			LinuxConfig: apitypes.LinuxConfig{
				RuntimeConfig:             rc,
				AllowIgnoreHostPreflights: flags.ignoreHostPreflights,
			},
			KubernetesConfig: apitypes.KubernetesConfig{
				RESTClientGetter: flags.kubernetesRESTClientGetter,
				Installation:     ki,
			},
		},

		ManagerPort:     flags.managerPort,
		InstallTarget:   flags.target,
		MetricsReporter: metricsReporter,
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := startAPI(ctx, flags.tlsCert, apiConfig, cancel); err != nil {
		return fmt.Errorf("unable to start api: %w", err)
	}

	logrus.Infof("\nVisit the %s manager to continue the upgrade: %s\n",
		appTitle,
		getManagerURL(flags.hostname, flags.managerPort))
	<-ctx.Done()

	return nil
}
