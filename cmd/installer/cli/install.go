package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/gorilla/mux"
	"github.com/gosimple/slug"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/api"
	installclient "github.com/replicatedhq/embedded-cluster/api/client/install"
	"github.com/replicatedhq/embedded-cluster/api/models"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/goods"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/configutils"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/extensions"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers/systemd"
	"github.com/replicatedhq/embedded-cluster/pkg/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/netutil"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/preflights"
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
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type installCmdFlags struct {
	configValues         string
	privateCAs           []string
	skipHostPreflights   bool
	ignoreHostPreflights bool
	airgapBundle         string
	assumeYes            bool
	licenseFile          string
	license              *kotsv1beta1.License
}

// InstallCmd returns a cobra command for installing the embedded cluster.
func InstallCmd(ctx context.Context, name string) *cobra.Command {
	var installConfig models.InstallationConfig
	var cliFlags installCmdFlags

	ctx, cancel := context.WithCancel(ctx)

	cmd := &cobra.Command{
		Use:   "install",
		Short: fmt.Sprintf("Install %s", name),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := preRunInstall(cmd, &installConfig, &cliFlags); err != nil {
				return err
			}
			if err := ensureAdminConsolePassword(&installConfig, &cliFlags); err != nil {
				return err
			}

			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			runtimeconfig.Cleanup()
			cancel() // Cancel context when command completes
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterID := metrics.ClusterID()
			metricsReporter := NewInstallReporter(
				replicatedAppURL(), clusterID, cmd.CalledAs(), flagsToStringSlice(cmd.Flags()),
				cliFlags.license.Spec.LicenseID, cliFlags.license.Spec.AppSlug,
			)
			metricsReporter.ReportInstallationStarted(ctx)

			// Setup signal handler with the metrics reporter cleanup function
			signalHandler(ctx, cancel, func(ctx context.Context, sig os.Signal) {
				metricsReporter.ReportSignalAborted(ctx, sig)
			})

			if err := runInstall(cmd.Context(), name, installConfig, cliFlags, metricsReporter); err != nil {
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

	if err := addInstallConfigFlags(cmd, &installConfig); err != nil {
		panic(err)
	}
	if err := addInstallCmdFlags(cmd, &cliFlags); err != nil {
		panic(err)
	}

	cmd.AddCommand(InstallRunPreflightsCmd(ctx, name))

	return cmd
}

func addInstallConfigFlags(cmd *cobra.Command, installConfig *models.InstallationConfig) error {
	cmd.Flags().StringVar(&installConfig.DataDirectory, "data-dir", ecv1beta1.DefaultDataDir, "Path to the data directory")
	cmd.Flags().IntVar(&installConfig.LocalArtifactMirrorPort, "local-artifact-mirror-port", ecv1beta1.DefaultLocalArtifactMirrorPort, "Port on which the Local Artifact Mirror will be served")
	cmd.Flags().StringVar(&installConfig.NetworkInterface, "network-interface", "", "The network interface to use for the cluster")

	cmd.Flags().StringVar(&installConfig.EndUserConfigOverrides, "overrides", "", "File with an EmbeddedClusterConfig object to override the default configuration")
	if err := cmd.Flags().MarkHidden("overrides"); err != nil {
		return err
	}

	cmd.Flags().StringVar(&installConfig.AdminConsolePassword, "admin-console-password", "", "Password for the Admin Console")
	cmd.Flags().IntVar(&installConfig.AdminConsolePort, "admin-console-port", ecv1beta1.DefaultAdminConsolePort, "Port on which the Admin Console will be served")

	if err := addProxyFlags(cmd, installConfig); err != nil {
		return err
	}
	if err := addCIDRFlags(cmd); err != nil {
		return err
	}

	return nil
}

func addInstallCmdFlags(cmd *cobra.Command, cliFlags *installCmdFlags) error {
	cmd.Flags().StringVar(&cliFlags.airgapBundle, "airgap-bundle", "", "Path to the air gap bundle. If set, the installation will complete without internet access.")

	cmd.Flags().BoolVarP(&cliFlags.assumeYes, "yes", "y", false, "Assume yes to all prompts.")
	cmd.Flags().SetNormalizeFunc(normalizeNoPromptToYes)

	cmd.Flags().StringSliceVar(&cliFlags.privateCAs, "private-ca", []string{}, "Path to a trusted private CA certificate file")

	cmd.Flags().BoolVar(&cliFlags.skipHostPreflights, "skip-host-preflights", false, "Skip host preflight checks. This is not recommended and has been deprecated.")
	if err := cmd.Flags().MarkHidden("skip-host-preflights"); err != nil {
		return err
	}
	if err := cmd.Flags().MarkDeprecated("skip-host-preflights", "This flag is deprecated and will be removed in a future version. Use --ignore-host-preflights instead."); err != nil {
		return err
	}

	cmd.Flags().BoolVar(&cliFlags.ignoreHostPreflights, "ignore-host-preflights", false, "Allow bypassing host preflight failures")

	cmd.Flags().StringVarP(&cliFlags.licenseFile, "license", "l", "", "Path to the license file")
	if err := cmd.MarkFlagRequired("license"); err != nil {
		return err
	}

	cmd.Flags().StringVar(&cliFlags.configValues, "config-values", "", "Path to the config values to use when installing")

	return nil
}

func preRunInstall(cmd *cobra.Command, installConfig *models.InstallationConfig, cliFlags *installCmdFlags) error {
	if os.Getuid() != 0 {
		return fmt.Errorf("install command must be run as root")
	}

	// set the umask to 022 so that we can create files/directories with 755 permissions
	// this does not return an error - it returns the previous umask
	_ = syscall.Umask(0o022)

	if err := validateCIDRFlags(cmd); err != nil {
		return err
	}

	// parse the various cidr flags to make sure we have exactly what we want
	if err := parseCIDRFlags(cmd, installConfig); err != nil {
		return fmt.Errorf("unable to parse cidr flags: %w", err)
	}

	// license file can be empty for restore
	if cliFlags.licenseFile != "" {
		// validate the the license is indeed a license file
		l, err := helpers.ParseLicense(cliFlags.licenseFile)
		if err != nil {
			if err == helpers.ErrNotALicenseFile {
				return fmt.Errorf("license file is not a valid license file")
			}

			return fmt.Errorf("unable to parse license file: %w", err)
		}
		cliFlags.license = l
	}

	if cliFlags.configValues != "" {
		err := configutils.ValidateKotsConfigValues(cliFlags.configValues)
		if err != nil {
			return fmt.Errorf("config values file is not valid: %w", err)
		}
	}

	return nil
}

func runInstallAPI(ctx context.Context, listener net.Listener, logger logrus.FieldLogger) error {
	router := mux.NewRouter()

	api, err := api.New(api.WithLogger(logger))
	if err != nil {
		return fmt.Errorf("new api: %w", err)
	}
	api.RegisterRoutes(router.PathPrefix("/api").Subrouter())

	webFs := http.FileServer(http.FS(web.Fs()))
	router.Handle("/", webFs).Methods("GET")

	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods("GET")

	server := &http.Server{
		Handler: router,
	}

	go func() {
		<-ctx.Done()
		logrus.Debugf("Shutting down install API")
		server.Shutdown(context.Background())
	}()

	logrus.Debugf("Install API listening on %s", listener.Addr().String())
	return server.Serve(listener)
}

func waitForInstallAPI(ctx context.Context, addr string) error {
	httpClient := http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			Proxy: nil,
		},
	}
	timeout := time.After(10 * time.Second)
	var lastErr error
	for {
		select {
		case <-timeout:
			if lastErr != nil {
				return fmt.Errorf("install API did not start in time: %w", lastErr)
			}
			return fmt.Errorf("install API did not start in time")
		case <-time.Tick(1 * time.Second):
			resp, err := httpClient.Get(fmt.Sprintf("http://%s/health", addr))
			if err != nil {
				lastErr = fmt.Errorf("unable to connect to install API: %w", err)
			} else if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
	}
}

func initializeInstallAPIConfig(in models.InstallationConfig, addr string) (*models.InstallationConfig, error) {
	client := installclient.New(fmt.Sprintf("http://%s/api", addr))

	install, err := client.InstallPhaseSetConfig(in)
	if err != nil {
		return nil, fmt.Errorf("upsert config: %w", err)
	}

	initEnvironmentFromRuntimeConfig()

	return &install.Config, nil
}

func initEnvironmentFromRuntimeConfig() {
	os.Setenv("KUBECONFIG", runtimeconfig.PathToKubeConfig())
	os.Setenv("TMPDIR", runtimeconfig.EmbeddedClusterTmpSubDir())

	proxySpec := runtimeconfig.ProxySpec()

	if proxySpec != nil {
		if proxySpec.HTTPProxy != "" {
			os.Setenv("HTTP_PROXY", proxySpec.HTTPProxy)
		}
		if proxySpec.HTTPSProxy != "" {
			os.Setenv("HTTPS_PROXY", proxySpec.HTTPSProxy)
		}
		if proxySpec.NoProxy != "" {
			os.Setenv("NO_PROXY", proxySpec.NoProxy)
		}
	}

	if err := os.Chmod(runtimeconfig.EmbeddedClusterHomeDirectory(), 0755); err != nil {
		// don't fail as there are cases where we can't change the permissions (bind mounts, selinux, etc...),
		// and we handle and surface those errors to the user later (host preflights, checking exec errors, etc...)
		logrus.Debugf("unable to chmod embedded-cluster home dir: %s", err)
	}
}

func runInstall(ctx context.Context, name string, inInstallConfig models.InstallationConfig, cliFlags installCmdFlags, metricsReporter preflights.MetricsReporter) error {
	logger, err := api.NewLogger()
	if err != nil {
		logrus.Warnf("Unable to setup API logging: %v", err)
	}

	listener, err := net.Listen("tcp", ":30080")
	if err != nil {
		return fmt.Errorf("unable to create listener: %w", err)
	}

	apiCtx, apiCancel := context.WithCancel(ctx)
	defer apiCancel()
	go runInstallAPI(apiCtx, listener, logger)

	if err := waitForInstallAPI(ctx, listener.Addr().String()); err != nil {
		return fmt.Errorf("unable to wait for install API: %w", err)
	}

	installConfig, err := initializeInstallAPIConfig(inInstallConfig, listener.Addr().String())
	if err != nil {
		return fmt.Errorf("unable to initialize install API config: %w", err)
	}

	return doInstall(ctx, name, *installConfig, cliFlags, metricsReporter)
}

func doInstall(ctx context.Context, name string, installConfig models.InstallationConfig, cliFlags installCmdFlags, metricsReporter preflights.MetricsReporter) error {
	if err := runInstallVerifyAndPrompt(ctx, name, installConfig, cliFlags); err != nil {
		return err
	}

	logrus.Debug("initializing install")
	if err := initializeInstall(ctx, installConfig, cliFlags); err != nil {
		return fmt.Errorf("unable to initialize install: %w", err)
	}

	logrus.Debugf("running install preflights")
	if err := runInstallPreflights(ctx, installConfig, cliFlags, metricsReporter); err != nil {
		if errors.Is(err, preflights.ErrPreflightsHaveFail) {
			return NewErrorNothingElseToAdd(err)
		}
		return fmt.Errorf("unable to run install preflights: %w", err)
	}

	k0sCfg, err := installAndStartCluster(ctx, installConfig, cliFlags.airgapBundle, nil)
	if err != nil {
		return fmt.Errorf("unable to install cluster: %w", err)
	}

	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	errCh := kubeutils.WaitForKubernetes(ctx, kcli)
	defer logKubernetesErrors(errCh)

	in, err := recordInstallation(ctx, kcli, installConfig, cliFlags, k0sCfg)
	if err != nil {
		return fmt.Errorf("unable to record installation: %w", err)
	}

	if err := createVersionMetadataConfigmap(ctx, kcli); err != nil {
		return fmt.Errorf("unable to create version metadata configmap: %w", err)
	}

	// TODO (@salah): update installation status to reflect what's happening

	logrus.Debugf("adding insecure registry")
	registryIP, err := registry.GetRegistryClusterIP(installConfig.ServiceCIDR)
	if err != nil {
		return fmt.Errorf("unable to get registry cluster IP: %w", err)
	}
	if err := airgap.AddInsecureRegistry(fmt.Sprintf("%s:5000", registryIP)); err != nil {
		return fmt.Errorf("unable to add insecure registry: %w", err)
	}

	var embCfgSpec *ecv1beta1.ConfigSpec
	if embCfg := release.GetEmbeddedClusterConfig(); embCfg != nil {
		embCfgSpec = &embCfg.Spec
	}

	euCfg, err := helpers.ParseEndUserConfig(installConfig.EndUserConfigOverrides)
	if err != nil {
		return fmt.Errorf("unable to process overrides file: %w", err)
	}
	var euCfgSpec *ecv1beta1.ConfigSpec
	if euCfg != nil {
		euCfgSpec = &euCfg.Spec
	}

	airgapChartsPath := ""
	if cliFlags.airgapBundle != "" {
		airgapChartsPath = runtimeconfig.EmbeddedClusterChartsSubDir()
	}

	hcli, err := helm.NewClient(helm.HelmOptions{
		KubeConfig: runtimeconfig.PathToKubeConfig(),
		K0sVersion: versions.K0sVersion,
		AirgapPath: airgapChartsPath,
	})
	if err != nil {
		return fmt.Errorf("unable to create helm client: %w", err)
	}
	defer hcli.Close()

	logrus.Debugf("installing addons")
	if err := addons.Install(ctx, hcli, addons.InstallOptions{
		AdminConsolePwd:         installConfig.AdminConsolePassword,
		License:                 cliFlags.license,
		IsAirgap:                cliFlags.airgapBundle != "",
		Proxy:                   runtimeconfig.ProxySpec(),
		HostCABundlePath:        runtimeconfig.HostCABundlePath(),
		PrivateCAs:              cliFlags.privateCAs,
		ServiceCIDR:             installConfig.ServiceCIDR,
		DisasterRecoveryEnabled: cliFlags.license.Spec.IsDisasterRecoverySupported,
		IsMultiNodeEnabled:      cliFlags.license.Spec.IsEmbeddedClusterMultiNodeEnabled,
		EmbeddedConfigSpec:      embCfgSpec,
		EndUserConfigSpec:       euCfgSpec,
		KotsInstaller: func(msg *spinner.MessageWriter) error {
			opts := kotscli.InstallOptions{
				AppSlug:               cliFlags.license.Spec.AppSlug,
				LicenseFile:           cliFlags.licenseFile,
				Namespace:             runtimeconfig.KotsadmNamespace,
				AirgapBundle:          cliFlags.airgapBundle,
				ConfigValuesFile:      cliFlags.configValues,
				ReplicatedAppEndpoint: replicatedAppURL(),
			}
			return kotscli.Install(opts, msg)
		},
	}); err != nil {
		return fmt.Errorf("unable to install addons: %w", err)
	}

	logrus.Debugf("installing extensions")
	if err := extensions.Install(ctx, hcli); err != nil {
		return fmt.Errorf("unable to install extensions: %w", err)
	}

	if err := kubeutils.SetInstallationState(ctx, kcli, in, ecv1beta1.InstallationStateInstalled, "Installed"); err != nil {
		return fmt.Errorf("unable to update installation: %w", err)
	}

	if err = support.CreateHostSupportBundle(); err != nil {
		logrus.Warnf("Unable to create host support bundle: %v", err)
	}

	if err := printSuccessMessage(cliFlags.license, installConfig.NetworkInterface); err != nil {
		return err
	}

	return nil
}

func runInstallVerifyAndPrompt(ctx context.Context, name string, installConfig models.InstallationConfig, cliFlags installCmdFlags) error {
	logrus.Debugf("checking if k0s is already installed")
	err := verifyNoInstallation(name, "reinstall")
	if err != nil {
		return err
	}

	isAirgap := cliFlags.airgapBundle != ""

	err = verifyChannelRelease("installation", isAirgap, cliFlags.assumeYes)
	if err != nil {
		return err
	}

	logrus.Debugf("checking license matches")
	license, err := getLicenseFromFilepath(cliFlags.licenseFile)
	if err != nil {
		return err
	}

	if isAirgap {
		logrus.Debugf("checking airgap bundle matches binary")
		if err := checkAirgapMatches(cliFlags.airgapBundle); err != nil {
			return err // we want the user to see the error message without a prefix
		}
	}

	if !isAirgap {
		if err := maybePromptForAppUpdate(ctx, prompts.New(), license, cliFlags.assumeYes); err != nil {
			if errors.As(err, &ErrorNothingElseToAdd{}) {
				return err
			}
			// If we get an error other than ErrorNothingElseToAdd, we warn and continue as this
			// check is not critical.
			logrus.Debugf("WARNING: Failed to check for newer app versions: %v", err)
		}
	}

	if err := preflights.ValidateApp(); err != nil {
		return err
	}

	return nil
}

func ensureAdminConsolePassword(installConfig *models.InstallationConfig, cliFlags *installCmdFlags) error {
	if installConfig.AdminConsolePassword == "" {
		// no password was provided
		if cliFlags.assumeYes {
			logrus.Infof("\nThe Admin Console password is set to %q.", "password")
			installConfig.AdminConsolePassword = "password"
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
					installConfig.AdminConsolePassword = promptA
					return nil
				}
			}
			return NewErrorNothingElseToAdd(errors.New("password is not valid"))
		}
	}

	if !validateAdminConsolePassword(installConfig.AdminConsolePassword, installConfig.AdminConsolePassword) {
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

func initializeInstall(ctx context.Context, installConfig models.InstallationConfig, cliFlags installCmdFlags) error {
	logrus.Info("")
	spinner := spinner.Start()
	spinner.Infof("Initializing")

	if err := materializeFiles(cliFlags.airgapBundle); err != nil {
		spinner.ErrorClosef("Initialization failed")
		return fmt.Errorf("unable to materialize files: %w", err)
	}

	logrus.Debugf("copy license file to %s", installConfig.DataDirectory)
	if err := copyLicenseFileToDataDir(cliFlags.licenseFile, installConfig.DataDirectory); err != nil {
		// We have decided not to report this error
		logrus.Warnf("Unable to copy license file to %s: %v", installConfig.DataDirectory, err)
	}

	logrus.Debugf("configuring sysctl")
	if err := configutils.ConfigureSysctl(); err != nil {
		logrus.Debugf("unable to configure sysctl: %v", err)
	}

	logrus.Debugf("configuring kernel modules")
	if err := configutils.ConfigureKernelModules(); err != nil {
		logrus.Debugf("unable to configure kernel modules: %v", err)
	}

	logrus.Debugf("configuring network manager")
	if err := configureNetworkManager(ctx); err != nil {
		spinner.ErrorClosef("Initialization failed")
		return fmt.Errorf("unable to configure network manager: %w", err)
	}

	logrus.Debugf("configuring firewalld")
	if err := configureFirewalld(ctx, installConfig.PodCIDR, installConfig.ServiceCIDR); err != nil {
		logrus.Debugf("unable to configure firewalld: %v", err)
	}

	spinner.Closef("Initialization complete")
	return nil
}

func materializeFiles(airgapBundle string) error {
	materializer := goods.NewMaterializer()
	if err := materializer.Materialize(); err != nil {
		return fmt.Errorf("materialize binaries: %w", err)
	}
	if err := support.MaterializeSupportBundleSpec(); err != nil {
		return fmt.Errorf("materialize support bundle spec: %w", err)
	}

	if airgapBundle != "" {
		// read file from path
		rawfile, err := os.Open(airgapBundle)
		if err != nil {
			return fmt.Errorf("failed to open airgap file: %w", err)
		}
		defer rawfile.Close()

		if err := airgap.MaterializeAirgap(rawfile); err != nil {
			err = fmt.Errorf("materialize airgap files: %w", err)
			return err
		}
	}

	return nil
}

func installAndStartCluster(
	ctx context.Context, installConfig models.InstallationConfig, airgapBundle string,
	mutate func(*k0sv1beta1.ClusterConfig) error,
) (*k0sv1beta1.ClusterConfig, error) {
	loading := spinner.Start()
	loading.Infof("Installing node")
	logrus.Debugf("creating k0s configuration file")

	cfg, err := k0s.WriteK0sConfig(ctx, installConfig, airgapBundle, mutate)
	if err != nil {
		loading.ErrorClosef("Failed to install node")
		return nil, fmt.Errorf("create config file: %w", err)
	}
	logrus.Debugf("creating systemd unit files")
	if err := createSystemdUnitFiles(ctx, false, runtimeconfig.ProxySpec()); err != nil {
		loading.ErrorClosef("Failed to install node")
		return nil, fmt.Errorf("create systemd unit files: %w", err)
	}

	logrus.Debugf("installing k0s")
	if err := k0s.Install(installConfig.NetworkInterface); err != nil {
		loading.ErrorClosef("Failed to install node")
		return nil, fmt.Errorf("install cluster: %w", err)
	}

	logrus.Debugf("waiting for k0s to be ready")
	if err := waitForK0s(); err != nil {
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

// configureNetworkManager configures the network manager (if the host is using it) to ignore
// the calico interfaces. This function restarts the NetworkManager service if the configuration
// was changed.
func configureNetworkManager(ctx context.Context) error {
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
	materializer := goods.NewMaterializer()
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
	return netutil.MaybeAddHTTPS(domains.ReplicatedAppDomain)
}

func proxyRegistryURL() string {
	var embCfgSpec *ecv1beta1.ConfigSpec
	if embCfg := release.GetEmbeddedClusterConfig(); embCfg != nil {
		embCfgSpec = &embCfg.Spec
	}
	domains := runtimeconfig.GetDomains(embCfgSpec)
	return netutil.MaybeAddHTTPS(domains.ProxyRegistryDomain)
}

// createSystemdUnitFiles links the k0s systemd unit file. this also creates a new
// systemd unit file for the local artifact mirror service.
func createSystemdUnitFiles(ctx context.Context, isWorker bool, proxy *ecv1beta1.ProxySpec) error {
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
	if err := installAndEnableLocalArtifactMirror(ctx); err != nil {
		return fmt.Errorf("unable to install and enable local artifact mirror: %w", err)
	}
	return nil
}

func systemdUnitFileName() string {
	return fmt.Sprintf("/etc/systemd/system/%s.service", runtimeconfig.BinaryName())
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
func installAndEnableLocalArtifactMirror(ctx context.Context) error {
	materializer := goods.NewMaterializer()
	if err := materializer.LocalArtifactMirrorUnitFile(); err != nil {
		return fmt.Errorf("failed to materialize artifact mirror unit: %w", err)
	}
	if err := writeLocalArtifactMirrorDropInFile(); err != nil {
		return fmt.Errorf("failed to write local artifact mirror environment file: %w", err)
	}
	if _, err := helpers.RunCommand("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("unable to get reload systemctl daemon: %w", err)
	}
	if _, err := helpers.RunCommand("systemctl", "enable", "local-artifact-mirror"); err != nil {
		return fmt.Errorf("unable to enable the local artifact mirror service: %w", err)
	}
	logrus.Debugf("Starting local artifact mirror")
	if _, err := helpers.RunCommand("systemctl", "start", "local-artifact-mirror"); err != nil {
		return fmt.Errorf("unable to start the local artifact mirror: %w", err)
	}
	if err := waitForLocalArtifactMirror(ctx); err != nil {
		return fmt.Errorf("unable to wait for the local artifact mirror: %w", err)
	}
	logrus.Debugf("Local artifact mirror started")
	return nil
}

func waitForLocalArtifactMirror(ctx context.Context) error {
	consecutiveSuccesses := 0
	requiredSuccesses := 3
	maxAttempts := 30
	checkInterval := 2 * time.Second

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		_, err := helpers.RunCommand("systemctl", "status", "local-artifact-mirror")
		if err == nil {
			consecutiveSuccesses++
			if consecutiveSuccesses >= requiredSuccesses {
				return nil
			}
		} else {
			consecutiveSuccesses = 0
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(checkInterval):
			continue
		}
	}

	return lastErr
}

const (
	localArtifactMirrorDropInFileContents = `[Service]
Environment="LOCAL_ARTIFACT_MIRROR_PORT=%d"
Environment="LOCAL_ARTIFACT_MIRROR_DATA_DIR=%s"
# Empty ExecStart= will clear out the previous ExecStart value
ExecStart=
ExecStart=%s serve
`
)

func writeLocalArtifactMirrorDropInFile() error {
	contents := fmt.Sprintf(
		localArtifactMirrorDropInFileContents,
		runtimeconfig.LocalArtifactMirrorPort(),
		runtimeconfig.EmbeddedClusterHomeDirectory(),
		runtimeconfig.PathToEmbeddedClusterBinary("local-artifact-mirror"),
	)
	err := systemd.WriteDropInFile("local-artifact-mirror.service", "embedded-cluster.conf", []byte(contents))
	if err != nil {
		return fmt.Errorf("write drop-in file: %w", err)
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
			spath := runtimeconfig.PathToK0sStatusSocket()
			if _, err := os.Stat(spath); err != nil {
				continue
			}
			success = true
			break
		}
		if !success {
			return fmt.Errorf("timeout waiting for %s", runtimeconfig.BinaryName())
		}
	}

	for i := 1; ; i++ {
		_, err := helpers.RunCommand(runtimeconfig.K0sBinaryPath(), "status")
		if err == nil {
			return nil
		} else if i == 30 {
			return fmt.Errorf("unable to get status: %w", err)
		}
		time.Sleep(2 * time.Second)
	}
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
	ctx context.Context, kcli client.Client,
	installConfig models.InstallationConfig, cliFlags installCmdFlags,
	k0sCfg *k0sv1beta1.ClusterConfig,
) (*ecv1beta1.Installation, error) {
	// ensure that the embedded-cluster namespace exists
	if err := createECNamespace(ctx, kcli); err != nil {
		return nil, fmt.Errorf("create embedded-cluster namespace: %w", err)
	}

	// ensure that the installation CRD exists
	if err := embeddedclusteroperator.EnsureInstallationCRD(ctx, kcli); err != nil {
		return nil, fmt.Errorf("create installation CRD: %w", err)
	}

	cfg := release.GetEmbeddedClusterConfig()
	var cfgspec *ecv1beta1.ConfigSpec
	if cfg != nil {
		cfgspec = &cfg.Spec
	}

	installation := &ecv1beta1.Installation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: ecv1beta1.GroupVersion.String(),
			Kind:       "Installation",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: time.Now().Format("20060102150405"),
		},
		Spec: ecv1beta1.InstallationSpec{
			ClusterID:      metrics.ClusterID().String(),
			MetricsBaseURL: replicatedAppURL(),
			AirGap:         cliFlags.airgapBundle != "",
			Config:         cfgspec,
			RuntimeConfig:  runtimeconfig.Get(),
			BinaryName:     runtimeconfig.BinaryName(),
			LicenseInfo: &ecv1beta1.LicenseInfo{
				IsDisasterRecoverySupported: cliFlags.license.Spec.IsDisasterRecoverySupported,
				IsMultiNodeEnabled:          cliFlags.license.Spec.IsEmbeddedClusterMultiNodeEnabled,
			},
		},
	}
	if err := kubeutils.CreateInstallation(ctx, kcli, installation); err != nil {
		return nil, fmt.Errorf("create installation: %w", err)
	}

	// the kubernetes api does not allow us to set the state of an object when creating it
	err := kubeutils.SetInstallationState(ctx, kcli, installation, ecv1beta1.InstallationStateKubernetesInstalled, "Kubernetes installed")
	if err != nil {
		return nil, fmt.Errorf("set installation state to KubernetesInstalled: %w", err)
	}

	return installation, nil
}

func createECNamespace(ctx context.Context, kcli client.Client) error {
	ns := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: runtimeconfig.EmbeddedClusterNamespace,
		},
	}
	if err := kcli.Create(ctx, &ns); err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func createVersionMetadataConfigmap(ctx context.Context, kcli client.Client) error {
	// This metadata should be the same as the artifact from the release without the vendor customizations
	metadata, err := gatherVersionMetadata(false)
	if err != nil {
		return fmt.Errorf("unable to gather release metadata: %w", err)
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("unable to marshal release metadata: %w", err)
	}

	// we trim out the prefix v from the version and then slugify it, we use
	// the result as a suffix for the config map name.
	slugver := slug.Make(strings.TrimPrefix(versions.Version, "v"))
	configmap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("version-metadata-%s", slugver),
			Namespace: "embedded-cluster",
			Labels: map[string]string{
				"replicated.com/disaster-recovery": "ec-install",
			},
		},
		Data: map[string]string{
			"metadata.json": string(data),
		},
	}

	if err := kcli.Create(ctx, configmap); err != nil {
		return fmt.Errorf("unable to create version metadata config map: %w", err)
	}
	return nil
}

// gatherVersionMetadata returns the release metadata for this version of
// embedded cluster. Release metadata involves the default versions of the
// components that are included in the release plus the default values used
// when deploying them.
func gatherVersionMetadata(withChannelRelease bool) (*types.ReleaseMetadata, error) {
	versionsMap := map[string]string{}
	for name, version := range addons.Versions() {
		versionsMap[name] = version
	}
	if withChannelRelease {
		for name, version := range extensions.Versions() {
			versionsMap[name] = version
		}
	}

	versionsMap["Kubernetes"] = versions.K0sVersion
	versionsMap["Installer"] = versions.Version
	versionsMap["Troubleshoot"] = versions.TroubleshootVersion

	if withChannelRelease {
		channelRelease := release.GetChannelRelease()
		if channelRelease != nil {
			versionsMap[runtimeconfig.BinaryName()] = channelRelease.VersionLabel
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

	chtconfig, repconfig, err := addons.GenerateChartConfigs()
	if err != nil {
		return nil, fmt.Errorf("unable to generate chart configs: %w", err)
	}

	additionalCharts := []ecv1beta1.Chart{}
	additionalRepos := []k0sv1beta1.Repository{}
	if withChannelRelease {
		additionalCharts = config.AdditionalCharts()
		additionalRepos = config.AdditionalRepositories()
	}

	meta.Configs = ecv1beta1.Helm{
		ConcurrencyLevel: 1,
		Charts:           append(chtconfig, additionalCharts...),
		Repositories:     append(repconfig, additionalRepos...),
	}

	k0sCfg := config.RenderK0sConfig(runtimeconfig.DefaultProxyRegistryDomain)
	meta.K0sImages = config.ListK0sImages(k0sCfg)
	meta.K0sImages = append(meta.K0sImages, addons.GetAdditionalImages()...)
	meta.K0sImages = helpers.UniqueStringSlice(meta.K0sImages)
	sort.Strings(meta.K0sImages)

	meta.Images = config.ListK0sImages(k0sCfg)
	meta.Images = append(meta.Images, addons.GetImages()...)
	meta.Images = append(meta.Images, versions.LocalArtifactMirrorImage)
	meta.Images = helpers.UniqueStringSlice(meta.Images)
	sort.Strings(meta.Images)

	return &meta, nil
}

func normalizeNoPromptToYes(f *pflag.FlagSet, name string) pflag.NormalizedName {
	switch name {
	case "no-prompt":
		name = "yes"
	}
	return pflag.NormalizedName(name)
}

func copyLicenseFileToDataDir(licenseFile, dataDir string) error {
	if licenseFile == "" {
		return nil
	}
	licenseData, err := os.ReadFile(licenseFile)
	if err != nil {
		return fmt.Errorf("unable to read license file: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "license.yaml"), licenseData, 0400); err != nil {
		return fmt.Errorf("unable to write license file: %w", err)
	}
	return nil
}

func printSuccessMessage(license *kotsv1beta1.License, networkInterface string) error {
	adminConsoleURL := getAdminConsoleURL(networkInterface, runtimeconfig.AdminConsolePort())

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

	return nil
}

func getAdminConsoleURL(networkInterface string, port int) string {
	ipaddr := runtimeconfig.TryDiscoverPublicIP()
	if ipaddr == "" {
		var err error
		ipaddr, err = netutils.FirstValidAddress(networkInterface)
		if err != nil {
			logrus.Errorf("Unable to determine node IP address: %v", err)
			ipaddr = "NODE-IP-ADDRESS"
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
