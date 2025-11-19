package cli

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/google/uuid"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/goods"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/cloudutils"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/replicatedhq/embedded-cluster/pkg-new/domains"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg-new/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg-new/kubernetesinstallation"
	licensepkg "github.com/replicatedhq/embedded-cluster/pkg-new/license"
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
	"github.com/replicatedhq/embedded-cluster/web"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/crypto/bcrypt"
	helmcli "helm.sh/helm/v3/pkg/cli"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/metadata"
	nodeutil "k8s.io/component-helpers/node/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type installFlags struct {
	adminConsolePassword string
	adminConsolePort     int
	airgapBundle         string
	licenseFile          string
	assumeYes            bool
	overrides            string
	configValues         string
	headless             bool

	// linux flags
	dataDir                 string
	localArtifactMirrorPort int
	skipHostPreflights      bool
	ignoreHostPreflights    bool
	ignoreAppPreflights     bool
	networkInterface        string
	cidrConfig              *newconfig.CIDRConfig
	proxySpec               *ecv1beta1.ProxySpec

	// kubernetes flags
	kubernetesEnvSettings *helmcli.EnvSettings

	// guided UI flags
	target      string
	managerPort int
	tlsCertFile string
	tlsKeyFile  string
	hostname    string
}

// installConfig holds computed/derived values from install flags
type installConfig struct {
	clusterID               string
	isAirgap                bool
	enableManagerExperience bool
	licenseBytes            []byte
	license                 *kotsv1beta1.License
	airgapMetadata          *airgap.AirgapMetadata
	embeddedAssetsSize      int64
	endUserConfig           *ecv1beta1.Config
	tlsCert                 tls.Certificate
	tlsCertBytes            []byte
	tlsKeyBytes             []byte
	configValues            *kotsv1beta1.ConfigValues
}

// webAssetsFS is the filesystem to be used by the web component. Defaults to nil allowing the web server to use the default assets embedded in the binary. Useful for testing.
var webAssetsFS fs.FS = nil

// InstallCmd returns a cobra command for installing the embedded cluster.
func InstallCmd(ctx context.Context, appSlug, appTitle string) *cobra.Command {
	var flags installFlags

	ctx, cancel := context.WithCancel(ctx)

	rc := runtimeconfig.New(nil)
	ki := kubernetesinstallation.New(nil)

	short := fmt.Sprintf("Install %s", appTitle)
	if isV3Enabled() {
		short = fmt.Sprintf("Install %s onto Linux or Kubernetes", appTitle)
	}

	cmd := &cobra.Command{
		Use:     "install",
		Short:   short,
		Example: installCmdExample(appSlug),
		PostRun: func(cmd *cobra.Command, args []string) {
			rc.Cleanup()
			cancel() // Cancel context when command completes
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			installCfg, err := preRunInstall(cmd, &flags, rc, ki)
			if err != nil {
				return err
			}

			if err := verifyAndPrompt(ctx, cmd, appSlug, &flags, installCfg, prompts.New()); err != nil {
				return err
			}

			metricsReporter := buildMetricsReporter(cmd, installCfg)
			metricsReporter.ReportInstallationStarted(ctx)

			if installCfg.enableManagerExperience {
				apiOptions, err := buildAPIOptions(flags, installCfg, rc, ki, metricsReporter.reporter)
				if err != nil {
					return fmt.Errorf("build api options: %w", err)
				}
				return runManagerExperienceInstall(ctx, flags, installCfg, apiOptions, metricsReporter.reporter, appTitle)
			}

			_ = rc.SetEnv()

			// Setup signal handler with the metrics reporter cleanup function
			signalHandler(ctx, cancel, func(ctx context.Context, sig os.Signal) {
				metricsReporter.ReportSignalAborted(ctx, sig)
			})

			if err := runInstall(cmd.Context(), flags, installCfg, rc, metricsReporter); err != nil {
				// Check if this is an interrupt error from the terminal
				if errors.Is(err, terminal.InterruptErr) {
					metricsReporter.ReportSignalAborted(ctx, syscall.SIGINT)
				} else {
					metricsReporter.ReportInstallationFailed(ctx, err)
				}
				return err
			}
			metricsReporter.ReportInstallationSucceeded(ctx)

			return nil
		},
	}

	cmd.SetUsageTemplate(defaultUsageTemplateV3)

	mustAddInstallFlags(cmd, &flags)

	if err := addInstallAdminConsoleFlags(cmd, &flags); err != nil {
		panic(err)
	}
	if err := addTLSFlags(cmd, &flags); err != nil {
		panic(err)
	}
	if err := addManagementConsoleFlags(cmd, &flags); err != nil {
		panic(err)
	}

	cmd.AddCommand(InstallRunPreflightsCmd(ctx, appSlug))

	return cmd
}

const (
	installCmdExampleText = `
  # Install on a Linux host
  %s install \
      --target linux \
      --data-dir /opt/embedded-cluster \
      --license ./license.yaml \
      --yes

  # Install in a Kubernetes cluster
  %s install \
      --target kubernetes \
      --kubeconfig ./kubeconfig \
      --airgap-bundle ./replicated.airgap \
      --license ./license.yaml
`
)

func installCmdExample(appSlug string) string {
	if !isV3Enabled() {
		return ""
	}

	return fmt.Sprintf(installCmdExampleText, appSlug, appSlug)
}

func mustAddInstallFlags(cmd *cobra.Command, flags *installFlags) {
	enableV3 := isV3Enabled()

	normalizeFuncs := []func(f *pflag.FlagSet, name string) pflag.NormalizedName{}

	commonFlagSet := newCommonInstallFlags(flags, enableV3)
	cmd.Flags().AddFlagSet(commonFlagSet)
	if fn := commonFlagSet.GetNormalizeFunc(); fn != nil {
		normalizeFuncs = append(normalizeFuncs, fn)
	}

	linuxFlagSet := newLinuxInstallFlags(flags, enableV3)
	cmd.Flags().AddFlagSet(linuxFlagSet)
	if fn := linuxFlagSet.GetNormalizeFunc(); fn != nil {
		normalizeFuncs = append(normalizeFuncs, fn)
	}

	kubernetesFlagSet := newKubernetesInstallFlags(flags, enableV3)
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

func newCommonInstallFlags(flags *installFlags, enableV3 bool) *pflag.FlagSet {
	flagSet := pflag.NewFlagSet("common", pflag.ContinueOnError)

	flagSet.StringVar(&flags.target, "target", "", "The target platform to install to. Valid options are 'linux' or 'kubernetes'.")
	if enableV3 {
		mustMarkFlagRequired(flagSet, "target")
	} else {
		mustMarkFlagHidden(flagSet, "target")
	}

	flagSet.StringVar(&flags.airgapBundle, "airgap-bundle", "", "Path to the air gap bundle. If set, the installation will complete without internet access.")

	flagSet.StringVar(&flags.overrides, "overrides", "", "File with an EmbeddedClusterConfig object to override the default configuration")
	mustMarkFlagHidden(flagSet, "overrides")

	mustAddProxyFlags(flagSet)

	flagSet.BoolVarP(&flags.assumeYes, "yes", "y", false, "Assume yes to all prompts.")
	flagSet.SetNormalizeFunc(normalizeNoPromptToYes)

	return flagSet
}

func newLinuxInstallFlags(flags *installFlags, enableV3 bool) *pflag.FlagSet {
	flagSet := pflag.NewFlagSet("linux", pflag.ContinueOnError)

	// Use the app slug as default data directory only when ENABLE_V3 is set
	defaultDataDir := ecv1beta1.DefaultDataDir
	if enableV3 {
		defaultDataDir = filepath.Join("/var/lib", runtimeconfig.AppSlug())
	}

	flagSet.StringVar(&flags.dataDir, "data-dir", defaultDataDir, "Path to the data directory")
	flagSet.IntVar(&flags.localArtifactMirrorPort, "local-artifact-mirror-port", ecv1beta1.DefaultLocalArtifactMirrorPort, "Port on which the Local Artifact Mirror will be served")
	flagSet.StringVar(&flags.networkInterface, "network-interface", "", "The network interface to use for the cluster")

	flagSet.StringSlice("private-ca", []string{}, "Path to a trusted private CA certificate file")
	mustMarkFlagHidden(flagSet, "private-ca")
	mustMarkFlagDeprecated(flagSet, "private-ca", "This flag is no longer used and will be removed in a future version. The CA bundle will be automatically detected from the host.")

	flagSet.BoolVar(&flags.skipHostPreflights, "skip-host-preflights", false, "Skip host preflight checks. This is not recommended and has been deprecated.")
	mustMarkFlagHidden(flagSet, "skip-host-preflights")
	mustMarkFlagDeprecated(flagSet, "skip-host-preflights", "This flag is deprecated and will be removed in a future version. Use --ignore-host-preflights instead.")

	flagSet.BoolVar(&flags.ignoreHostPreflights, "ignore-host-preflights", false, "Allow bypassing host preflight failures")
	flagSet.BoolVar(&flags.ignoreAppPreflights, "ignore-app-preflights", false, "Allow bypassing app preflight failures")

	mustAddCIDRFlags(flagSet)

	flagSet.VisitAll(func(flag *pflag.Flag) {
		mustSetFlagTargetLinux(flagSet, flag.Name)
	})

	return flagSet
}

func newKubernetesInstallFlags(flags *installFlags, enableV3 bool) *pflag.FlagSet {
	flagSet := pflag.NewFlagSet("kubernetes", pflag.ContinueOnError)

	addKubernetesCLIFlags(flagSet, flags)

	flagSet.VisitAll(func(flag *pflag.Flag) {
		if !enableV3 {
			mustMarkFlagHidden(flagSet, flag.Name)
		}
		mustSetFlagTargetKubernetes(flagSet, flag.Name)
	})

	return flagSet
}

func addKubernetesCLIFlags(flagSet *pflag.FlagSet, flags *installFlags) {
	s := helmcli.New()
	helm.AddKubernetesCLIFlags(flagSet, s)
	flags.kubernetesEnvSettings = s
}

func addInstallAdminConsoleFlags(cmd *cobra.Command, flags *installFlags) error {
	cmd.Flags().StringVar(&flags.adminConsolePassword, "admin-console-password", "", "Password for the Admin Console")
	cmd.Flags().IntVar(&flags.adminConsolePort, "admin-console-port", ecv1beta1.DefaultAdminConsolePort, "Port on which the Admin Console will be served")
	cmd.Flags().StringVarP(&flags.licenseFile, "license", "l", "", "Path to the license file")
	mustMarkFlagRequired(cmd.Flags(), "license")
	cmd.Flags().StringVar(&flags.configValues, "config-values", "", "Path to the config values to use when installing")

	return nil
}

func addTLSFlags(cmd *cobra.Command, flags *installFlags) error {
	managerName := "Admin Console"
	if isV3Enabled() {
		managerName = "Manager"
	}

	cmd.Flags().StringVar(&flags.tlsCertFile, "tls-cert", "", fmt.Sprintf("Path to the TLS certificate file for the %s", managerName))
	cmd.Flags().StringVar(&flags.tlsKeyFile, "tls-key", "", fmt.Sprintf("Path to the TLS key file for the %s", managerName))
	cmd.Flags().StringVar(&flags.hostname, "hostname", "", fmt.Sprintf("Hostname to use for accessing the %s", managerName))

	return nil
}

func addManagementConsoleFlags(cmd *cobra.Command, flags *installFlags) error {
	cmd.Flags().IntVar(&flags.managerPort, "manager-port", ecv1beta1.DefaultManagerPort, "Port on which the Manager will be served")
	cmd.Flags().BoolVar(&flags.headless, "headless", false, "Run installation in headless mode without UI interaction.")

	// If the ENABLE_V3 environment variable is set, default to the new manager experience and do
	// not hide the manager-port flag.
	if !isV3Enabled() {
		if err := cmd.Flags().MarkHidden("manager-port"); err != nil {
			return err
		}
		if err := cmd.Flags().MarkHidden("headless"); err != nil {
			return err
		}
	}

	return nil
}

// Hop: buildMetricsReporter builds the metrics reporter for installation tracking
func buildMetricsReporter(cmd *cobra.Command, installCfg *installConfig) *installReporter {
	return newInstallReporter(
		replicatedAppURL(), cmd.CalledAs(), flagsToStringSlice(cmd.Flags()),
		installCfg.license.Spec.LicenseID, installCfg.clusterID, installCfg.license.Spec.AppSlug,
	)
}

func preRunInstall(cmd *cobra.Command, flags *installFlags, rc runtimeconfig.RuntimeConfig, ki kubernetesinstallation.Installation) (*installConfig, error) {
	// Build install flags
	if err := buildInstallFlags(cmd, flags); err != nil {
		return nil, err
	}

	// Build install config
	installCfg, err := buildInstallConfig(flags)
	if err != nil {
		return nil, err
	}

	// sync the license if we are in the manager experience and a license is provided and we are
	// not in airgap mode
	if installCfg.enableManagerExperience && installCfg.license != nil && !installCfg.isAirgap {
		replicatedAPI, err := newReplicatedAPIClient(installCfg.license, installCfg.clusterID)
		if err != nil {
			return nil, fmt.Errorf("failed to create replicated API client: %w", err)
		}

		updatedLicense, licenseBytes, err := syncLicense(cmd.Context(), replicatedAPI, installCfg.license)
		if err != nil {
			return nil, fmt.Errorf("failed to sync license: %w", err)
		}
		installCfg.license = updatedLicense
		installCfg.licenseBytes = licenseBytes
	}

	// Target-specific configuration
	switch flags.target {
	case "linux":
		if err := buildRuntimeConfig(flags, installCfg, rc); err != nil {
			return nil, fmt.Errorf("build runtime config: %w", err)
		}
		if err := preRunInstallLinux(); err != nil {
			return nil, fmt.Errorf("pre run install linux: %w", err)
		}
	case "kubernetes":
		if err := buildKubernetesInstallation(flags, ki); err != nil {
			return nil, fmt.Errorf("build kubernetes installation: %w", err)
		}
		if err := preRunInstallKubernetes(flags); err != nil {
			return nil, fmt.Errorf("pre run install kubernetes: %w", err)
		}
	}

	return installCfg, nil
}

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
		} else {
			logrus.Info("\nContinuing with a self-signed certificate...\n")
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

func preRunInstallLinux() error {
	if os.Getuid() != 0 {
		return fmt.Errorf("install command must be run as root")
	}

	// set the umask to 022 so that we can create files/directories with 755 permissions
	// this does not return an error - it returns the previous umask
	_ = syscall.Umask(0o022)

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

func preRunInstallKubernetes(flags *installFlags) error {
	// TODO: we only support amd64 clusters for target=kubernetes installs
	helpers.SetClusterArch("amd64")

	// If set, validate that the kubeconfig file exists and can be read
	if flags.kubernetesEnvSettings.KubeConfig != "" {
		if _, err := os.Stat(flags.kubernetesEnvSettings.KubeConfig); os.IsNotExist(err) {
			return fmt.Errorf("kubeconfig file does not exist: %s", flags.kubernetesEnvSettings.KubeConfig)
		} else if err != nil {
			return fmt.Errorf("failed to stat kubeconfig file: %w", err)
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

	return nil
}

// Hop: buildKubernetesInstallation sets kubernetes installation values from install flags
func buildKubernetesInstallation(flags *installFlags, ki kubernetesinstallation.Installation) error {
	ki.SetAdminConsolePort(flags.adminConsolePort)
	ki.SetManagerPort(flags.managerPort)
	ki.SetProxySpec(flags.proxySpec)
	ki.SetKubernetesEnvSettings(flags.kubernetesEnvSettings)

	return nil
}

func runManagerExperienceInstall(
	ctx context.Context, flags installFlags, installCfg *installConfig, apiOpts apiOptions,
	metricsReporter metrics.ReporterInterface, appTitle string,
) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	apiExitCh, err := startAPI(ctx, installCfg.tlsCert, apiOpts)
	if err != nil {
		return fmt.Errorf("failed to start api: %w", err)
	}

	if flags.headless {
		return runV3InstallHeadless(ctx, cancel, flags, apiOpts, metricsReporter)
	}

	logrus.Infof("\nVisit the %s manager to continue: %s\n",
		appTitle,
		getManagerURL(flags.hostname, flags.managerPort))

	// Wait for either user cancellation or API unexpected exit
	select {
	case <-ctx.Done():
		// Normal exit (user interrupted)
		return nil
	case err := <-apiExitCh:
		// API exited unexpectedly
		if err != nil {
			return err
		}
		return fmt.Errorf("api server exited unexpectedly")
	}
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

func runInstall(ctx context.Context, flags installFlags, installCfg *installConfig, rc runtimeconfig.RuntimeConfig, metricsReporter *installReporter) (finalErr error) {
	if installCfg.enableManagerExperience {
		return nil
	}

	logrus.Debug("initializing install")
	if err := initializeInstall(ctx, flags, installCfg, rc); err != nil {
		return fmt.Errorf("failed to initialize install: %w", err)
	}

	logrus.Debugf("running install preflights")
	if err := runInstallPreflights(ctx, flags, installCfg, rc, metricsReporter.reporter); err != nil {
		if errors.Is(err, preflights.ErrPreflightsHaveFail) {
			return NewErrorNothingElseToAdd(err)
		}
		return fmt.Errorf("failed to run install preflights: %w", err)
	}

	if _, err := installAndStartCluster(ctx, flags, installCfg, rc, nil); err != nil {
		return fmt.Errorf("failed to install cluster: %w", err)
	}

	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("failed to create kube client: %w", err)
	}

	mcli, err := kubeutils.MetadataClient()
	if err != nil {
		return fmt.Errorf("failed to create metadata client: %w", err)
	}

	errCh := kubeutils.WaitForKubernetes(ctx, kcli)
	defer logKubernetesErrors(errCh)

	in, err := kubeutils.RecordInstallation(ctx, kcli, buildRecordInstallationOptions(installCfg, rc))
	if err != nil {
		return fmt.Errorf("record installation: %w", err)
	}

	if err := ecmetadata.CreateVersionMetadataConfigmap(ctx, kcli); err != nil {
		return fmt.Errorf("failed to create version metadata configmap: %w", err)
	}

	// TODO (@salah): update installation status to reflect what's happening

	logrus.Debugf("adding insecure registry")
	registryIP, err := registry.GetRegistryClusterIP(rc.ServiceCIDR())
	if err != nil {
		return fmt.Errorf("failed to get registry cluster IP: %w", err)
	}
	if err := hostutils.AddInsecureRegistry(fmt.Sprintf("%s:5000", registryIP)); err != nil {
		return fmt.Errorf("failed to add insecure registry: %w", err)
	}

	helmOpts := buildHelmClientOptions(installCfg, rc)

	hcli, err := helm.NewClient(helmOpts)
	if err != nil {
		return fmt.Errorf("failed to create helm client: %w", err)
	}
	defer hcli.Close()

	logrus.Debugf("installing addons")
	if err := installAddons(ctx, kcli, mcli, hcli, flags, installCfg, rc); err != nil {
		return err
	}

	logrus.Debugf("installing extensions")
	if err := installExtensions(ctx, hcli); err != nil {
		return fmt.Errorf("failed to install extensions: %w", err)
	}

	if err := kubeutils.SetInstallationState(ctx, kcli, in, ecv1beta1.InstallationStateInstalled, "Installed"); err != nil {
		return fmt.Errorf("failed to update installation: %w", err)
	}

	if err = support.CreateHostSupportBundle(ctx, kcli); err != nil {
		logrus.Warnf("failed to create host support bundle: %v", err)
	}

	isHeadlessInstall := flags.configValues != "" && flags.adminConsolePassword != ""

	printSuccessMessage(installCfg.license, flags.hostname, flags.networkInterface, rc, isHeadlessInstall)

	return nil
}

// Hop: buildK0sConfig builds k0s cluster configuration from install flags and config
func buildK0sConfig(flags *installFlags, installCfg *installConfig) (*k0sv1beta1.ClusterConfig, error) {
	return k0s.NewK0sConfig(flags.networkInterface, installCfg.isAirgap, flags.cidrConfig.PodCIDR, flags.cidrConfig.ServiceCIDR, installCfg.endUserConfig, nil)
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

func verifyAndPrompt(ctx context.Context, cmd *cobra.Command, appSlug string, flags *installFlags, installCfg *installConfig, prompt prompts.Prompt) error {
	logrus.Debugf("checking if k0s is already installed")
	err := verifyNoInstallation(appSlug, "reinstall")
	if err != nil {
		return err
	}

	err = verifyChannelRelease("installation", installCfg.isAirgap, flags.assumeYes)
	if err != nil {
		return err
	}

	logrus.Debugf("checking license matches")
	verifiedLicense, err := verifyLicense(installCfg.license)
	if err != nil {
		return err
	}
	installCfg.license = verifiedLicense

	if installCfg.airgapMetadata != nil && installCfg.airgapMetadata.AirgapInfo != nil {
		logrus.Debugf("checking airgap bundle matches binary")
		if err := checkAirgapMatches(installCfg.airgapMetadata.AirgapInfo); err != nil {
			return err // we want the user to see the error message without a prefix
		}
	}

	if !installCfg.isAirgap {
		if err := maybePromptForAppUpdate(ctx, prompt, installCfg.license, flags.assumeYes); err != nil {
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

	// TODO (@salah): figure out how we can move this to buildInstallFlags without changing product behavior
	if err := verifyProxyConfig(flags.proxySpec, prompts.New(), flags.assumeYes); err != nil {
		return err
	}

	// restore command doesn't have a password flag
	if cmd.Flags().Lookup("admin-console-password") != nil {
		// TODO (@salah): figure out how we can move this to buildInstallFlags without changing product behavior
		if err := ensureAdminConsolePassword(flags); err != nil {
			return err
		}
	}

	return nil
}

func ensureAdminConsolePassword(flags *installFlags) error {
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

func verifyLicense(license *kotsv1beta1.License) (*kotsv1beta1.License, error) {
	channelRelease := release.GetChannelRelease()
	if err := verifyLicensePresence(license, channelRelease); err != nil {
		return nil, err
	}

	if isV3Enabled() {
		verifiedLicense, err := licensepkg.VerifySignature(license)
		if err != nil {
			return nil, fmt.Errorf("license signature verification failed: %w", err)
		}
		license = verifiedLicense
	}

	if err := verifyLicenseFields(license, channelRelease); err != nil {
		return nil, err
	}

	return license, nil
}

// verifyLicensePresence checks if license presence matches the release requirements
func verifyLicensePresence(license *kotsv1beta1.License, channelRelease *release.ChannelRelease) error {
	if channelRelease == nil {
		if license == nil {
			// No license, no release - valid
			return nil
		}
		// Valid license, no release - invalid
		return fmt.Errorf("a license was provided but no release was found in binary, please rerun without the license flag")
	}
	if license == nil {
		// No license, with release - invalid
		return fmt.Errorf("no license was provided for %s and one is required, please rerun with '--license <path to license file>'", channelRelease.AppSlug)
	}

	return nil
}

// verifyLicenseFields validates license fields against the release data
func verifyLicenseFields(license *kotsv1beta1.License, channelRelease *release.ChannelRelease) error {
	if channelRelease == nil || license == nil {
		return nil
	}

	// Check if the license matches the application version data
	if channelRelease.AppSlug != license.Spec.AppSlug {
		// if the app is different, we will not be able to provide the correct vendor supplied charts and k0s overrides
		return fmt.Errorf("license app %s does not match binary app %s, please provide the correct license", license.Spec.AppSlug, channelRelease.AppSlug)
	}

	// Ensure the binary channel actually is present in the supplied license
	if err := checkChannelExistence(license, channelRelease); err != nil {
		return err
	}

	expiresAt, ok := license.Spec.Entitlements["expires_at"]
	if ok && expiresAt.Value.StrVal != "" {
		// read the expiration date, and check it against the current date
		expiration, err := time.Parse(time.RFC3339, expiresAt.Value.StrVal)
		if err != nil {
			return fmt.Errorf("parse expiration date: %w", err)
		}
		if time.Now().After(expiration) {
			return fmt.Errorf("license expired on %s, please provide a valid license", expiration)
		}
	}

	if !license.Spec.IsEmbeddedClusterDownloadEnabled {
		return fmt.Errorf("license does not have embedded cluster enabled, please provide a valid license")
	}

	return nil
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

func verifyNoInstallation(appSlug string, cmdName string) error {
	installed, err := k0s.IsInstalled()
	if err != nil {
		return err
	}
	if installed {
		logrus.Errorf("\nAn installation is detected on this machine.")
		logrus.Infof("To %s, you must first remove the existing installation.", cmdName)
		logrus.Infof("You can do this by running the following command:")
		logrus.Infof("\n  sudo ./%s reset\n", appSlug)
		return NewErrorNothingElseToAdd(errors.New("previous installation detected"))
	}
	return nil
}

func initializeInstall(ctx context.Context, flags installFlags, installCfg *installConfig, rc runtimeconfig.RuntimeConfig) error {
	logrus.Info("")
	spinner := spinner.Start()
	spinner.Infof("Initializing")

	if err := hostutils.ConfigureHost(ctx, rc, hostutils.InitForInstallOptions{
		License:      installCfg.licenseBytes,
		AirgapBundle: flags.airgapBundle,
	}); err != nil {
		spinner.ErrorClosef("Initialization failed")
		return fmt.Errorf("configure host: %w", err)
	}

	spinner.Closef("Initialization complete")
	return nil
}

func installAndStartCluster(ctx context.Context, flags installFlags, installCfg *installConfig, rc runtimeconfig.RuntimeConfig, mutate func(*k0sv1beta1.ClusterConfig) error) (*k0sv1beta1.ClusterConfig, error) {
	loading := spinner.Start()
	loading.Infof("Installing node")

	// Detect stable hostname early in installation
	hostname, err := nodeutil.GetHostname("")
	if err != nil {
		loading.ErrorClosef("Failed to install node")
		return nil, fmt.Errorf("failed to detect hostname: %w", err)
	}

	logrus.Debugf("creating k0s configuration file")

	cfg, err := buildK0sConfig(&flags, installCfg)
	if err != nil {
		return nil, fmt.Errorf("unable to build k0s config: %w", err)
	}

	err = k0s.WriteK0sConfig(ctx, cfg)
	if err != nil {
		loading.ErrorClosef("Failed to install node")
		return nil, fmt.Errorf("create config file: %w", err)
	}

	logrus.Debugf("creating systemd unit files")
	if err := hostutils.CreateSystemdUnitFiles(ctx, logrus.StandardLogger(), rc, hostname, false); err != nil {
		loading.ErrorClosef("Failed to install node")
		return nil, fmt.Errorf("create systemd unit files: %w", err)
	}

	logrus.Debugf("installing k0s")
	if err := k0s.Install(rc, hostname); err != nil {
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

func installAddons(ctx context.Context, kcli client.Client, mcli metadata.Interface, hcli helm.Client, flags installFlags, installCfg *installConfig, rc runtimeconfig.RuntimeConfig) error {
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
		addons.WithDomains(getDomains()),
		addons.WithProgressChannel(progressChan),
	)

	kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(ctx, kcli)
	if err != nil {
		return fmt.Errorf("get kotsadm namespace: %w", err)
	}

	opts := buildAddonInstallOpts(flags, installCfg, rc, kotsadmNamespace, &loading)

	if err := addOns.Install(ctx, *opts); err != nil {
		return fmt.Errorf("install addons: %w", err)
	}

	return nil
}

func getDomains() ecv1beta1.Domains {
	var embCfgSpec *ecv1beta1.ConfigSpec
	if embCfg := release.GetEmbeddedClusterConfig(); embCfg != nil {
		embCfgSpec = &embCfg.Spec
	}

	return domains.GetDomains(embCfgSpec, release.GetChannelRelease())
}

func installExtensions(ctx context.Context, hcli helm.Client) error {
	progressChan := make(chan extensions.ExtensionsProgress)

	loading := spinner.Start()
	loading.Infof("Installing additional components")

	// Use a done channel to signal when the progress goroutine has finished
	done := make(chan struct{})
	go func() {
		defer close(done)
		for progress := range progressChan {
			loading.Infof("Installing additional components (%d/%d)", progress.Current, progress.Total)
		}
	}()

	err := extensions.Install(ctx, hcli, progressChan)
	<-done // Wait for the goroutine to finish processing all progress updates

	if err != nil {
		loading.ErrorClosef("Failed to install additional components")
		return fmt.Errorf("failed to install extensions: %w", err)
	}

	loading.Closef("Additional components are ready")

	return nil
}

func checkAirgapMatches(airgapInfo *kotsv1beta1.Airgap) error {
	if airgapInfo == nil {
		return fmt.Errorf("airgap info is required")
	}

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
	confirmed, err := prompt.Confirm(text, false)
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

func waitForNode(ctx context.Context) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("create kube client: %w", err)
	}
	nodename, err := nodeutil.GetHostname("")
	if err != nil {
		return fmt.Errorf("get hostname: %w", err)
	}
	if err := kubeutils.WaitForNode(ctx, kcli, nodename, false); err != nil {
		return fmt.Errorf("wait for node: %w", err)
	}
	return nil
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

func normalizeNoPromptToYes(f *pflag.FlagSet, name string) pflag.NormalizedName {
	switch name {
	case "no-prompt":
		name = "yes"
	}
	return pflag.NormalizedName(name)
}

func printSuccessMessage(license *kotsv1beta1.License, hostname string, networkInterface string, rc runtimeconfig.RuntimeConfig, isHeadlessInstall bool) {
	adminConsoleURL := getAdminConsoleURL(hostname, networkInterface, rc.AdminConsolePort())

	// Create the message content
	var message string
	if isHeadlessInstall {
		message = fmt.Sprintf("The Admin Console for %s is available at:", license.Spec.AppSlug)
	} else {
		message = fmt.Sprintf("Visit the Admin Console to configure and install %s:", license.Spec.AppSlug)
	}

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
				logrus.Errorf("failed to determine node IP address: %v", err)
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
