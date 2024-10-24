package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	k8syaml "sigs.k8s.io/yaml"

	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/configutils"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/goods"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/embedded-cluster/pkg/support"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

// Minimum character length for the Admin Console password
const minAdminPasswordLength = 6

// ErrNothingElseToAdd is an error returned when there is nothing else to add to the
// screen. This is useful when we want to exit an error from a function here but
// don't want to print anything else (possibly because we have already printed the
// necessary data to the screen).
var ErrNothingElseToAdd = fmt.Errorf("")

// ErrPreflightsHaveFail is an error returned when we managed to execute the
// host preflights but they contain failures. We use this to differentiate the
// way we provide user feedback.
var ErrPreflightsHaveFail = fmt.Errorf("host preflight failures detected")

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

// updateLocalArtifactMirrorService updates the port on which the local artifact mirror is served.
func updateLocalArtifactMirrorService(provider *defaults.Provider) error {
	if err := writeLocalArtifactMirrorDropInFile(provider); err != nil {
		return fmt.Errorf("failed to write local artifact mirror environment file: %w", err)
	}
	if _, err := helpers.RunCommand("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("unable to get reload systemctl daemon: %w", err)
	}
	if _, err := helpers.RunCommand("systemctl", "restart", "local-artifact-mirror"); err != nil {
		return fmt.Errorf("unable to restart the local artifact mirror service: %w", err)
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

// configureNetworkManager configures the network manager (if the host is using it) to ignore
// the calico interfaces. This function restarts the NetworkManager service if the configuration
// was changed.
func configureNetworkManager(c *cli.Context, provider *defaults.Provider) error {
	if active, err := helpers.IsSystemdServiceActive(c.Context, "NetworkManager"); err != nil {
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

// RunHostPreflights runs the host preflights we found embedded in the binary
// on all configured hosts. We attempt to read HostPreflights from all the
// embedded Helm Charts and from the Kots Application Release files.
func RunHostPreflights(c *cli.Context, provider *defaults.Provider, applier *addons.Applier, replicatedAPIURL, proxyRegistryURL string, isAirgap bool, proxy *ecv1beta1.ProxySpec) error {
	hpf, err := applier.HostPreflights()
	if err != nil {
		return fmt.Errorf("unable to read host preflights: %w", err)
	}

	data, err := preflights.TemplateData{
		ReplicatedAPIURL:        replicatedAPIURL,
		ProxyRegistryURL:        proxyRegistryURL,
		IsAirgap:                isAirgap,
		AdminConsolePort:        provider.AdminConsolePort(),
		LocalArtifactMirrorPort: provider.LocalArtifactMirrorPort(),
		DataDir:                 provider.EmbeddedClusterHomeDirectory(),
		K0sDataDir:              provider.EmbeddedClusterK0sSubDir(),
		OpenEBSDataDir:          provider.EmbeddedClusterOpenEBSLocalSubDir(),
		SystemArchitecture:      runtime.GOARCH,
	}.WithCIDRData(getCIDRs(c))

	if err != nil {
		return fmt.Errorf("unable to get host preflights data: %w", err)
	}
	chpfs, err := preflights.GetClusterHostPreflights(c.Context, data)
	if err != nil {
		return fmt.Errorf("unable to get cluster host preflights: %w", err)
	}

	for _, h := range chpfs {
		hpf.Collectors = append(hpf.Collectors, h.Spec.Collectors...)
		hpf.Analyzers = append(hpf.Analyzers, h.Spec.Analyzers...)
	}

	return runHostPreflights(c, provider, hpf, proxy)
}

func runHostPreflights(c *cli.Context, provider *defaults.Provider, hpf *v1beta2.HostPreflightSpec, proxy *ecv1beta1.ProxySpec) error {
	if len(hpf.Collectors) == 0 && len(hpf.Analyzers) == 0 {
		return nil
	}
	pb := spinner.Start()
	if c.Bool("skip-host-preflights") {
		pb.Infof("Host preflights skipped")
		pb.Close()
		return nil
	}
	pb.Infof("Running host preflights")
	output, stderr, err := preflights.Run(c.Context, provider, hpf, proxy)
	if err != nil {
		pb.CloseWithError()
		return fmt.Errorf("host preflights failed to run: %w", err)
	}
	if stderr != "" {
		logrus.Debugf("preflight stderr: %s", stderr)
	}

	err = output.SaveToDisk(provider.PathToEmbeddedClusterSupportFile("host-preflight-results.json"))
	if err != nil {
		logrus.Warnf("unable to save preflights output: %v", err)
	}

	err = preflights.CopyBundleToECSupportDir(provider)
	if err != nil {
		logrus.Warnf("unable to copy preflight bundle to embedded-cluster support dir: %v", err)
	}

	// Failures found
	if output.HasFail() {
		s := "preflights"
		if len(output.Fail) == 1 {
			s = "preflight"
		}
		if output.HasWarn() {
			pb.Errorf("%d host %s failed and %d warned", len(output.Fail), s, len(output.Warn))
		} else {
			pb.Errorf("%d host %s failed", len(output.Fail), s)
		}

		pb.CloseWithError()
		output.PrintTableWithoutInfo()
		return ErrPreflightsHaveFail
	}

	// Warnings found
	if output.HasWarn() {
		s := "preflights"
		if len(output.Warn) == 1 {
			s = "preflight"
		}
		pb.Warnf("%d host %s warned", len(output.Warn), s)
		if c.Bool("no-prompt") {
			// We have warnings but we are not in interactive mode
			// so we just print the warnings and continue
			pb.Close()
			output.PrintTableWithoutInfo()
			return nil
		}
		pb.CloseWithError()
		output.PrintTableWithoutInfo()
		if !prompts.New().Confirm("Do you want to continue ?", false) {
			return fmt.Errorf("user aborted")
		}
	}

	// No failures or warnings
	pb.Infof("Host preflights succeeded!")
	pb.Close()
	return nil
}

// isAlreadyInstalled checks if the embedded cluster is already installed by looking for
// the k0s configuration file existence.
func isAlreadyInstalled() (bool, error) {
	cfgpath := defaults.PathToK0sConfig()
	_, err := os.Stat(cfgpath)
	switch {
	case err == nil:
		return true, nil
	case os.IsNotExist(err):
		return false, nil
	default:
		return false, fmt.Errorf("unable to check if already installed: %w", err)
	}
}

func getLicenseFromFilepath(licenseFile string) (*kotsv1beta1.License, error) {
	rel, err := release.GetChannelRelease()
	if err != nil {
		return nil, fmt.Errorf("failed to get release from binary: %w", err) // this should only be if the release is malformed
	}

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
			return nil, fmt.Errorf("unable to parse expiration date: %w", err)
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

func checkAirgapMatches(c *cli.Context) error {
	rel, err := release.GetChannelRelease()
	if err != nil {
		return fmt.Errorf("failed to get release from binary: %w", err) // this should only be if the release is malformed
	}
	if rel == nil {
		return fmt.Errorf("airgap bundle provided but no release was found in binary, please rerun without the airgap-bundle flag")
	}

	// read file from path
	rawfile, err := os.Open(c.String("airgap-bundle"))
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

func materializeFiles(c *cli.Context, provider *defaults.Provider) error {
	mat := spinner.Start()
	defer mat.Close()
	mat.Infof("Materializing files")

	materializer := goods.NewMaterializer(provider)
	if err := materializer.Materialize(); err != nil {
		return fmt.Errorf("unable to materialize binaries: %w", err)
	}
	if err := support.MaterializeSupportBundleSpec(provider); err != nil {
		return fmt.Errorf("unable to materialize support bundle spec: %w", err)
	}
	if c.String("airgap-bundle") != "" {
		mat.Infof("Materializing airgap installation files")

		// read file from path
		rawfile, err := os.Open(c.String("airgap-bundle"))
		if err != nil {
			return fmt.Errorf("failed to open airgap file: %w", err)
		}
		defer rawfile.Close()

		if err := airgap.MaterializeAirgap(provider, rawfile); err != nil {
			err = fmt.Errorf("unable to materialize airgap files: %w", err)
			return err
		}
	}

	mat.Infof("Host files materialized!")

	return nil
}

// createK0sConfig creates a new k0s.yaml configuration file. The file is saved in the
// global location (as returned by defaults.PathToK0sConfig()). If a file already sits
// there, this function returns an error.
func ensureK0sConfig(c *cli.Context, provider *defaults.Provider, applier *addons.Applier) (*k0sconfig.ClusterConfig, error) {
	cfgpath := defaults.PathToK0sConfig()
	if _, err := os.Stat(cfgpath); err == nil {
		return nil, fmt.Errorf("configuration file already exists")
	}
	if err := os.MkdirAll(filepath.Dir(cfgpath), 0755); err != nil {
		return nil, fmt.Errorf("unable to create directory: %w", err)
	}
	cfg := config.RenderK0sConfig()
	address, err := netutils.FirstValidAddress(c.String("network-interface"))
	if err != nil {
		return nil, fmt.Errorf("unable to find first valid address: %w", err)
	}
	cfg.Spec.API.Address = address
	cfg.Spec.Storage.Etcd.PeerAddress = address

	podCIDR, serviceCIDR, err := DeterminePodAndServiceCIDRs(c)
	if err != nil {
		return nil, fmt.Errorf("unable to determine pod and service CIDRs: %w", err)
	}
	cfg.Spec.Network.PodCIDR = podCIDR
	cfg.Spec.Network.ServiceCIDR = serviceCIDR
	if err := config.UpdateHelmConfigs(applier, cfg); err != nil {
		return nil, fmt.Errorf("unable to update helm configs: %w", err)
	}
	cfg, err = applyUnsupportedOverrides(c, cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to apply unsupported overrides: %w", err)
	}
	if c.String("airgap-bundle") != "" {
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
	fp, err := os.OpenFile(cfgpath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, fmt.Errorf("unable to create config file: %w", err)
	}
	defer fp.Close()
	if _, err := fp.Write(data); err != nil {
		return nil, fmt.Errorf("unable to write config file: %w", err)
	}

	return cfg, nil
}

// applyUnsupportedOverrides applies overrides to the k0s configuration. Applies first the
// overrides embedded into the binary and after the ones provided by the user (--overrides).
// we first apply the k0s config override and then apply the built in overrides.
func applyUnsupportedOverrides(c *cli.Context, cfg *k0sconfig.ClusterConfig) (*k0sconfig.ClusterConfig, error) {
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

	eucfg, err := helpers.ParseEndUserConfig(c.String("overrides"))
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

// installK0s runs the k0s install command and waits for it to finish. If no configuration
// is found one is generated.
func installK0s(c *cli.Context, provider *defaults.Provider) error {
	ourbin := provider.PathToEmbeddedClusterBinary("k0s")
	hstbin := defaults.K0sBinaryPath()
	if err := helpers.MoveFile(ourbin, hstbin); err != nil {
		return fmt.Errorf("unable to move k0s binary: %w", err)
	}
	nodeIP, err := netutils.FirstValidAddress(c.String("network-interface"))
	if err != nil {
		return fmt.Errorf("unable to find first valid address: %w", err)
	}
	if _, err := helpers.RunCommand(hstbin, config.InstallFlags(provider, nodeIP)...); err != nil {
		return fmt.Errorf("unable to install: %w", err)
	}
	if _, err := helpers.RunCommand(hstbin, "start"); err != nil {
		return fmt.Errorf("unable to start: %w", err)
	}
	return nil
}

// waitForK0s waits for the k0s API to be available. We wait for the k0s socket to
// appear in the system and until the k0s status command to finish.
func waitForK0s() error {
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

// installAndWaitForK0s installs the k0s binary and waits for it to be ready
func installAndWaitForK0s(c *cli.Context, provider *defaults.Provider, applier *addons.Applier, proxy *ecv1beta1.ProxySpec) (*k0sconfig.ClusterConfig, error) {
	loading := spinner.Start()
	defer loading.Close()
	loading.Infof("Installing %s node", defaults.BinaryName())
	logrus.Debugf("creating k0s configuration file")
	cfg, err := ensureK0sConfig(c, provider, applier)
	if err != nil {
		err := fmt.Errorf("unable to create config file: %w", err)
		metrics.ReportApplyFinished(c, err)
		return nil, err
	}
	logrus.Debugf("creating systemd unit files")
	if err := createSystemdUnitFiles(provider, false, proxy); err != nil {
		err := fmt.Errorf("unable to create systemd unit files: %w", err)
		metrics.ReportApplyFinished(c, err)
		return nil, err
	}

	logrus.Debugf("installing k0s")
	if err := installK0s(c, provider); err != nil {
		err := fmt.Errorf("unable to install cluster: %w", err)
		metrics.ReportApplyFinished(c, err)
		return nil, err
	}
	loading.Infof("Waiting for %s node to be ready", defaults.BinaryName())
	logrus.Debugf("waiting for k0s to be ready")
	if err := waitForK0s(); err != nil {
		err := fmt.Errorf("unable to wait for node: %w", err)
		metrics.ReportApplyFinished(c, err)
		return nil, err
	}

	loading.Infof("Node installation finished!")
	return cfg, nil
}

// runOutro calls Outro() in all enabled addons by means of Applier.
func runOutro(c *cli.Context, provider *defaults.Provider, applier *addons.Applier, cfg *k0sconfig.ClusterConfig) error {
	os.Setenv("KUBECONFIG", provider.PathToKubeConfig())

	// This metadata should be the same as the artifact from the release without the vendor customizations
	defaultCfg := config.RenderK0sConfig()
	metadata, err := gatherVersionMetadata(defaultCfg, false)
	if err != nil {
		return fmt.Errorf("unable to gather release metadata: %w", err)
	}

	eucfg, err := helpers.ParseEndUserConfig(c.String("overrides"))
	if err != nil {
		return fmt.Errorf("unable to process overrides file: %w", err)
	}

	return applier.Outro(c.Context, cfg, eucfg, metadata, c.String("network-interface"))
}

func maybeAskAdminConsolePassword(c *cli.Context) (string, error) {
	defaultPassword := "password"
	userProvidedPassword := c.String("admin-console-password")
	// If there's a user provided password we'll try that first
	if userProvidedPassword != "" {
		// Password isn't retyped so we provided it twice
		if !validateAdminConsolePassword(userProvidedPassword, userProvidedPassword) {
			return "", fmt.Errorf("unable to set the Admin Console password")
		}
		return userProvidedPassword, nil
	}
	// No user provided password but prompt is disabled so we set our default password
	if c.Bool("no-prompt") {
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

// installCommands executes the "install" command. This will ensure that a k0s.yaml file exists
// and then run `k0s install` to apply the cluster. Once this is finished then a "kubeconfig"
// file is created. Resulting kubeconfig is stored in the configuration dir.
func installCommand() *cli.Command {
	runtimeConfig := ecv1beta1.GetDefaultRuntimeConfig()

	return &cli.Command{
		Name:  "install",
		Usage: fmt.Sprintf("Install %s", binName),
		Subcommands: []*cli.Command{
			installRunPreflightsCommand(),
		},
		Before: func(c *cli.Context) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("install command must be run as root")
			}
			if c.String("airgap-bundle") != "" {
				metrics.DisableMetrics()
			}
			return nil
		},
		Flags: withProxyFlags(withSubnetCIDRFlags(
			[]cli.Flag{
				&cli.StringFlag{
					Name:   "admin-console-password",
					Usage:  fmt.Sprintf("Password for the Admin Console (minimum %d characters)", minAdminPasswordLength),
					Hidden: false,
				},
				getAdminConsolePortFlag(runtimeConfig),
				&cli.StringFlag{
					Name:  "airgap-bundle",
					Usage: "Path to the air gap bundle. If set, the installation will complete without internet access.",
				},
				getDataDirFlagWithDefault(runtimeConfig),
				&cli.StringFlag{
					Name:    "license",
					Aliases: []string{"l"},
					Usage:   "Path to the license file",
					Hidden:  false,
				},
				getLocalArtifactMirrorPortFlag(runtimeConfig),
				&cli.StringFlag{
					Name:  "network-interface",
					Usage: "The network interface to use for the cluster",
					Value: "",
				},
				&cli.BoolFlag{
					Name:  "no-prompt",
					Usage: "Disable interactive prompts. The Admin Console password will be set to password.",
					Value: false,
				},
				&cli.StringFlag{
					Name:   "overrides",
					Usage:  "File with an EmbeddedClusterConfig object to override the default configuration",
					Hidden: true,
				},
				&cli.StringSliceFlag{
					Name:  "private-ca",
					Usage: "Path to a trusted private CA certificate file",
				},
				&cli.BoolFlag{
					Name:  "skip-host-preflights",
					Usage: "Skip host preflight checks. This is not recommended.",
					Value: false,
				},
			},
		)),
		Action: func(c *cli.Context) error {
			provider := defaults.NewProviderFromRuntimeConfig(runtimeConfig)
			os.Setenv("TMPDIR", provider.EmbeddedClusterTmpSubDir())

			defer tryRemoveTmpDirContents(provider)

			var err error
			err = configutils.WriteRuntimeConfig(runtimeConfig)
			if err != nil {
				return fmt.Errorf("unable to write runtime config: %w", err)
			}

			proxy, err := getProxySpecFromFlags(c)
			if err != nil {
				return fmt.Errorf("unable to get proxy spec from flags: %w", err)
			}

			proxy, err = includeLocalIPInNoProxy(c, proxy)
			if err != nil {
				metrics.ReportApplyFinished(c, err)
				return err
			}
			setProxyEnv(proxy)

			logrus.Debugf("checking if %s is already installed", binName)
			if installed, err := isAlreadyInstalled(); err != nil {
				return err
			} else if installed {
				logrus.Errorf("An installation has been detected on this machine.")
				logrus.Infof("If you want to reinstall, you need to remove the existing installation first.")
				logrus.Infof("You can do this by running the following command:")
				logrus.Infof("\n  sudo ./%s reset\n", binName)
				return ErrNothingElseToAdd
			}
			metrics.ReportApplyStarted(c)
			logrus.Debugf("configuring network manager")
			if err := configureNetworkManager(c, provider); err != nil {
				return fmt.Errorf("unable to configure network manager: %w", err)
			}
			logrus.Debugf("checking license matches")
			license, err := getLicenseFromFilepath(c.String("license"))
			if err != nil {
				metricErr := fmt.Errorf("unable to get license: %w", err)
				metrics.ReportApplyFinished(c, metricErr)
				return err // do not return the metricErr, as we want the user to see the error message without a prefix
			}
			isAirgap := c.String("airgap-bundle") != ""
			if isAirgap {
				logrus.Debugf("checking airgap bundle matches binary")
				if err := checkAirgapMatches(c); err != nil {
					return err // we want the user to see the error message without a prefix
				}
			}
			if err := preflights.ValidateApp(); err != nil {
				metrics.ReportApplyFinished(c, err)
				return err
			}
			adminConsolePwd, err := maybeAskAdminConsolePassword(c)
			if err != nil {
				metrics.ReportApplyFinished(c, err)
				return err
			}

			logrus.Debugf("materializing binaries")
			if err := materializeFiles(c, provider); err != nil {
				metrics.ReportApplyFinished(c, err)
				return err
			}
			applier, err := getAddonsApplier(c, runtimeConfig, adminConsolePwd, proxy)
			if err != nil {
				metrics.ReportApplyFinished(c, err)
				return err
			}
			logrus.Debugf("running host preflights")
			var replicatedAPIURL, proxyRegistryURL string
			if license != nil {
				replicatedAPIURL = license.Spec.Endpoint
				proxyRegistryURL = fmt.Sprintf("https://%s", defaults.ProxyRegistryAddress)
			}

			if err := RunHostPreflights(c, provider, applier, replicatedAPIURL, proxyRegistryURL, isAirgap, proxy); err != nil {
				metrics.ReportApplyFinished(c, err)
				if err == ErrPreflightsHaveFail {
					return ErrNothingElseToAdd
				}
				return err
			}

			cfg, err := installAndWaitForK0s(c, provider, applier, proxy)
			if err != nil {
				return err
			}
			logrus.Debugf("running outro")
			if err := runOutro(c, provider, applier, cfg); err != nil {
				metrics.ReportApplyFinished(c, err)
				return err
			}
			metrics.ReportApplyFinished(c, nil)
			return nil
		},
	}
}

func getAddonsApplier(c *cli.Context, runtimeConfig *ecv1beta1.RuntimeConfigSpec, adminConsolePwd string, proxy *ecv1beta1.ProxySpec) (*addons.Applier, error) {
	opts := []addons.Option{}
	opts = append(opts, addons.WithRuntimeConfig(runtimeConfig))

	if c.Bool("no-prompt") {
		opts = append(opts, addons.WithoutPrompt())
	}
	if l := c.String("license"); l != "" {
		license, err := helpers.ParseLicense(l)
		if err != nil {
			return nil, fmt.Errorf("unable to parse license: %w", err)
		}

		opts = append(opts, addons.WithLicense(license))
		opts = append(opts, addons.WithLicenseFile(l))
	}
	if ab := c.String("airgap-bundle"); ab != "" {
		opts = append(opts, addons.WithAirgapBundle(ab))
	}
	if proxy != nil {
		opts = append(opts, addons.WithProxy(proxy))
	}
	if c.String("overrides") != "" {
		eucfg, err := helpers.ParseEndUserConfig(c.String("overrides"))
		if err != nil {
			return nil, fmt.Errorf("unable to process overrides file: %w", err)
		}
		opts = append(opts, addons.WithEndUserConfig(eucfg))
	}
	if len(c.StringSlice("private-ca")) > 0 {
		privateCAs := map[string]string{}
		for i, path := range c.StringSlice("private-ca") {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("unable to read private CA file %s: %w", path, err)
			}
			name := fmt.Sprintf("ca_%d.crt", i)
			privateCAs[name] = string(data)
		}
		opts = append(opts, addons.WithPrivateCAs(privateCAs))
	}

	if adminConsolePwd != "" {
		opts = append(opts, addons.WithAdminConsolePassword(adminConsolePwd))
	}
	return addons.NewApplier(opts...), nil
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
