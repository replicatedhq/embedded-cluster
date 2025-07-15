package cli

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/google/uuid"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/cloudutils"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
	"github.com/replicatedhq/embedded-cluster/pkg-new/domains"
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
	"github.com/replicatedhq/embedded-cluster/pkg/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
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
	helmcli "helm.sh/helm/v3/pkg/cli"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type InstallCmdFlags struct {
	adminConsolePassword string
	adminConsolePort     int
	airgapBundle         string
	isAirgap             bool
	licenseFile          string
	assumeYes            bool
	overrides            string
	configValues         string

	// linux flags
	dataDir                 string
	localArtifactMirrorPort int
	skipHostPreflights      bool
	ignoreHostPreflights    bool
	networkInterface        string

	// kubernetes flags
	kubernetesEnvSettings *helmcli.EnvSettings

	// guided UI flags
	enableManagerExperience bool
	target                  string
	managerPort             int
	tlsCertFile             string
	tlsKeyFile              string
	hostname                string

	installConfig
}

type installConfig struct {
	clusterID    string
	license      *kotsv1beta1.License
	licenseBytes []byte
	tlsCert      tls.Certificate
	tlsCertBytes []byte
	tlsKeyBytes  []byte

	kubernetesRESTClientGetter genericclioptions.RESTClientGetter
}

// webAssetsFS is the filesystem to be used by the web component. Defaults to nil allowing the web server to use the default assets embedded in the binary. Useful for testing.
var webAssetsFS fs.FS = nil

// InstallCmd returns a cobra command for installing the embedded cluster.
func InstallCmd(ctx context.Context, appSlug, appTitle string) *cobra.Command {
	var flags InstallCmdFlags

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
			if err := preRunInstall(cmd, &flags, rc, ki); err != nil {
				return err
			}
			if err := verifyAndPrompt(ctx, cmd, appSlug, &flags, prompts.New()); err != nil {
				return err
			}

			installReporter := newInstallReporter(
				replicatedAppURL(), cmd.CalledAs(), flagsToStringSlice(cmd.Flags()),
				flags.license.Spec.LicenseID, flags.clusterID, flags.license.Spec.AppSlug,
			)
			installReporter.ReportInstallationStarted(ctx)

			if flags.enableManagerExperience {
				return runManagerExperienceInstall(ctx, flags, rc, ki, installReporter, appTitle)
			}

			_ = rc.SetEnv()

			// Setup signal handler with the metrics reporter cleanup function
			signalHandler(ctx, cancel, func(ctx context.Context, sig os.Signal) {
				installReporter.ReportSignalAborted(ctx, sig)
			})

			if err := runInstall(cmd.Context(), flags, rc, installReporter); err != nil {
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

	cmd.SetUsageTemplate(defaultUsageTemplateV3)

	mustAddInstallFlags(cmd, &flags)

	if err := addInstallAdminConsoleFlags(cmd, &flags); err != nil {
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

func mustAddInstallFlags(cmd *cobra.Command, flags *InstallCmdFlags) {
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

func newCommonInstallFlags(flags *InstallCmdFlags, enableV3 bool) *pflag.FlagSet {
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

func newLinuxInstallFlags(flags *InstallCmdFlags, enableV3 bool) *pflag.FlagSet {
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

	mustAddCIDRFlags(flagSet)

	flagSet.VisitAll(func(flag *pflag.Flag) {
		mustSetFlagTargetLinux(flagSet, flag.Name)
	})

	return flagSet
}

func newKubernetesInstallFlags(flags *InstallCmdFlags, enableV3 bool) *pflag.FlagSet {
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

func addKubernetesCLIFlags(flagSet *pflag.FlagSet, flags *InstallCmdFlags) {
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
	// flagSet.BoolVar(&s.Debug, "helm-debug", s.Debug, "enable verbose output")
	flagSet.BoolVar(&s.KubeInsecureSkipTLSVerify, "kube-insecure-skip-tls-verify", s.KubeInsecureSkipTLSVerify, "If true, the Kubernetes API server's certificate will not be checked for validity. This will make your HTTPS connections insecure")
	// flagSet.StringVar(&s.RegistryConfig, "helm-registry-config", s.RegistryConfig, "Path to the Helm registry config file")
	// flagSet.StringVar(&s.RepositoryConfig, "helm-repository-config", s.RepositoryConfig, "Path to the file containing Helm repository names and URLs")
	// flagSet.StringVar(&s.RepositoryCache, "helm-repository-cache", s.RepositoryCache, "Path to the directory containing cached Helm repository indexes")
	flagSet.IntVar(&s.BurstLimit, "burst-limit", s.BurstLimit, "Kubernetes API client-side default throttling limit")
	flagSet.Float32Var(&s.QPS, "qps", s.QPS, "Queries per second used when communicating with the Kubernetes API, not including bursting")

	flags.kubernetesEnvSettings = s
}

func addInstallAdminConsoleFlags(cmd *cobra.Command, flags *InstallCmdFlags) error {
	cmd.Flags().StringVar(&flags.adminConsolePassword, "admin-console-password", "", "Password for the Admin Console")
	cmd.Flags().IntVar(&flags.adminConsolePort, "admin-console-port", ecv1beta1.DefaultAdminConsolePort, "Port on which the Admin Console will be served")
	cmd.Flags().StringVarP(&flags.licenseFile, "license", "l", "", "Path to the license file")
	mustMarkFlagRequired(cmd.Flags(), "license")
	cmd.Flags().StringVar(&flags.configValues, "config-values", "", "Path to the config values to use when installing")

	return nil
}

func addManagementConsoleFlags(cmd *cobra.Command, flags *InstallCmdFlags) error {
	cmd.Flags().IntVar(&flags.managerPort, "manager-port", ecv1beta1.DefaultManagerPort, "Port on which the Manager will be served")
	cmd.Flags().StringVar(&flags.tlsCertFile, "tls-cert", "", "Path to the TLS certificate file")
	cmd.Flags().StringVar(&flags.tlsKeyFile, "tls-key", "", "Path to the TLS key file")
	cmd.Flags().StringVar(&flags.hostname, "hostname", "", "Hostname to use for TLS configuration")

	// If the ENABLE_V3 environment variable is set, default to the new manager experience and do
	// not hide the new flags.
	if !isV3Enabled() {
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
	}

	return nil
}

func preRunInstall(cmd *cobra.Command, flags *InstallCmdFlags, rc runtimeconfig.RuntimeConfig, ki kubernetesinstallation.Installation) error {
	if !isV3Enabled() {
		flags.target = "linux"
	}

	if !slices.Contains([]string{"linux", "kubernetes"}, flags.target) {
		return fmt.Errorf(`invalid --target (must be one of: "linux", "kubernetes")`)
	}

	flags.clusterID = uuid.New().String()

	if err := preRunInstallCommon(cmd, flags, rc, ki); err != nil {
		return err
	}

	switch flags.target {
	case "linux":
		return preRunInstallLinux(cmd, flags, rc)
	case "kubernetes":
		return preRunInstallKubernetes(cmd, flags, ki)
	}

	return nil
}

func preRunInstallCommon(cmd *cobra.Command, flags *InstallCmdFlags, rc runtimeconfig.RuntimeConfig, ki kubernetesinstallation.Installation) error {
	flags.enableManagerExperience = isV3Enabled()

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

	return nil
}

func preRunInstallLinux(cmd *cobra.Command, flags *InstallCmdFlags, rc runtimeconfig.RuntimeConfig) error {
	if !cmd.Flags().Changed("skip-host-preflights") && (os.Getenv("SKIP_HOST_PREFLIGHTS") == "1" || os.Getenv("SKIP_HOST_PREFLIGHTS") == "true") {
		flags.skipHostPreflights = true
	}

	if os.Getuid() != 0 {
		return fmt.Errorf("install command must be run as root")
	}

	// set the umask to 022 so that we can create files/directories with 755 permissions
	// this does not return an error - it returns the previous umask
	_ = syscall.Umask(0o022)

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
	rc.SetHostCABundlePath(hostCABundlePath)
	rc.SetNetworkSpec(networkSpec)

	return nil
}

func preRunInstallKubernetes(_ *cobra.Command, flags *InstallCmdFlags, _ kubernetesinstallation.Installation) error {
	// TODO: we only support amd64 clusters for target=kubernetes installs
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

	// testing

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

func runManagerExperienceInstall(
	ctx context.Context, flags InstallCmdFlags, rc runtimeconfig.RuntimeConfig, ki kubernetesinstallation.Installation,
	installReporter *InstallReporter, appTitle string,
) (finalErr error) {
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

	var configValues map[string]string
	if flags.configValues != "" {
		configValues = make(map[string]string)
		kotsConfigValues, err := helpers.ParseConfigValues(flags.configValues)
		if err != nil {
			return fmt.Errorf("parse config values file: %w", err)
		}
		if kotsConfigValues != nil {
			for key, value := range kotsConfigValues.Spec.Values {
				configValues[key] = value.Value
			}
		}
	}

	apiConfig := apiOptions{
		APIConfig: apitypes.APIConfig{
			Password: flags.adminConsolePassword,
			TLSConfig: apitypes.TLSConfig{
				CertBytes: flags.tlsCertBytes,
				KeyBytes:  flags.tlsKeyBytes,
				Hostname:  flags.hostname,
			},
			License:       flags.licenseBytes,
			AirgapBundle:  flags.airgapBundle,
			ConfigValues:  configValues,
			ReleaseData:   release.GetReleaseData(),
			EndUserConfig: eucfg,
			ClusterID:     flags.clusterID,

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
		MetricsReporter: installReporter.reporter,
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := startAPI(ctx, flags.tlsCert, apiConfig, cancel); err != nil {
		return fmt.Errorf("unable to start api: %w", err)
	}

	logrus.Infof("\nVisit the %s manager to continue: %s\n",
		appTitle,
		getManagerURL(flags.hostname, flags.managerPort))
	<-ctx.Done()

	return nil
}

func runInstall(ctx context.Context, flags InstallCmdFlags, rc runtimeconfig.RuntimeConfig, installReporter *InstallReporter) (finalErr error) {
	if flags.enableManagerExperience {
		return nil
	}

	logrus.Debug("initializing install")
	if err := initializeInstall(ctx, flags, rc); err != nil {
		return fmt.Errorf("unable to initialize install: %w", err)
	}

	logrus.Debugf("running install preflights")
	if err := runInstallPreflights(ctx, flags, rc, installReporter.reporter); err != nil {
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

	in, err := recordInstallation(ctx, kcli, flags, rc)
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
	if err := installAddons(ctx, kcli, mcli, hcli, flags, rc); err != nil {
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
		ClusterID:               flags.clusterID,
		AdminConsolePwd:         flags.adminConsolePassword,
		AdminConsolePort:        rc.AdminConsolePort(),
		License:                 flags.license,
		IsAirgap:                flags.airgapBundle != "",
		TLSCertBytes:            flags.tlsCertBytes,
		TLSKeyBytes:             flags.tlsKeyBytes,
		Hostname:                flags.hostname,
		DisasterRecoveryEnabled: flags.license.Spec.IsDisasterRecoverySupported,
		IsMultiNodeEnabled:      flags.license.Spec.IsEmbeddedClusterMultiNodeEnabled,
		EmbeddedConfigSpec:      embCfgSpec,
		EndUserConfigSpec:       euCfgSpec,
		ProxySpec:               rc.ProxySpec(),
		HostCABundlePath:        rc.HostCABundlePath(),
		DataDir:                 rc.EmbeddedClusterHomeDirectory(),
		K0sDataDir:              rc.EmbeddedClusterK0sSubDir(),
		OpenEBSDataDir:          rc.EmbeddedClusterOpenEBSLocalSubDir(),
		ServiceCIDR:             rc.ServiceCIDR(),
		KotsInstaller: func() error {
			opts := kotscli.InstallOptions{
				RuntimeConfig:         rc,
				AppSlug:               flags.license.Spec.AppSlug,
				License:               flags.licenseBytes,
				Namespace:             constants.KotsadmNamespace,
				ClusterID:             flags.clusterID,
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

func verifyAndPrompt(ctx context.Context, cmd *cobra.Command, appSlug string, flags *InstallCmdFlags, prompt prompts.Prompt) error {
	logrus.Debugf("checking if k0s is already installed")
	err := verifyNoInstallation(appSlug, "reinstall")
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
	if flags.isAirgap {
		logrus.Debugf("checking airgap bundle matches binary")
		if err := checkAirgapMatches(flags.airgapBundle); err != nil {
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

	// restore command doesn't have a password flag
	if cmd.Flags().Lookup("admin-console-password") != nil {
		if err := ensureAdminConsolePassword(flags); err != nil {
			return err
		}
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

func installAddons(ctx context.Context, kcli client.Client, mcli metadata.Interface, hcli helm.Client, flags InstallCmdFlags, rc runtimeconfig.RuntimeConfig) error {
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

	opts, err := getAddonInstallOpts(flags, rc, &loading)
	if err != nil {
		return fmt.Errorf("get addon install opts: %w", err)
	}

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

func checkAirgapMatches(airgapBundle string) error {
	rel := release.GetChannelRelease()
	if rel == nil {
		return fmt.Errorf("airgap bundle provided but no release was found in binary, please rerun without the airgap-bundle flag")
	}

	// read file from path
	rawfile, err := os.Open(airgapBundle)
	if err != nil {
		return fmt.Errorf("failed to open airgap file: %w", err)
	}
	defer rawfile.Close()

	appSlug, channelID, airgapVersion, err := airgap.ChannelReleaseMetadata(rawfile)
	if err != nil {
		return fmt.Errorf("failed to get airgap bundle versions: %w", err)
	}

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
	domains := getDomains()
	return netutils.MaybeAddHTTPS(domains.ReplicatedAppDomain)
}

func proxyRegistryURL() string {
	domains := getDomains()
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
	ctx context.Context, kcli client.Client, flags InstallCmdFlags, rc runtimeconfig.RuntimeConfig,
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

	// record the installation
	installation, err := kubeutils.RecordInstallation(ctx, kcli, kubeutils.RecordInstallationOptions{
		ClusterID:      flags.clusterID,
		IsAirgap:       flags.isAirgap,
		License:        flags.license,
		ConfigSpec:     cfgspec,
		MetricsBaseURL: replicatedAppURL(),
		RuntimeConfig:  rc.Get(),
		EndUserConfig:  eucfg,
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
