package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/configutils"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// Install2Cmd returns a cobra command for installing the embedded cluster.
// This is the upcoming version of install without the operator and where
// install does all of the work. This is a hidden command until it's tested
// and ready.
func Install2Cmd(ctx context.Context, name string) *cobra.Command {
	var (
		adminConsolePassword    string
		adminConsolePort        int
		airgapBundle            string
		dataDir                 string
		licenseFile             string
		localArtifactMirrorPort int
		networkInterface        string
		assumeYes               bool
		overrides               string
		privateCAs              []string
		skipHostPreflights      bool
		ignoreHostPreflights    bool
		configValues            string

		proxy *ecv1beta1.ProxySpec
	)

	cmd := &cobra.Command{
		Use:          "install2",
		Short:        fmt.Sprintf("Experimental installer for %s", name),
		Hidden:       true,
		SilenceUsage: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("install command must be run as root")
			}

			if skipHostPreflights {
				logrus.Warnf("Warning: --skip-host-preflights is deprecated and will be removed in a later version. Use --ignore-host-preflights instead.")
			}

			p, err := parseProxyFlags(cmd)
			if err != nil {
				return err
			}
			proxy = p

			if err := parseCIDRFlags(cmd); err != nil {
				return err
			}

			// validate the the license is indeed a license file
			_, err = helpers.ParseLicense(licenseFile)
			if err != nil {
				if err == helpers.ErrNotALicenseFile {
					return fmt.Errorf("license file is not a valid license file")
				}

				return fmt.Errorf("unable to parse license file: %w", err)
			}

			runtimeconfig.ApplyFlags(cmd.Flags())
			os.Setenv("TMPDIR", runtimeconfig.EmbeddedClusterTmpSubDir())

			if err := runtimeconfig.WriteToDisk(); err != nil {
				return fmt.Errorf("unable to write runtime config to disk: %w", err)
			}

			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			runtimeconfig.Cleanup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
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

			if channelRelease != nil && channelRelease.Airgap && airgapBundle == "" && !assumeYes {
				logrus.Warnf("You downloaded an air gap bundle but didn't provide it with --airgap-bundle.")
				logrus.Warnf("If you continue, the installation will not use an air gap bundle and will connect to the internet.")
				if !prompts.New().Confirm("Do you want to proceed with an online installation?", false) {
					return ErrNothingElseToAdd
				}
			}

			metrics.ReportApplyStarted(cmd.Context(), licenseFile)

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
				if err := maybePromptForAppUpdate(cmd.Context(), prompts.New(), license, assumeYes); err != nil {
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

			_, err = maybeAskAdminConsolePassword(cmd, assumeYes)
			if err != nil {
				metrics.ReportApplyFinished(cmd.Context(), licenseFile, err)
				return err
			}

			logrus.Debugf("materializing binaries")
			if err := materializeFiles(airgapBundle); err != nil {
				metrics.ReportApplyFinished(cmd.Context(), licenseFile, err)
				return err
			}

			// run host preglights
			logrus.Debugf("running host preflights")
			fromCIDR, toCIDR, err := getPODAndServiceCIDR(cmd)
			if err != nil {
				return fmt.Errorf("unable to determine pod and service CIDRs: %w", err)
			}

			preflightOptions := preflights.PrepareAndRunOptions{
				License:              license,
				Proxy:                proxy,
				ConnectivityFromCIDR: fromCIDR,
				ConnectivityToCIDR:   toCIDR,
			}
			if err := preflights.PrepareAndRun(cmd.Context(), preflightOptions); err != nil {
				return fmt.Errorf("unable to prepare and run preflights: %w", err)
			}

			logrus.Debugf("configuring sysctl")
			if err := configutils.ConfigureSysctl(); err != nil {
				return fmt.Errorf("unable to configure sysctl: %w", err)
			}

			logrus.Debugf("configuring network manager")
			if err := configureNetworkManager(cmd.Context()); err != nil {
				return fmt.Errorf("unable to configure network manager: %w", err)
			}

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
	cmd.Flags().BoolVar(&assumeYes, "yes", false, "Assume yes to all prompts.")

	cmd.Flags().StringVar(&overrides, "overrides", "", "File with an EmbeddedClusterConfig object to override the default configuration")
	cmd.Flags().MarkHidden("overrides")

	cmd.Flags().StringSliceVar(&privateCAs, "private-ca", []string{}, "Path to a trusted private CA certificate file")

	cmd.Flags().BoolVar(&skipHostPreflights, "skip-host-preflights", false, "Skip host preflight checks. This is not recommended and has been deprecated.")
	cmd.Flags().MarkHidden("skip-host-preflights")
	cmd.Flags().MarkDeprecated("skip-host-preflights", "This flag is deprecated and will be removed in a future version. Use --ignore-host-preflights instead.")

	cmd.Flags().BoolVar(&ignoreHostPreflights, "ignore-host-preflights", false, "Run host preflight checks, but prompt the user to continue if they fail instead of exiting.")
	cmd.Flags().StringVar(&configValues, "config-values", "", "path to a manifest containing config values (must be apiVersion: kots.io/v1beta1, kind: ConfigValues)")

	addProxyFlags(cmd)
	addCIDRFlags(cmd)
	cmd.Flags().SetNormalizeFunc(normalizeNoPromptToYes)

	return cmd
}
