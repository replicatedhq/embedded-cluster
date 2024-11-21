package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	eckinds "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/configutils"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/goods"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	k8syaml "sigs.k8s.io/yaml"
)

func InstallCmd(ctx context.Context, name string) *cobra.Command {

	var (
		adminConsolePassword    string
		adminConsolePort        int
		airgapBundle            string
		dataDir                 string
		licenseFile             string
		localArtifactMirrorPort int
		networkInterface        string
		noPrompt                bool
		overrides               string
		privateCAs              []string
		skipHostPreflights      bool
		configValues            string

		proxy *ecv1beta1.ProxySpec
	)

	cmd := &cobra.Command{
		Use:   "install",
		Short: fmt.Sprintf("Install %s", name),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("install command must be run as root")
			}

			var err error
			proxy, err = getProxySpecFromFlags(cmd)
			if err != nil {
				return fmt.Errorf("unable to get proxy spec from flags: %w", err)
			}

			proxy, err = includeLocalIPInNoProxy(cmd, proxy)
			if err != nil {
				licenseFlag, err := cmd.Flags().GetString("license")
				if err != nil {
					return fmt.Errorf("unable to get license flag: %w", err)
				}
				metrics.ReportApplyFinished(cmd.Context(), licenseFlag, err)
				return err
			}
			setProxyEnv(proxy)

			if cmd.Flags().Changed("cidr") && (cmd.Flags().Changed("pod-cidr") || cmd.Flags().Changed("service-cidr")) {
				return fmt.Errorf("--cidr flag can't be used with --pod-cidr or --service-cidr")
			}

			cidr, err := cmd.Flags().GetString("cidr")
			if err != nil {
				return fmt.Errorf("unable to get cidr flag: %w", err)
			}

			if err := netutils.ValidateCIDR(cidr, 16, true); err != nil {
				return fmt.Errorf("invalid cidr %q: %w", cidr, err)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			err = configutils.WriteRuntimeConfig(provider.GetRuntimeConfig())
			if err != nil {
				return fmt.Errorf("unable to write runtime config: %w", err)
			}

			logrus.Debugf("checking if %s is already installed", name)
			installed, err := k0s.IsInstalled(name)
			if err != nil {
				return err
			}
			if installed {
				return ErrNothingElseToAdd
			}

			channelRelease, err := release.GetChannelRelease()
			if err != nil {
				return fmt.Errorf("unable to read channel release data: %w", err)
			}

			if channelRelease != nil && channelRelease.Airgap && airgapBundle == "" && !noPrompt {
				logrus.Warnf("You downloaded an air gap bundle but didn't provide it with --airgap-bundle.")
				logrus.Warnf("If you continue, the installation will not use an air gap bundle and will connect to the internet.")
				if !prompts.New().Confirm("Do you want to proceed with an online installation?", false) {
					return ErrNothingElseToAdd
				}
			}

			metrics.ReportApplyStarted(cmd.Context(), licenseFile)

			logrus.Debugf("configuring sysctl")
			if err := configutils.ConfigureSysctl(provider); err != nil {
				return fmt.Errorf("unable to configure sysctl: %w", err)
			}

			logrus.Debugf("configuring network manager")
			if err := configureNetworkManager(cmd.Context(), provider); err != nil {
				return fmt.Errorf("unable to configure network manager: %w", err)
			}

			logrus.Debugf("checking license matches")
			license, err := getLicenseFromFilepath(licenseFile)
			if err != nil {
				metricErr := fmt.Errorf("unable to get license: %w", err)
				metrics.ReportApplyFinished(cmd.Context(), licenseFile, metricErr)
				return err // do not return the metricErr, as we want the user to see the error message without a prefix
			}
			isAirgap := false
			if airgapBundle != "" {
				isAirgap = true
			}
			if isAirgap {
				logrus.Debugf("checking airgap bundle matches binary")
				if err := checkAirgapMatches(airgapBundle); err != nil {
					return err // we want the user to see the error message without a prefix
				}
			}

			if !isAirgap {
				if err := maybePromptForAppUpdate(cmd.Context(), prompts.New(), license); err != nil {
					if errors.Is(err, ErrNothingElseToAdd) {
						metrics.ReportApplyFinished(cmd.Context(), licenseFile, err)
						return err
					}
					// If we get an error other than ErrNothingElseToAdd, we warn and continue as
					// this check is not critical.
					logrus.Debugf("WARNING: Failed to check for newer app versions: %v", err)
				}
			}

			if err := preflights.ValidateApp(); err != nil {
				metrics.ReportApplyFinished(cmd.Context(), licenseFile, err)
				return err
			}

			adminConsolePwd, err := maybeAskAdminConsolePassword(cmd)
			if err != nil {
				metrics.ReportApplyFinished(cmd.Context(), licenseFile, err)
				return err
			}

			logrus.Debugf("materializing binaries")
			if err := materializeFiles(airgapBundle, provider); err != nil {
				metrics.ReportApplyFinished(cmd.Context(), licenseFile, err)
				return err
			}

			opts := addonsApplierOpts{
				noPrompt:     noPrompt,
				license:      licenseFile,
				airgapBundle: airgapBundle,
				overrides:    overrides,
				privateCAs:   privateCAs,
				configValues: configValues,
			}
			applier, err := getAddonsApplier(cmd, opts, provider.GetRuntimeConfig(), adminConsolePwd, proxy)
			if err != nil {
				metrics.ReportApplyFinished(cmd.Context(), licenseFile, err)
				return err
			}

			logrus.Debugf("running host preflights")
			var replicatedAPIURL, proxyRegistryURL string
			if license != nil {
				replicatedAPIURL = license.Spec.Endpoint
				proxyRegistryURL = fmt.Sprintf("https://%s", defaults.ProxyRegistryAddress)
			}

			fromCIDR, toCIDR, err := DeterminePodAndServiceCIDRs(cmd)
			if err != nil {
				return fmt.Errorf("unable to determine pod and service CIDRs: %w", err)
			}

			if err := RunHostPreflights(cmd, provider, applier, replicatedAPIURL, proxyRegistryURL, isAirgap, proxy, fromCIDR, toCIDR); err != nil {
				metrics.ReportApplyFinished(cmd.Context(), licenseFile, err)
				if err == ErrPreflightsHaveFail {
					return ErrNothingElseToAdd
				}
				return err
			}

			cfg, err := installAndWaitForK0s(cmd, provider, applier, proxy)
			if err != nil {
				return err
			}
			logrus.Debugf("running outro")
			if err := runOutro(cmd, provider, applier, cfg); err != nil {
				metrics.ReportApplyFinished(cmd.Context(), licenseFile, err)
				return err
			}
			metrics.ReportApplyFinished(cmd.Context(), licenseFile, nil)
			return nil
		},
	}

	cmd.Flags().StringVar(&adminConsolePassword, "admin-console-password", "", "Password for the Admin Console")
	cmd.Flags().IntVar(&adminConsolePort, "admin-console-port", ecv1beta1.DefaultAdminConsolePort, "Port on which the Admin Console will be served")
	cmd.Flags().StringVar(&airgapBundle, "airgap-bundle", "", "Path to the air gap bundle. If set, the installation will complete without internet access.")
	cmd.Flags().StringVar(&dataDir, "data-dir", ecv1beta1.DefaultDataDir, "Path to the data directory")
	cmd.Flags().StringVar(&licenseFile, "license", "", "Path to the license file")
	cmd.Flags().IntVar(&localArtifactMirrorPort, "local-artifact-mirror-port", ecv1beta1.DefaultLocalArtifactMirrorPort, "Port on which the Local Artifact Mirror will be served")
	cmd.Flags().StringVar(&networkInterface, "network-interface", "", "The network interface to use for the cluster")
	cmd.Flags().BoolVar(&noPrompt, "no-prompt", false, "Disable interactive prompts.")
	cmd.Flags().StringVar(&overrides, "overrides", "", "File with an EmbeddedClusterConfig object to override the default configuration")
	cmd.Flags().MarkHidden("overrides")
	cmd.Flags().StringSliceVar(&privateCAs, "private-ca", []string{}, "Path to a trusted private CA certificate file")
	cmd.Flags().BoolVar(&skipHostPreflights, "skip-host-preflights", false, "Skip host preflight checks. This is not recommended.")
	cmd.Flags().StringVar(&configValues, "config-values", "", "path to a manifest containing config values (must be apiVersion: kots.io/v1beta1, kind: ConfigValues)")

	addProxyFlags(cmd)
	addCIDRFlags(cmd)

	cmd.AddCommand(InstallRunPreflightsCmd(ctx, name))

	return cmd
}

// configureNetworkManager configures the network manager (if the host is using it) to ignore
// the calico interfaces. This function restarts the NetworkManager service if the configuration
// was changed.
func configureNetworkManager(ctx context.Context, provider *defaults.Provider) error {
	if active, err := helpers.IsSystemdServiceActive(ctx, "NetworkManager"); err != nil {
		return fmt.Errorf("unable to check if NetworkManager is active: %w", err)
	} else if !active {
		logrus.Debugf("NetworkManager is not active, skipping configuration")
		return nil
	}

	dir := "/etc/NetworkManager/conf.d"
	if _, err := os.Stat(dir); err != nil {
		logrus.Debugf("skiping NetworkManager config (%s): %v", dir, err)
		return nil
	}

	logrus.Debugf("creating NetworkManager config file")
	materializer := goods.NewMaterializer(provider)
	if err := materializer.CalicoNetworkManagerConfig(); err != nil {
		return fmt.Errorf("unable to materialize configuration: %w", err)
	}

	logrus.Debugf("network manager config created, restarting the service")
	if _, err := helpers.RunCommand("systemctl", "restart", "NetworkManager"); err != nil {
		return fmt.Errorf("unable to restart network manager: %w", err)
	}
	return nil
}

func checkAirgapMatches(airgapBundle string) error {
	rel, err := release.GetChannelRelease()
	if err != nil {
		return fmt.Errorf("failed to get release from binary: %w", err) // this should only be if the release is malformed
	}
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
func maybePromptForAppUpdate(ctx context.Context, prompt prompts.Prompt, license *kotsv1beta1.License) error {
	channelRelease, err := release.GetChannelRelease()
	if err != nil {
		return fmt.Errorf("unable to get channel release: %w", err)
	} else if channelRelease == nil {
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

	apiURL := metrics.BaseURL(license)
	releaseURL := fmt.Sprintf("%s/embedded/%s/%s", apiURL, channelRelease.AppSlug, channelRelease.ChannelSlug)
	logrus.Warnf("A newer version %s is available.", currentRelease.VersionLabel)
	logrus.Infof(
		"To download it, run:\n  curl -fL \"%s\" \\\n    -H \"Authorization: %s\" \\\n    -o %s-%s.tgz",
		releaseURL,
		license.Spec.LicenseID,
		channelRelease.AppSlug,
		channelRelease.ChannelSlug,
	)

	// if there is no terminal, we don't prompt the user and continue by default.
	if !prompts.IsTerminal() {
		return nil
	}

	text := fmt.Sprintf("Do you want to continue installing %s anyway?", channelRelease.VersionLabel)
	if !prompt.Confirm(text, true) {
		return ErrNothingElseToAdd
	}

	logrus.Debug("User confirmed prompt to continue installing out-of-date release")

	return nil
}

func maybeAskAdminConsolePassword(cmd *cobra.Command) (string, error) {
	defaultPassword := "password"

	adminConsolePasswordFlag, err := cmd.Flags().GetString("admin-console-password")
	if err != nil {
		return "", fmt.Errorf("unable to get admin-console-password flag: %w", err)
	}
	userProvidedPassword := adminConsolePasswordFlag
	// If there's a user provided password we'll try that first
	if userProvidedPassword != "" {
		// Password isn't retyped so we provided it twice
		if !validateAdminConsolePassword(userProvidedPassword, userProvidedPassword) {
			return "", fmt.Errorf("unable to set the Admin Console password")
		}
		return userProvidedPassword, nil
	}
	// No user provided password but prompt is disabled so we set our default password

	noPromptFlag, err := cmd.Flags().GetBool("no-prompt")
	if err != nil {
		return "", fmt.Errorf("unable to get no-prompt flag: %w", err)
	}
	if noPromptFlag {
		logrus.Infof("The Admin Console password is set to %s", defaultPassword)
		return defaultPassword, nil
	}
	maxTries := 3
	for i := 0; i < maxTries; i++ {
		promptA := prompts.New().Password(fmt.Sprintf("Set the Admin Console password (minimum %d characters):", minAdminPasswordLength))
		promptB := prompts.New().Password("Confirm the Admin Console password:")

		if validateAdminConsolePassword(promptA, promptB) {
			return promptA, nil
		}
	}
	return "", fmt.Errorf("unable to set the Admin Console password after %d tries", maxTries)
}

// Minimum character length for the Admin Console password
const minAdminPasswordLength = 6

func validateAdminConsolePassword(password, passwordCheck string) bool {
	if password != passwordCheck {
		logrus.Info("Passwords don't match. Please try again.")
		return false
	}
	if len(password) < minAdminPasswordLength {
		logrus.Infof("Passwords must have more than %d characters. Please try again.", minAdminPasswordLength)
		return false
	}
	return true
}

// installAndWaitForK0s installs the k0s binary and waits for it to be ready
func installAndWaitForK0s(cmd *cobra.Command, provider *defaults.Provider, applier *addons.Applier, proxy *ecv1beta1.ProxySpec) (*k0sconfig.ClusterConfig, error) {
	loading := spinner.Start()
	defer loading.Close()
	loading.Infof("Installing %s node", defaults.BinaryName())
	logrus.Debugf("creating k0s configuration file")

	licenseFlag, err := cmd.Flags().GetString("license")
	if err != nil {
		return nil, fmt.Errorf("unable to get license flag: %w", err)
	}
	cfg, err := ensureK0sConfig(cmd, provider, applier)
	if err != nil {
		err := fmt.Errorf("unable to create config file: %w", err)
		metrics.ReportApplyFinished(cmd.Context(), licenseFlag, err)
		return nil, err
	}
	logrus.Debugf("creating systemd unit files")
	if err := createSystemdUnitFiles(provider, false, proxy); err != nil {
		err := fmt.Errorf("unable to create systemd unit files: %w", err)
		metrics.ReportApplyFinished(cmd.Context(), licenseFlag, err)
		return nil, err
	}

	logrus.Debugf("installing k0s")
	networkInterface, err := cmd.Flags().GetString("network-interface")
	if err != nil {
		return nil, fmt.Errorf("unable to get network-interface flag: %w", err)
	}
	if err := k0s.Install(networkInterface, provider); err != nil {
		err := fmt.Errorf("unable to install cluster: %w", err)
		metrics.ReportApplyFinished(cmd.Context(), licenseFlag, err)
		return nil, err
	}
	loading.Infof("Waiting for %s node to be ready", defaults.BinaryName())
	logrus.Debugf("waiting for k0s to be ready")
	if err := waitForK0s(); err != nil {
		err := fmt.Errorf("unable to wait for node: %w", err)
		metrics.ReportApplyFinished(cmd.Context(), licenseFlag, err)
		return nil, err
	}

	loading.Infof("Node installation finished!")
	return cfg, nil
}

// runOutro calls Outro() in all enabled addons by means of Applier.
func runOutro(cmd *cobra.Command, provider *defaults.Provider, applier *addons.Applier, cfg *k0sconfig.ClusterConfig) error {
	os.Setenv("KUBECONFIG", provider.PathToKubeConfig())

	// This metadata should be the same as the artifact from the release without the vendor customizations
	defaultCfg := config.RenderK0sConfig()
	metadata, err := gatherVersionMetadata(defaultCfg, false)
	if err != nil {
		return fmt.Errorf("unable to gather release metadata: %w", err)
	}

	overridesFlag, err := cmd.Flags().GetString("overrides")
	if err != nil {
		return fmt.Errorf("unable to get overrides flag: %w", err)
	}
	eucfg, err := helpers.ParseEndUserConfig(overridesFlag)
	if err != nil {
		return fmt.Errorf("unable to process overrides file: %w", err)
	}

	networkInterfaceFlag, err := cmd.Flags().GetString("network-interface")
	if err != nil {
		return fmt.Errorf("unable to get network-interface flag: %w", err)
	}
	return applier.Outro(cmd.Context(), cfg, eucfg, metadata, networkInterfaceFlag)
}

// gatherVersionMetadata returns the release metadata for this version of
// embedded cluster. Release metadata involves the default versions of the
// components that are included in the release plus the default values used
// when deploying them.
func gatherVersionMetadata(k0sCfg *k0sconfig.ClusterConfig, withChannelRelease bool) (*types.ReleaseMetadata, error) {
	applier := addons.NewApplier(
		addons.WithoutPrompt(),
		addons.OnlyDefaults(),
		addons.Quiet(),
	)

	additionalCharts := []eckinds.Chart{}
	additionalRepos := []k0sconfig.Repository{}
	if withChannelRelease {
		additionalCharts = config.AdditionalCharts()
		additionalRepos = config.AdditionalRepositories()
	}

	versionsMap, err := applier.Versions(additionalCharts)
	if err != nil {
		return nil, fmt.Errorf("unable to get versions: %w", err)
	}
	versionsMap["Kubernetes"] = versions.K0sVersion
	versionsMap["Installer"] = versions.Version
	versionsMap["Troubleshoot"] = versions.TroubleshootVersion

	if withChannelRelease {
		channelRelease, err := release.GetChannelRelease()
		if err == nil && channelRelease != nil {
			versionsMap[defaults.BinaryName()] = channelRelease.VersionLabel
		}
	}

	sha, err := goods.K0sBinarySHA256()
	if err != nil {
		return nil, fmt.Errorf("unable to get k0s binary sha256: %w", err)
	}

	artifacts := map[string]string{
		"k0s":                         fmt.Sprintf("k0s-binaries/%s-%s", versions.K0sVersion, runtime.GOARCH),
		"kots":                        fmt.Sprintf("kots-binaries/%s-%s.tar.gz", adminconsole.KotsVersion, runtime.GOARCH),
		"operator":                    fmt.Sprintf("operator-binaries/%s-%s.tar.gz", embeddedclusteroperator.Metadata.Version, runtime.GOARCH),
		"local-artifact-mirror-image": versions.LocalArtifactMirrorImage,
	}
	if versions.K0sBinaryURLOverride != "" {
		artifacts["k0s"] = versions.K0sBinaryURLOverride
	}
	if versions.KOTSBinaryURLOverride != "" {
		artifacts["kots"] = versions.KOTSBinaryURLOverride
	}
	if versions.OperatorBinaryURLOverride != "" {
		artifacts["operator"] = versions.OperatorBinaryURLOverride
	}

	meta := types.ReleaseMetadata{
		Versions:  versionsMap,
		K0sSHA:    sha,
		Artifacts: artifacts,
	}

	chtconfig, repconfig, err := applier.GenerateHelmConfigs(
		k0sCfg,
		additionalCharts,
		additionalRepos,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to apply addons: %w", err)
	}

	meta.Configs = eckinds.Helm{
		ConcurrencyLevel: 1,
		Charts:           chtconfig,
		Repositories:     repconfig,
	}

	protectedFields, err := applier.ProtectedFields()
	if err != nil {
		return nil, fmt.Errorf("unable to get protected fields: %w", err)
	}
	meta.Protected = protectedFields

	// Additional builtin addons
	builtinCharts, err := applier.GetBuiltinCharts(k0sCfg)
	if err != nil {
		return nil, fmt.Errorf("unable to get builtin charts: %w", err)
	}
	meta.BuiltinConfigs = builtinCharts

	meta.K0sImages = config.ListK0sImages(k0sCfg)

	additionalImages, err := applier.GetAdditionalImages()
	if err != nil {
		return nil, fmt.Errorf("unable to get additional images: %w", err)
	}
	meta.K0sImages = append(meta.K0sImages, additionalImages...)

	meta.K0sImages = helpers.UniqueStringSlice(meta.K0sImages)
	sort.Strings(meta.K0sImages)

	meta.Images = config.ListK0sImages(k0sCfg)

	images, err := applier.GetImages()
	if err != nil {
		return nil, fmt.Errorf("unable to get images: %w", err)
	}
	meta.Images = append(meta.Images, images...)

	meta.Images = append(meta.Images, versions.LocalArtifactMirrorImage)

	meta.Images = helpers.UniqueStringSlice(meta.Images)
	sort.Strings(meta.Images)

	return &meta, nil
}

// createK0sConfig creates a new k0s.yaml configuration file. The file is saved in the
// global location (as returned by defaults.PathToK0sConfig()). If a file already sits
// there, this function returns an error.
func ensureK0sConfig(cmd *cobra.Command, provider *defaults.Provider, applier *addons.Applier) (*k0sconfig.ClusterConfig, error) {
	cfgpath := defaults.PathToK0sConfig()
	if _, err := os.Stat(cfgpath); err == nil {
		return nil, fmt.Errorf("configuration file already exists")
	}
	if err := os.MkdirAll(filepath.Dir(cfgpath), 0755); err != nil {
		return nil, fmt.Errorf("unable to create directory: %w", err)
	}
	cfg := config.RenderK0sConfig()

	networkInterfaceFlag, err := cmd.Flags().GetString("network-interface")
	if err != nil {
		return nil, fmt.Errorf("unable to get network-interface flag: %w", err)
	}
	address, err := netutils.FirstValidAddress(networkInterfaceFlag)
	if err != nil {
		return nil, fmt.Errorf("unable to find first valid address: %w", err)
	}
	cfg.Spec.API.Address = address
	cfg.Spec.Storage.Etcd.PeerAddress = address

	podCIDR, serviceCIDR, err := DeterminePodAndServiceCIDRs(cmd)
	if err != nil {
		return nil, fmt.Errorf("unable to determine pod and service CIDRs: %w", err)
	}
	cfg.Spec.Network.PodCIDR = podCIDR
	cfg.Spec.Network.ServiceCIDR = serviceCIDR
	if err := config.UpdateHelmConfigs(applier, cfg); err != nil {
		return nil, fmt.Errorf("unable to update helm configs: %w", err)
	}
	cfg, err = applyUnsupportedOverrides(cmd, cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to apply unsupported overrides: %w", err)
	}

	airgapBundleFlag, err := cmd.Flags().GetString("airgap-bundle")
	if err != nil {
		return nil, fmt.Errorf("unable to get airgap-bundle flag: %w", err)
	}
	if airgapBundleFlag != "" {
		// update the k0s config to install with airgap
		airgap.RemapHelm(provider, cfg)
		airgap.SetAirgapConfig(cfg)
	}
	// This is necessary to install the previous version of k0s in e2e tests
	// TODO: remove this once the previous version is > 1.29
	unstructured, err := helpers.K0sClusterConfigTo129Compat(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to convert cluster config to 1.29 compat: %w", err)
	}
	data, err := k8syaml.Marshal(unstructured)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal config: %w", err)
	}
	if err := os.WriteFile(cfgpath, data, 0600); err != nil {
		return nil, fmt.Errorf("unable to write config file: %w", err)
	}
	return cfg, nil
}

// applyUnsupportedOverrides applies overrides to the k0s configuration. Applies first the
// overrides embedded into the binary and after the ones provided by the user (--overrides).
// we first apply the k0s config override and then apply the built in overrides.
func applyUnsupportedOverrides(cmd *cobra.Command, cfg *k0sconfig.ClusterConfig) (*k0sconfig.ClusterConfig, error) {
	embcfg, err := release.GetEmbeddedClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to get embedded cluster config: %w", err)
	}
	if embcfg != nil {
		overrides := embcfg.Spec.UnsupportedOverrides.K0s
		cfg, err = config.PatchK0sConfig(cfg, overrides)
		if err != nil {
			return nil, fmt.Errorf("unable to patch k0s config: %w", err)
		}
		cfg, err = config.ApplyBuiltInExtensionsOverrides(cfg, embcfg)
		if err != nil {
			return nil, fmt.Errorf("unable to release built in overrides: %w", err)
		}
	}

	overridesFlag, err := cmd.Flags().GetString("overrides")
	if err != nil {
		return nil, fmt.Errorf("unable to get overrides flag: %w", err)
	}
	eucfg, err := helpers.ParseEndUserConfig(overridesFlag)
	if err != nil {
		return nil, fmt.Errorf("unable to process overrides file: %w", err)
	}
	if eucfg != nil {
		overrides := eucfg.Spec.UnsupportedOverrides.K0s
		cfg, err = config.PatchK0sConfig(cfg, overrides)
		if err != nil {
			return nil, fmt.Errorf("unable to apply overrides: %w", err)
		}
		cfg, err = config.ApplyBuiltInExtensionsOverrides(cfg, eucfg)
		if err != nil {
			return nil, fmt.Errorf("unable to end user built in overrides: %w", err)
		}
	}

	return cfg, nil
}

// DeterminePodAndServiceCIDRS determines, based on the command line flags,
// what are the pod and service CIDRs to be used for the cluster. If both
// --pod-cidr and --service-cidr have been set, they are used. Otherwise,
// the cidr flag is split into pod and service CIDRs.
func DeterminePodAndServiceCIDRs(cmd *cobra.Command) (string, string, error) {
	if cmd.Flags().Changed("pod-cidr") || cmd.Flags().Changed("service-cidr") {
		podCIDRFlag, err := cmd.Flags().GetString("pod-cidr")
		if err != nil {
			return "", "", fmt.Errorf("unable to get pod-cidr flag: %w", err)
		}
		serviceCIDRFlag, err := cmd.Flags().GetString("service-cidr")
		if err != nil {
			return "", "", fmt.Errorf("unable to get service-cidr flag: %w", err)
		}
		return podCIDRFlag, serviceCIDRFlag, nil
	}

	cidrFlag, err := cmd.Flags().GetString("cidr")
	if err != nil {
		return "", "", fmt.Errorf("unable to get cidr flag: %w", err)
	}
	return netutils.SplitNetworkCIDR(cidrFlag)
}

// createSystemdUnitFiles links the k0s systemd unit file. this also creates a new
// systemd unit file for the local artifact mirror service.
func createSystemdUnitFiles(provider *defaults.Provider, isWorker bool, proxy *ecv1beta1.ProxySpec) error {
	dst := systemdUnitFileName()
	if _, err := os.Lstat(dst); err == nil {
		if err := os.Remove(dst); err != nil {
			return err
		}
	}
	src := "/etc/systemd/system/k0scontroller.service"
	if isWorker {
		src = "/etc/systemd/system/k0sworker.service"
	}
	if proxy != nil {
		if err := ensureProxyConfig(fmt.Sprintf("%s.d", src), proxy.HTTPProxy, proxy.HTTPSProxy, proxy.NoProxy); err != nil {
			return fmt.Errorf("unable to create proxy config: %w", err)
		}
	}
	logrus.Debugf("linking %s to %s", src, dst)
	if err := os.Symlink(src, dst); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	if _, err := helpers.RunCommand("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("unable to get reload systemctl daemon: %w", err)
	}
	if err := installAndEnableLocalArtifactMirror(provider); err != nil {
		return fmt.Errorf("unable to install and enable local artifact mirror: %w", err)
	}
	return nil
}

func systemdUnitFileName() string {
	return fmt.Sprintf("/etc/systemd/system/%s.service", defaults.BinaryName())
}

// ensureProxyConfig creates a new http-proxy.conf configuration file. The file is saved in the
// systemd directory (/etc/systemd/system/k0scontroller.service.d/).
func ensureProxyConfig(servicePath string, httpProxy string, httpsProxy string, noProxy string) error {
	// create the directory
	if err := os.MkdirAll(servicePath, 0755); err != nil {
		return fmt.Errorf("unable to create directory: %w", err)
	}

	// create and write the file
	content := fmt.Sprintf(`[Service]
Environment="HTTP_PROXY=%s"
Environment="HTTPS_PROXY=%s"
Environment="NO_PROXY=%s"`, httpProxy, httpsProxy, noProxy)

	err := os.WriteFile(filepath.Join(servicePath, "http-proxy.conf"), []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("unable to create and write proxy file: %w", err)
	}

	return nil
}

// installAndEnableLocalArtifactMirror installs and enables the local artifact mirror. This
// service is responsible for serving on localhost, through http, all files that are used
// during a cluster upgrade.
func installAndEnableLocalArtifactMirror(provider *defaults.Provider) error {
	materializer := goods.NewMaterializer(provider)
	if err := materializer.LocalArtifactMirrorUnitFile(); err != nil {
		return fmt.Errorf("failed to materialize artifact mirror unit: %w", err)
	}
	if err := writeLocalArtifactMirrorDropInFile(provider); err != nil {
		return fmt.Errorf("failed to write local artifact mirror environment file: %w", err)
	}
	if _, err := helpers.RunCommand("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("unable to get reload systemctl daemon: %w", err)
	}
	if _, err := helpers.RunCommand("systemctl", "start", "local-artifact-mirror"); err != nil {
		return fmt.Errorf("unable to start the local artifact mirror: %w", err)
	}
	if _, err := helpers.RunCommand("systemctl", "enable", "local-artifact-mirror"); err != nil {
		return fmt.Errorf("unable to start the local artifact mirror service: %w", err)
	}
	return nil
}

const (
	localArtifactMirrorSystemdConfFile    = "/etc/systemd/system/local-artifact-mirror.service.d/embedded-cluster.conf"
	localArtifactMirrorDropInFileContents = `[Service]
Environment="LOCAL_ARTIFACT_MIRROR_PORT=%d"
Environment="LOCAL_ARTIFACT_MIRROR_DATA_DIR=%s"
# Empty ExecStart= will clear out the previous ExecStart value
ExecStart=
ExecStart=%s serve
`
)

func writeLocalArtifactMirrorDropInFile(provider *defaults.Provider) error {
	dir := filepath.Dir(localArtifactMirrorSystemdConfFile)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	contents := fmt.Sprintf(
		localArtifactMirrorDropInFileContents,
		provider.LocalArtifactMirrorPort(),
		provider.EmbeddedClusterHomeDirectory(),
		provider.PathToEmbeddedClusterBinary("local-artifact-mirror"),
	)
	err = os.WriteFile(localArtifactMirrorSystemdConfFile, []byte(contents), 0644)
	if err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

// waitForK0s waits for the k0s API to be available. We wait for the k0s socket to
// appear in the system and until the k0s status command to finish.
func waitForK0s() error {
	if !dryrun.Enabled() {
		var success bool
		for i := 0; i < 30; i++ {
			time.Sleep(2 * time.Second)
			spath := defaults.PathToK0sStatusSocket()
			if _, err := os.Stat(spath); err != nil {
				continue
			}
			success = true
			break
		}
		if !success {
			return fmt.Errorf("timeout waiting for %s", defaults.BinaryName())
		}
	}

	for i := 1; ; i++ {
		_, err := helpers.RunCommand(defaults.K0sBinaryPath(), "status")
		if err == nil {
			return nil
		} else if i == 30 {
			return fmt.Errorf("unable to get status: %w", err)
		}
		time.Sleep(2 * time.Second)
	}
}
