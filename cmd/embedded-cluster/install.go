package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	k8syaml "sigs.k8s.io/yaml"

	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/goods"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

// ErrNothingElseToAdd is an error returned when there is nothing else to add to the
// screen. This is useful when we want to exit an error from a function here but
// don't want to print anything else (possibly because we have already printed the
// necessary data to the screen).
var ErrNothingElseToAdd = fmt.Errorf("")

// installAndEnableLocalArtifactMirror installs and enables the local artifact mirror. This
// service is responsible for serving on localhost, through http, all files that are used
// during a cluster upgrade.
func installAndEnableLocalArtifactMirror() error {
	if err := goods.MaterializeLocalArtifactMirrorUnitFile(); err != nil {
		return fmt.Errorf("failed to materialize artifact mirror unit: %w", err)
	}
	if _, err := helpers.RunCommand("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("unable to get reload systemctl daemon: %w", err)
	}
	if _, err := helpers.RunCommand("systemctl", "start", "local-artifact-mirror"); err != nil {
		return fmt.Errorf("unable to start the local artifact mirror: %w", err)
	}
	if _, err := helpers.RunCommand("systemctl", "enable", "local-artifact-mirror"); err != nil {
		return fmt.Errorf("unable to start the local artifact mirror: %w", err)
	}
	return nil
}

// configureNetworkManager configures the network manager (if the host is using it) to ignore
// the calico interfaces. This function restarts the NetworkManager service if the configuration
// was changed.
func configureNetworkManager(c *cli.Context) error {
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
	if err := goods.MaterializeCalicoNetworkManagerConfig(); err != nil {
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
func RunHostPreflights(c *cli.Context) error {
	hpf, err := addons.NewApplier().HostPreflights()
	if err != nil {
		return fmt.Errorf("unable to read host preflights: %w", err)
	}
	return runHostPreflights(c, hpf)
}

func runHostPreflights(c *cli.Context, hpf *v1beta2.HostPreflightSpec) error {
	if len(hpf.Collectors) == 0 && len(hpf.Analyzers) == 0 {
		return nil
	}
	pb := spinner.Start()
	pb.Infof("Running host preflights on node")
	output, err := preflights.Run(c.Context, hpf)
	if err != nil {
		pb.CloseWithError()
		return fmt.Errorf("host preflights failed: %w", err)
	}
	if output.HasFail() {
		pb.CloseWithError()
		output.PrintTable()
		return fmt.Errorf("preflights haven't passed on the host")
	}
	if !output.HasWarn() || c.Bool("no-prompt") {
		pb.Close()
		output.PrintTable()
		return nil
	}
	pb.CloseWithError()
	output.PrintTable()
	logrus.Infof("Host preflights have warnings")
	if !prompts.New().Confirm("Do you want to continue ?", false) {
		return fmt.Errorf("user aborted")
	}
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

func checkLicenseMatches(licenseFile string) error {
	rel, err := release.GetChannelRelease()
	if err != nil {
		return fmt.Errorf("failed to get release from binary: %w", err) // this should only be if the release is malformed
	}

	// handle the three cases that do not require parsing the license file
	// 1. no release and no license, which is OK
	// 2. no license and a release, which is not OK
	// 3. a license and no release, which is not OK
	if rel == nil && licenseFile == "" {
		// no license and no release, this is OK
		return nil
	} else if rel == nil && licenseFile != "" {
		// license is present but no release, this means we would install without vendor charts and k0s overrides
		return fmt.Errorf("a license was provided but no release was found in binary, please rerun without the license flag")
	} else if rel != nil && licenseFile == "" {
		// release is present but no license, this is not OK
		return fmt.Errorf("no license was provided for %s and one is required, please rerun with '--license <path to license file>'", rel.AppSlug)
	}

	license, err := helpers.ParseLicense(licenseFile)
	if err != nil {
		return fmt.Errorf("unable to parse the license file at %q, please ensure it is not corrupt: %w", licenseFile, err)
	}

	// Check if the license matches the application version data
	if rel.AppSlug != license.Spec.AppSlug {
		// if the app is different, we will not be able to provide the correct vendor supplied charts and k0s overrides
		return fmt.Errorf("license app %s does not match binary app %s, please provide the correct license", license.Spec.AppSlug, rel.AppSlug)
	}
	if rel.ChannelID != license.Spec.ChannelID {
		// if the channel is different, we will not be able to install the pinned vendor application version within kots
		// this may result in an immediate k8s upgrade after installation, which is undesired
		return fmt.Errorf("license channel %s (%s) does not match binary channel %s, please provide the correct license", license.Spec.ChannelID, license.Spec.ChannelName, rel.ChannelID)
	}

	if license.Spec.Entitlements["expires_at"].Value.StrVal != "" {
		// read the expiration date, and check it against the current date
		expiration, err := time.Parse(time.RFC3339, license.Spec.Entitlements["expires_at"].Value.StrVal)
		if err != nil {
			return fmt.Errorf("unable to parse expiration date: %w", err)
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

func materializeFiles(c *cli.Context) error {
	mat := spinner.Start()
	defer mat.Close()
	mat.Infof("Materializing files")

	if err := goods.Materialize(); err != nil {
		return fmt.Errorf("unable to materialize binaries: %w", err)
	}
	if c.String("airgap-bundle") != "" {
		mat.Infof("Materializing airgap installation files")

		// read file from path
		rawfile, err := os.Open(c.String("airgap-bundle"))
		if err != nil {
			return fmt.Errorf("failed to open airgap file: %w", err)
		}
		defer rawfile.Close()

		if err := airgap.MaterializeAirgap(rawfile); err != nil {
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
func ensureK0sConfig(c *cli.Context) (*k0sconfig.ClusterConfig, error) {
	cfgpath := defaults.PathToK0sConfig()
	if _, err := os.Stat(cfgpath); err == nil {
		return nil, fmt.Errorf("configuration file already exists")
	}
	if err := os.MkdirAll(filepath.Dir(cfgpath), 0755); err != nil {
		return nil, fmt.Errorf("unable to create directory: %w", err)
	}
	cfg := config.RenderK0sConfig()
	if c.String("pod-cidr") != "" {
		cfg.Spec.Network.PodCIDR = c.String("pod-cidr")
	}
	if c.String("service-cidr") != "" {
		cfg.Spec.Network.ServiceCIDR = c.String("service-cidr")
	}
	opts := []addons.Option{}
	if c.Bool("no-prompt") {
		opts = append(opts, addons.WithoutPrompt())
	}
	if l := c.String("license"); l != "" {
		opts = append(opts, addons.WithLicense(l))
	}
	if ab := c.String("airgap-bundle"); ab != "" {
		opts = append(opts, addons.WithAirgapBundle(ab))
	}
	if c.Bool("proxy") {
		opts = append(opts, addons.WithProxyFromEnv())
	}
	if c.String("http-proxy") != "" || c.String("https-proxy") != "" || c.String("no-proxy") != "" {
		opts = append(opts, addons.WithProxyFromArgs(c.String("http-proxy"), c.String("https-proxy"), c.String("no-proxy"), cfg.Spec.Network.PodCIDR, cfg.Spec.Network.ServiceCIDR))
	}
	if err := config.UpdateHelmConfigs(cfg, opts...); err != nil {
		return nil, fmt.Errorf("unable to update helm configs: %w", err)
	}
	var err error
	if cfg, err = applyUnsupportedOverrides(c, cfg); err != nil {
		return nil, fmt.Errorf("unable to apply unsupported overrides: %w", err)
	}
	if c.String("airgap-bundle") != "" {
		// update the k0s config to install with airgap
		airgap.RemapHelm(cfg)
		airgap.SetAirgapConfig(cfg)
	}
	data, err := k8syaml.Marshal(cfg)
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
	var err error
	if embcfg, err := release.GetEmbeddedClusterConfig(); err != nil {
		return nil, fmt.Errorf("unable to get embedded cluster config: %w", err)
	} else if embcfg != nil {
		overrides := embcfg.Spec.UnsupportedOverrides.K0s
		if cfg, err = config.PatchK0sConfig(cfg, overrides); err != nil {
			return nil, fmt.Errorf("unable to patch k0s config: %w", err)
		}
		if cfg, err = config.ApplyBuiltInExtensionsOverrides(cfg, embcfg); err != nil {
			return nil, fmt.Errorf("unable to release built in overrides: %w", err)
		}
	}
	if c.String("overrides") == "" {
		return cfg, nil
	}
	eucfg, err := helpers.ParseEndUserConfig(c.String("overrides"))
	if err != nil {
		return nil, fmt.Errorf("unable to process overrides file: %w", err)
	}
	overrides := eucfg.Spec.UnsupportedOverrides.K0s
	if cfg, err = config.PatchK0sConfig(cfg, overrides); err != nil {
		return nil, fmt.Errorf("unable to apply overrides: %w", err)
	}
	if cfg, err = config.ApplyBuiltInExtensionsOverrides(cfg, eucfg); err != nil {
		return nil, fmt.Errorf("unable to end user built in overrides: %w", err)
	}
	return cfg, nil
}

// installK0s runs the k0s install command and waits for it to finish. If no configuration
// is found one is generated.
func installK0s() error {
	ourbin := defaults.PathToEmbeddedClusterBinary("k0s")
	hstbin := defaults.K0sBinaryPath()
	if err := helpers.MoveFile(ourbin, hstbin); err != nil {
		return fmt.Errorf("unable to move k0s binary: %w", err)
	}
	if _, err := helpers.RunCommand(hstbin, config.InstallFlags()...); err != nil {
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
	loading := spinner.Start()
	defer loading.Close()
	loading.Infof("Waiting for %s node to be ready", defaults.BinaryName())
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
	if _, err := helpers.RunCommand(defaults.K0sBinaryPath(), "status"); err != nil {
		return fmt.Errorf("unable to get status: %w", err)
	}
	loading.Infof("Node installation finished!")
	return nil
}

// runOutro calls Outro() in all enabled addons by means of Applier.
func runOutro(c *cli.Context, cfg *k0sconfig.ClusterConfig) error {
	os.Setenv("KUBECONFIG", defaults.PathToKubeConfig())
	opts := []addons.Option{}

	metadata, err := gatherVersionMetadata()
	if err != nil {
		return fmt.Errorf("unable to gather release metadata: %w", err)
	}
	opts = append(opts, addons.WithVersionMetadata(metadata))

	if l := c.String("license"); l != "" {
		opts = append(opts, addons.WithLicense(l))
	}
	if c.String("overrides") != "" {
		eucfg, err := helpers.ParseEndUserConfig(c.String("overrides"))
		if err != nil {
			return fmt.Errorf("unable to load overrides: %w", err)
		}
		opts = append(opts, addons.WithEndUserConfig(eucfg))
	}
	if ab := c.String("airgap-bundle"); ab != "" {
		opts = append(opts, addons.WithAirgapBundle(ab))
	}
	if c.String("http-proxy") != "" || c.String("https-proxy") != "" || c.String("no-proxy") != "" {
		opts = append(opts, addons.WithProxyFromArgs(c.String("http-proxy"), c.String("https-proxy"), c.String("no-proxy"), cfg.Spec.Network.PodCIDR, cfg.Spec.Network.ServiceCIDR))
	}
	return addons.NewApplier(opts...).Outro(c.Context)
}

// installCommands executes the "install" command. This will ensure that a k0s.yaml file exists
// and then run `k0s install` to apply the cluster. Once this is finished then a "kubeconfig"
// file is created. Resulting kubeconfig is stored in the configuration dir.
var installCommand = &cli.Command{
	Name:  "install",
	Usage: fmt.Sprintf("Install %s", binName),
	Before: func(c *cli.Context) error {
		if os.Getuid() != 0 {
			return fmt.Errorf("install command must be run as root")
		}
		if c.String("airgap-bundle") != "" {
			metrics.DisableMetrics()
		}
		return nil
	},
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "no-prompt",
			Usage: "Disable interactive prompts. Admin console password will be set to password.",
			Value: false,
		},
		&cli.StringFlag{
			Name:   "overrides",
			Usage:  "File with an EmbeddedClusterConfig object to override the default configuration",
			Hidden: true,
		},
		&cli.StringFlag{
			Name:    "license",
			Aliases: []string{"l"},
			Usage:   "Path to the application license file",
			Hidden:  false,
		},
		&cli.StringFlag{
			Name:   "airgap-bundle",
			Usage:  "Path to the airgap bundle. If set, the installation will be completed without internet access.",
			Hidden: true,
		},
		&cli.StringFlag{
			Name:   "http-proxy",
			Usage:  "HTTP proxy to use for the installation",
			Hidden: false,
		},
		&cli.StringFlag{
			Name:   "https-proxy",
			Usage:  "HTTPS proxy to use for the installation",
			Hidden: false,
		},
		&cli.StringFlag{
			Name:   "no-proxy",
			Usage:  "Comma separated list of hosts to bypass the proxy for",
			Hidden: false,
		},
		&cli.BoolFlag{
			Name:   "proxy",
			Usage:  "Use the system proxy settings for the install operation. These variables are currently only passed through to Velero and the Admin Console.",
			Hidden: true,
		},
		&cli.StringFlag{
			Name:   "pod-cidr",
			Usage:  "pod CIDR range to use for the installation",
			Hidden: false,
		},
		&cli.StringFlag{
			Name:   "service-cidr",
			Usage:  "service CIDR range to use for the installation",
			Hidden: false,
		},
	},
	Action: func(c *cli.Context) error {
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
		if err := configureNetworkManager(c); err != nil {
			return fmt.Errorf("unable to configure network manager: %w", err)
		}
		logrus.Debugf("checking license matches")
		if err := checkLicenseMatches(c.String("license")); err != nil {
			metricErr := fmt.Errorf("unable to check license: %w", err)
			metrics.ReportApplyFinished(c, metricErr)
			return err // do not return the metricErr, as we want the user to see the error message without a prefix
		}
		if c.String("airgap-bundle") != "" {
			logrus.Debugf("checking airgap bundle matches binary")
			if err := checkAirgapMatches(c); err != nil {
				return err // we want the user to see the error message without a prefix
			}
		}
		logrus.Debugf("materializing binaries")
		if err := materializeFiles(c); err != nil {
			metrics.ReportApplyFinished(c, err)
			return err
		}
		logrus.Debugf("running host preflights")
		if err := RunHostPreflights(c); err != nil {
			err := fmt.Errorf("unable to finish preflight checks: %w", err)
			metrics.ReportApplyFinished(c, err)
			return err
		}
		logrus.Debugf("creating k0s configuration file")
		var cfg *k0sconfig.ClusterConfig
		var err error
		if cfg, err = ensureK0sConfig(c); err != nil {
			err := fmt.Errorf("unable to create config file: %w", err)
			metrics.ReportApplyFinished(c, err)
			return err
		}
		var proxy *Proxy
		if c.String("http-proxy") != "" || c.String("https-proxy") != "" || c.String("no-proxy") != "" {
			proxy = &Proxy{
				HTTPProxy:  c.String("http-proxy"),
				HTTPSProxy: c.String("https-proxy"),
				NoProxy:    strings.Join(append(defaults.DefaultNoProxy, c.String("no-proxy"), cfg.Spec.Network.PodCIDR, cfg.Spec.Network.ServiceCIDR), ","),
			}
		}
		logrus.Debugf("creating systemd unit files")
		if err := createSystemdUnitFiles(false, proxy); err != nil {
			err := fmt.Errorf("unable to create systemd unit files: %w", err)
			metrics.ReportApplyFinished(c, err)
			return err
		}
		logrus.Debugf("installing k0s")
		if err := installK0s(); err != nil {
			err := fmt.Errorf("unable update cluster: %w", err)
			metrics.ReportApplyFinished(c, err)
			return err
		}
		logrus.Debugf("waiting for k0s to be ready")
		if err := waitForK0s(); err != nil {
			err := fmt.Errorf("unable to wait for node: %w", err)
			metrics.ReportApplyFinished(c, err)
			return err
		}
		logrus.Debugf("running outro")
		if err := runOutro(c, cfg); err != nil {
			metrics.ReportApplyFinished(c, err)
			return err
		}
		metrics.ReportApplyFinished(c, nil)
		return nil
	},
}
