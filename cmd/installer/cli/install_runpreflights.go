package cli

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/configutils"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/goods"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/embedded-cluster/pkg/support"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// ErrNothingElseToAdd is an error returned when there is nothing else to add to the
// screen. This is useful when we want to exit an error from a function here but
// don't want to print anything else (possibly because we have already printed the
// necessary data to the screen).
var ErrNothingElseToAdd = fmt.Errorf("")

// ErrPreflightsHaveFail is an error returned when we managed to execute the
// host preflights but they contain failures. We use this to differentiate the
// way we provide user feedback.
var ErrPreflightsHaveFail = fmt.Errorf("host preflight failures detected")

func InstallRunPreflightsCmd(ctx context.Context, name string) *cobra.Command {
	var (
		airgapBundle         string
		licenseFile          string
		noPrompt             bool
		networkInterface     string
		adminConsolePassword string
		overrides            string
		privateCAs           []string
		configValues         string
		podCIDR              string
		serviceCIDR          string
		cidr                 string
		skipHostPreflights   bool
		ignoreHostPreflights bool

		proxy *ecv1beta1.ProxySpec
	)

	cmd := &cobra.Command{
		Use:   "run-preflights",
		Short: "Run install host preflights",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("run-preflights command must be run as root")
			}

			// bind proxy flags
			proxy = &ecv1beta1.ProxySpec{}

			proxyFlag, err := cmd.Flags().GetBool("proxy")
			if err != nil {
				return fmt.Errorf("unable to get proxy flag: %w", err)
			}
			if proxyFlag {
				proxy.HTTPProxy = os.Getenv("HTTP_PROXY")
				proxy.HTTPSProxy = os.Getenv("HTTPS_PROXY")
				if os.Getenv("NO_PROXY") != "" {
					proxy.ProvidedNoProxy = os.Getenv("NO_PROXY")
				}
			}

			httpProxyFlag, err := cmd.Flags().GetString("http-proxy")
			if err != nil {
				return fmt.Errorf("unable to get http-proxy flag: %w", err)
			}
			proxy.HTTPProxy = httpProxyFlag

			httpsProxyFlag, err := cmd.Flags().GetString("https-proxy")
			if err != nil {
				return fmt.Errorf("unable to get https-proxy flag: %w", err)
			}
			proxy.HTTPSProxy = httpsProxyFlag

			noProxyFlag, err := cmd.Flags().GetString("no-proxy")
			if err != nil {
				return fmt.Errorf("unable to get no-proxy flag: %w", err)
			}
			proxy.ProvidedNoProxy = noProxyFlag

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			runtimeConfig := ecv1beta1.GetDefaultRuntimeConfig()

			provider := defaults.NewProviderFromRuntimeConfig(runtimeConfig)
			os.Setenv("TMPDIR", provider.EmbeddedClusterTmpSubDir())

			defer tryRemoveTmpDirContents(provider)

			var err error
			proxy, err := getProxySpecFromFlags(cmd)
			if err != nil {
				return fmt.Errorf("unable to get proxy spec from flags: %w", err)
			}

			proxy, err = includeLocalIPInNoProxy(cmd, proxy)
			if err != nil {
				return err
			}
			setProxyEnv(proxy)

			licenseFile, err := cmd.Flags().GetString("license")
			if err != nil {
				return fmt.Errorf("unable to get license flag: %w", err)
			}
			license, err := getLicenseFromFilepath(licenseFile)
			if err != nil {
				return err
			}

			airgapBundle, err := cmd.Flags().GetString("airgap-bundle")
			if err != nil {
				return fmt.Errorf("unable to get airgap-bundle flag: %w", err)
			}
			isAirgap := false
			if airgapBundle != "" {
				isAirgap = true
			}

			logrus.Debugf("materializing binaries")
			if err := materializeFiles(cmd, provider); err != nil {
				return err
			}

			if err := configutils.ConfigureSysctl(provider); err != nil {
				return err
			}

			opts := addonsApplierOpts{
				noPrompt:     noPrompt,
				license:      "",
				airgapBundle: airgapBundle,
				overrides:    overrides,
				privateCAs:   privateCAs,
				configValues: configValues,
			}
			applier, err := getAddonsApplier(cmd, opts, runtimeConfig, "", proxy)
			if err != nil {
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
				if err == ErrPreflightsHaveFail {
					return ErrNothingElseToAdd
				}
				return err
			}

			logrus.Info("Host preflights completed successfully")

			return nil
		},
	}

	cmd.Flags().StringVar(&airgapBundle, "airgap-bundle", "", "Path to the air gap bundle. If set, the installation will complete without internet access.")
	cmd.Flags().MarkHidden("airgap-bundle")

	cmd.Flags().StringVarP(&licenseFile, "license", "l", "", "Path to the license file")

	cmd.Flags().BoolVar(&noPrompt, "no-prompt", false, "Disable interactive prompts.")
	cmd.Flags().StringVar(&networkInterface, "network-interface", "", "The network interface to use for the cluster")
	cmd.Flags().StringVar(&adminConsolePassword, "admin-console-password", "", "Password for the Admin Console")

	cmd.Flags().StringVar(&overrides, "overrides", "", "File with an EmbeddedClusterConfig object to override the default configuration")
	cmd.Flags().MarkHidden("overrides")

	cmd.Flags().StringSliceVar(&privateCAs, "private-ca", []string{}, "Path to a trusted private CA certificate file")
	cmd.Flags().StringVar(&configValues, "config-values", "", "path to a manifest containing config values (must be apiVersion: kots.io/v1beta1, kind: ConfigValues)")

	cmd.Flags().StringVar(&podCIDR, "pod-cidr", "", "IP address range for Pods")
	cmd.Flags().StringVar(&serviceCIDR, "service-cidr", "", "IP address range for Service CIDR")
	cmd.Flags().StringVar(&cidr, "cidr", ecv1beta1.DefaultNetworkCIDR, "IP address range for the cluster")

	cmd.Flags().BoolVar(&skipHostPreflights, "skip-host-preflights", false, "Skip host preflight checks. This is not recommended and has been deprecated.")
	cmd.Flags().MarkHidden("skip-host-preflights")
	cmd.Flags().BoolVar(&ignoreHostPreflights, "ignore-host-preflights", false, "Run host preflight checks, but prompt the user to continue if they fail instead of exiting.")

	cmd.Flags().String("http-proxy", "", "HTTP proxy to use for the installation")
	cmd.Flags().String("https-proxy", "", "HTTPS proxy to use for the installation")
	cmd.Flags().String("no-proxy", "", "Comma-separated list of hosts for which not to use a proxy")
	cmd.Flags().Bool("proxy", false, "Use the system proxy settings for the install operation. These variables are currently only passed through to Velero and the Admin Console.")
	cmd.Flags().MarkHidden("proxy")

	return cmd
}

func tryRemoveTmpDirContents(provider *defaults.Provider) {
	err := helpers.RemoveAll(provider.EmbeddedClusterTmpSubDir())
	if err != nil {
		logrus.Debugf("failed to remove tmp dir contents: %v", err)
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

func materializeFiles(cmd *cobra.Command, provider *defaults.Provider) error {
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

	airgapBundleFlag, err := cmd.Flags().GetString("airgap-bundle")
	if err != nil {
		return fmt.Errorf("unable to get airgap-bundle flag: %w", err)
	}

	if airgapBundleFlag != "" {
		mat.Infof("Materializing airgap installation files")

		// read file from path
		rawfile, err := os.Open(airgapBundleFlag)
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

type addonsApplierOpts struct {
	noPrompt     bool
	license      string
	airgapBundle string
	overrides    string
	privateCAs   []string
	configValues string
}

func getAddonsApplier(cmd *cobra.Command, opts addonsApplierOpts, runtimeConfig *ecv1beta1.RuntimeConfigSpec, adminConsolePwd string, proxy *ecv1beta1.ProxySpec) (*addons.Applier, error) {
	addonOpts := []addons.Option{}
	addonOpts = append(addonOpts, addons.WithRuntimeConfig(runtimeConfig))

	if opts.noPrompt {
		addonOpts = append(addonOpts, addons.WithoutPrompt())
	}

	if opts.license != "" {
		license, err := helpers.ParseLicense(opts.license)
		if err != nil {
			return nil, fmt.Errorf("unable to parse license: %w", err)
		}

		addonOpts = append(addonOpts, addons.WithLicense(license))
		addonOpts = append(addonOpts, addons.WithLicenseFile(opts.license))
	}

	if opts.airgapBundle != "" {
		addonOpts = append(addonOpts, addons.WithAirgapBundle(opts.airgapBundle))
	}

	if proxy != nil {
		addonOpts = append(addonOpts, addons.WithProxy(proxy))
	}

	if opts.overrides != "" {
		eucfg, err := helpers.ParseEndUserConfig(opts.overrides)
		if err != nil {
			return nil, fmt.Errorf("unable to process overrides file: %w", err)
		}
		addonOpts = append(addonOpts, addons.WithEndUserConfig(eucfg))
	}

	if len(opts.privateCAs) > 0 {
		privateCAs := map[string]string{}
		for i, path := range opts.privateCAs {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("unable to read private CA file %s: %w", path, err)
			}
			name := fmt.Sprintf("ca_%d.crt", i)
			privateCAs[name] = string(data)
		}
		addonOpts = append(addonOpts, addons.WithPrivateCAs(privateCAs))
	}

	if adminConsolePwd != "" {
		addonOpts = append(addonOpts, addons.WithAdminConsolePassword(adminConsolePwd))
	}

	if opts.configValues != "" {
		err := configutils.ValidateKotsConfigValues(opts.configValues)
		if err != nil {
			return nil, fmt.Errorf("unable to validate config values file %q: %w", opts.configValues, err)
		}

		addonOpts = append(addonOpts, addons.WithConfigValuesFile(opts.configValues))
	}

	return addons.NewApplier(addonOpts...), nil
}

// RunHostPreflights runs the host preflights we found embedded in the binary
// on all configured hosts. We attempt to read HostPreflights from all the
// embedded Helm Charts and from the Kots Application Release files.
func RunHostPreflights(cmd *cobra.Command, provider *defaults.Provider, applier *addons.Applier, replicatedAPIURL, proxyRegistryURL string, isAirgap bool, proxy *ecv1beta1.ProxySpec, fromCIDR, toCIDR string) error {
	hpf, err := applier.HostPreflights()
	if err != nil {
		return fmt.Errorf("unable to read host preflights: %w", err)
	}

	privateCAs := getPrivateCAPath(cmd)

	data, err := preflights.TemplateData{
		ReplicatedAPIURL:        replicatedAPIURL,
		ProxyRegistryURL:        proxyRegistryURL,
		IsAirgap:                isAirgap,
		AdminConsolePort:        provider.AdminConsolePort(),
		LocalArtifactMirrorPort: provider.LocalArtifactMirrorPort(),
		DataDir:                 provider.EmbeddedClusterHomeDirectory(),
		K0sDataDir:              provider.EmbeddedClusterK0sSubDir(),
		OpenEBSDataDir:          provider.EmbeddedClusterOpenEBSLocalSubDir(),
		PrivateCA:               privateCAs,
		SystemArchitecture:      runtime.GOARCH,
		FromCIDR:                fromCIDR,
		ToCIDR:                  toCIDR,
	}.WithCIDRData(getCIDRs(cmd))

	if err != nil {
		return fmt.Errorf("unable to get host preflights data: %w", err)
	}

	if proxy != nil {
		data.HTTPProxy = proxy.HTTPProxy
		data.HTTPSProxy = proxy.HTTPSProxy
		data.ProvidedNoProxy = proxy.ProvidedNoProxy
		data.NoProxy = proxy.NoProxy
	}

	chpfs, err := preflights.GetClusterHostPreflights(cmd.Context(), data)
	if err != nil {
		return fmt.Errorf("unable to get cluster host preflights: %w", err)
	}

	for _, h := range chpfs {
		hpf.Collectors = append(hpf.Collectors, h.Spec.Collectors...)
		hpf.Analyzers = append(hpf.Analyzers, h.Spec.Analyzers...)
	}

	if dryrun.Enabled() {
		dryrun.RecordHostPreflightSpec(hpf)
		return nil
	}

	return runHostPreflights(cmd, provider, hpf, proxy)
}

func runHostPreflights(cmd *cobra.Command, provider *defaults.Provider, hpf *v1beta2.HostPreflightSpec, proxy *ecv1beta1.ProxySpec) error {
	if len(hpf.Collectors) == 0 && len(hpf.Analyzers) == 0 {
		return nil
	}
	pb := spinner.Start()

	skipHostPreflightsFlag, err := cmd.Flags().GetBool("skip-host-preflights")
	if err != nil {
		pb.CloseWithError()
		return fmt.Errorf("unable to get skip-host-preflights flag: %w", err)
	}
	if skipHostPreflightsFlag {
		pb.Infof("Host preflights skipped")
		pb.Close()
		return nil
	}
	pb.Infof("Running host preflights")
	output, stderr, err := preflights.Run(cmd.Context(), provider, hpf, proxy)
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

		noPromptFlag, err := cmd.Flags().GetBool("no-prompt")
		if err != nil {
			return fmt.Errorf("unable to get no-prompt flag: %w", err)
		}
		if noPromptFlag {
			// We have warnings but we are not in interactive mode
			// so we just print the warnings and continue
			pb.Close()
			output.PrintTableWithoutInfo()
			return nil
		}
		pb.Close()
		output.PrintTableWithoutInfo()
		if !prompts.New().Confirm("Do you want to continue?", false) {
			pb.Close()
			return fmt.Errorf("user aborted")
		}
		return nil
	}

	// No failures or warnings
	pb.Infof("Host preflights succeeded!")
	pb.Close()
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

// return only the first private CA path for now - troubleshoot needs a refactor to support multiple CAs in the future
func getPrivateCAPath(cmd *cobra.Command) string {
	privateCA := ""

	privateCAsFlag, err := cmd.Flags().GetStringSlice("private-ca")
	if err != nil {
		return ""
	}
	if len(privateCAsFlag) > 0 {
		privateCA = privateCAsFlag[0]
	}
	return privateCA
}

// getCIDRs returns the CIDRs in use based on the provided cli flags.
func getCIDRs(cmd *cobra.Command) (string, string, string) {
	podCIDRFlag, err := cmd.Flags().GetString("pod-cidr")
	if err != nil {
		return "", "", ""
	}

	serviceCIDRFlag, err := cmd.Flags().GetString("service-cidr")
	if err != nil {
		return "", "", ""
	}

	cidrFlag, err := cmd.Flags().GetString("cidr")
	if err != nil {
		return "", "", ""
	}

	if podCIDRFlag != "" || serviceCIDRFlag != "" {
		return podCIDRFlag, serviceCIDRFlag, ""
	}
	return "", "", cidrFlag
}
