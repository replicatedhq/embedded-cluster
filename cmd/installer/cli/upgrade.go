package cli

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"syscall"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/goods"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	rcutil "github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig/util"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/replicatedhq/embedded-cluster/web"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// UpgradeCmdFlags holds command-line flags for the upgrade command
type UpgradeCmdFlags struct {
	target       string
	licenseFile  string
	assumeYes    bool
	overrides    string
	airgapBundle string
	managerPort  int
}

// upgradeConfig holds configuration data gathered during upgrade preparation
type upgradeConfig struct {
	passwordHash         []byte
	tlsConfig            apitypes.TLSConfig
	tlsCert              *tls.Certificate
	license              *kotsv1beta1.License
	licenseBytes         []byte
	airgapMetadata       *airgap.AirgapMetadata
	embeddedAssetsSize   int64
	configValues         apitypes.AppConfigValues
	endUserConfig        *ecv1beta1.Config
	clusterID            string
	managerPort          int
	requiresInfraUpgrade bool
	kotsadmNamespace     string
}

// UpgradeCmd returns a cobra command for upgrading the embedded cluster application.
func UpgradeCmd(ctx context.Context, appSlug, appTitle string) *cobra.Command {
	var flags UpgradeCmdFlags
	var upgradeConfig upgradeConfig

	ctx, cancel := context.WithCancel(ctx)
	rc := runtimeconfig.New(nil)
	short := fmt.Sprintf("Upgrade %s onto Linux", appTitle)

	cmd := &cobra.Command{
		Use:     "upgrade",
		Short:   short,
		Example: upgradeCmdExample(appSlug),
		PostRun: func(cmd *cobra.Command, args []string) {
			rc.Cleanup()
			cancel() // Cancel context when command completes
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Add check for kubernetes target when kubernetes support is added
			if !slices.Contains([]string{"linux"}, flags.target) {
				return fmt.Errorf(`invalid --target (must be: "linux")`)
			}

			if os.Getuid() != 0 {
				return fmt.Errorf("upgrade command must be run as root")
			}

			// set the umask to 022 so that we can create files/directories with 755 permissions
			// this does not return an error - it returns the previous umask
			_ = syscall.Umask(0o022)

			// Set up environment variables from existing runtime config
			existingRC, err := rcutil.GetRuntimeConfigFromCluster(ctx)
			if err != nil {
				return fmt.Errorf("failed to get runtime config from cluster: %w", err)
			}
			if err := existingRC.SetEnv(); err != nil {
				return fmt.Errorf("failed to set environment variables: %w", err)
			}

			// Set up kubernetes client
			kcli, err := kubeutils.KubeClient()
			if err != nil {
				return fmt.Errorf("failed to create kubernetes client: %w", err)
			}

			if err := preRunUpgrade(ctx, flags, &upgradeConfig, existingRC, kcli, appSlug); err != nil {
				return err
			}
			if err := verifyAndPromptUpgrade(ctx, flags, upgradeConfig, prompts.New()); err != nil {
				return err
			}

			targetVersion := versions.Version
			initialVersion := ""
			currentInstallation, err := kubeutils.GetLatestInstallation(ctx, kcli)
			if err != nil {
				return fmt.Errorf("failed to get latest installation: %w", err)
			}
			if currentInstallation.Spec.Config != nil {
				initialVersion = currentInstallation.Spec.Config.Version
			}

			metricsReporter := newUpgradeReporter(
				replicatedAppURL(), cmd.CalledAs(), flagsToStringSlice(cmd.Flags()),
				upgradeConfig.license.Spec.LicenseID, upgradeConfig.clusterID, upgradeConfig.license.Spec.AppSlug,
				targetVersion, initialVersion,
			)
			metricsReporter.ReportUpgradeStarted(ctx)

			// Run the manager experience upgrade - the upgrade controller will handle
			// reporting success/failure events through its event handlers
			if err := runManagerExperienceUpgrade(
				ctx, flags, upgradeConfig, existingRC, metricsReporter.reporter, appTitle,
				targetVersion, initialVersion,
			); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.SetUsageTemplate(upgradeUsageTemplateV3Linux)

	mustAddUpgradeFlags(cmd, &flags)

	if err := addUpgradeAdminConsoleFlags(cmd, &flags); err != nil {
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
`
)

func upgradeCmdExample(appSlug string) string {
	return fmt.Sprintf(upgradeCmdExampleText, appSlug)
}

func mustAddUpgradeFlags(cmd *cobra.Command, flags *UpgradeCmdFlags) {
	normalizeFuncs := []func(f *pflag.FlagSet, name string) pflag.NormalizedName{}

	commonFlagSet := newCommonUpgradeFlags(flags)
	cmd.Flags().AddFlagSet(commonFlagSet)
	if fn := commonFlagSet.GetNormalizeFunc(); fn != nil {
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

	flagSet.BoolVarP(&flags.assumeYes, "yes", "y", false, "Assume yes to all prompts.")
	flagSet.SetNormalizeFunc(normalizeNoPromptToYes)

	return flagSet
}

func addUpgradeAdminConsoleFlags(cmd *cobra.Command, flags *UpgradeCmdFlags) error {
	cmd.Flags().StringVarP(&flags.licenseFile, "license", "l", "", "Path to the license file")
	mustMarkFlagRequired(cmd.Flags(), "license")

	return nil
}

func addUpgradeManagementConsoleFlags(cmd *cobra.Command, flags *UpgradeCmdFlags) error {
	// default value of 0 indicates no user input - will use existing runtime config value
	cmd.Flags().IntVar(&flags.managerPort, "manager-port", 0, "Port on which the Manager will be served")
	return nil
}

func preRunUpgrade(ctx context.Context, flags UpgradeCmdFlags, upgradeConfig *upgradeConfig, rc runtimeconfig.RuntimeConfig, kcli client.Client, appSlug string) error {
	// Verify an installation exists and get the cluster ID
	clusterID, err := getClusterID(ctx, kcli)
	if err != nil {
		return fmt.Errorf("failed to get existing installation: %w", err)
	}
	upgradeConfig.clusterID = clusterID

	// Verify that a data directory exists
	dataDir := rc.EmbeddedClusterHomeDirectory()
	if _, err := os.Stat(dataDir); err != nil {
		return fmt.Errorf("failed to stat data directory: %w", err)
	}

	// Validate the license is indeed a license file
	license, err := getLicenseFromFilepath(flags.licenseFile)
	if err != nil {
		return err
	}
	upgradeConfig.license = license
	data, err := os.ReadFile(flags.licenseFile)
	if err != nil {
		return fmt.Errorf("failed to read license file: %w", err)
	}
	upgradeConfig.licenseBytes = data

	// Continue using "kotsadm" namespace if it exists for backwards compatibility, otherwise use the appSlug
	ns, err := runtimeconfig.KotsadmNamespace(ctx, kcli)
	if err != nil {
		return fmt.Errorf("get kotsadm namespace: %w", err)
	}
	upgradeConfig.kotsadmNamespace = ns

	if flags.airgapBundle != "" {
		metadata, err := airgap.AirgapMetadataFromPath(flags.airgapBundle)
		if err != nil {
			return fmt.Errorf("failed to get airgap info: %w", err)
		}
		upgradeConfig.airgapMetadata = metadata
	}

	assetsSize, err := goods.SizeOfEmbeddedAssets()
	if err != nil {
		return fmt.Errorf("failed to get size of embedded files: %w", err)
	}
	upgradeConfig.embeddedAssetsSize = assetsSize

	// Read existing TLS certificates from kotsadm-tls secret in the cluster
	tlsConfig, err := readTLSConfig(ctx, kcli, upgradeConfig.kotsadmNamespace)
	if err != nil {
		return fmt.Errorf("failed to read tls config: %w", err)
	}
	upgradeConfig.tlsConfig = tlsConfig
	cert, err := tls.X509KeyPair(upgradeConfig.tlsConfig.CertBytes, upgradeConfig.tlsConfig.KeyBytes)
	if err != nil {
		return fmt.Errorf("failed to create TLS certificate from data: %w", err)
	}
	upgradeConfig.tlsCert = &cert

	// Read password hash from the kotsadm-password secret in the cluster
	pwdHash, err := readPasswordHash(ctx, kcli, upgradeConfig.kotsadmNamespace)
	if err != nil {
		return fmt.Errorf("failed to read password hash from cluster: %w", err)
	}
	upgradeConfig.passwordHash = pwdHash

	// Use the user-provided manager port if specified, otherwise use the existing runtime config value
	if flags.managerPort != 0 {
		// User provided a custom manager port
		upgradeConfig.managerPort = flags.managerPort
	} else {
		// Use existing manager port from runtime config
		upgradeConfig.managerPort = rc.ManagerPort()
	}

	eucfg, err := helpers.ParseEndUserConfig(flags.overrides)
	if err != nil {
		return fmt.Errorf("failed to process overrides file: %w", err)
	}
	upgradeConfig.endUserConfig = eucfg

	cv, err := getCurrentConfigValues(appSlug, upgradeConfig.clusterID, upgradeConfig.kotsadmNamespace)
	if err != nil {
		return fmt.Errorf("failed to get current config values: %w", err)
	}
	upgradeConfig.configValues = cv

	// Check if infrastructure upgrade is required
	requiresInfraUpgrade, err := checkRequiresInfraUpgrade(ctx)
	if err != nil {
		return fmt.Errorf("check if infrastructure upgrade is required: %w", err)
	}
	upgradeConfig.requiresInfraUpgrade = requiresInfraUpgrade

	return nil
}

func verifyAndPromptUpgrade(ctx context.Context, flags UpgradeCmdFlags, upgradeConfig upgradeConfig, prompt prompts.Prompt) error {
	isAirgap := flags.airgapBundle != ""

	err := verifyChannelRelease("upgrade", isAirgap, flags.assumeYes)
	if err != nil {
		return err
	}

	if upgradeConfig.airgapMetadata != nil && upgradeConfig.airgapMetadata.AirgapInfo != nil {
		logrus.Debugf("checking airgap bundle matches binary")
		if err := checkAirgapMatches(upgradeConfig.airgapMetadata.AirgapInfo); err != nil {
			return err // we want the user to see the error message without a prefix
		}
	}

	if !isAirgap {
		if err := maybePromptForAppUpdate(ctx, prompt, upgradeConfig.license, flags.assumeYes); err != nil {
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

func getCurrentConfigValues(appSlug string, clusterID string, namespace string) (apitypes.AppConfigValues, error) {
	// Get the kots config YAML using the kotscli package
	configYaml, err := kotscli.GetConfigValues(kotscli.GetConfigValuesOptions{
		AppSlug:               appSlug,
		Namespace:             namespace,
		ClusterID:             clusterID,
		ReplicatedAppEndpoint: replicatedAppURL(),
	})
	if err != nil {
		return nil, fmt.Errorf("get current config values for app %s: %w", appSlug, err)
	}

	// Return empty AppConfigValues if no config values were returned by kots
	// It is valid for an app to have no config values
	if strings.TrimSpace(configYaml) == "" {
		return apitypes.AppConfigValues{}, nil
	}

	// Parse the YAML using helpers
	kotsConfigValues, err := helpers.ParseConfigValuesFromString(configYaml)
	if err != nil {
		return nil, fmt.Errorf("parse config values YAML for app %s: %w", appSlug, err)
	}

	// Convert to AppConfigValues format
	return apitypes.ConvertToAppConfigValues(kotsConfigValues), nil
}

// getClusterID gets the cluster ID from the latest installation in the cluster
func getClusterID(ctx context.Context, kcli client.Client) (string, error) {
	installation, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		return "", fmt.Errorf("get latest installation: %w", err)
	}
	if installation.Spec.ClusterID == "" {
		return "", fmt.Errorf("existing installation has empty cluster ID")
	}

	return installation.Spec.ClusterID, nil
}

// readTLSConfig reads the TLS certificate from the kotsadm-tls secret
func readTLSConfig(ctx context.Context, kcli client.Client, namespace string) (apitypes.TLSConfig, error) {
	var tlsConfig apitypes.TLSConfig

	tlsSecret := &corev1.Secret{}
	err := kcli.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      "kotsadm-tls",
	}, tlsSecret)
	if err != nil {
		return tlsConfig, fmt.Errorf("read kotsadm-tls secret from cluster: %w", err)
	}

	certData, hasCert := tlsSecret.Data["tls.crt"]
	keyData, hasKey := tlsSecret.Data["tls.key"]

	if !hasCert || !hasKey || len(certData) == 0 || len(keyData) == 0 {
		return tlsConfig, fmt.Errorf("kotsadm-tls secret is missing required tls.crt or tls.key data")
	}

	return apitypes.TLSConfig{
		CertBytes: certData,
		KeyBytes:  keyData,
		Hostname:  tlsSecret.StringData["hostname"],
	}, nil
}

// readPasswordHash reads the bcrypt password hash from the kotsadm-password secret
func readPasswordHash(ctx context.Context, kcli client.Client, namespace string) ([]byte, error) {
	pwdSecret := &corev1.Secret{}

	err := kcli.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      "kotsadm-password",
	}, pwdSecret)
	if err != nil {
		return nil, fmt.Errorf("read kotsadm-password secret from cluster: %w", err)
	}

	passwordBcryptData, hasData := pwdSecret.Data["passwordBcrypt"]
	if !hasData {
		return nil, fmt.Errorf("kotsadm-password secret is missing required passwordBcrypt data")
	}

	return passwordBcryptData, nil
}

func runManagerExperienceUpgrade(
	ctx context.Context, flags UpgradeCmdFlags, upgradeConfig upgradeConfig, rc runtimeconfig.RuntimeConfig,
	metricsReporter metrics.ReporterInterface, appTitle string, targetVersion string, initialVersion string,
) (finalErr error) {
	apiConfig := apiOptions{
		APIConfig: apitypes.APIConfig{
			InstallTarget:        apitypes.InstallTarget(flags.target),
			Password:             "", // Only PasswordHash is necessary for upgrades because the kotsadm-password secret has been created already
			PasswordHash:         upgradeConfig.passwordHash,
			TLSConfig:            upgradeConfig.tlsConfig,
			License:              upgradeConfig.licenseBytes,
			AirgapBundle:         flags.airgapBundle,
			AirgapMetadata:       upgradeConfig.airgapMetadata,
			EmbeddedAssetsSize:   upgradeConfig.embeddedAssetsSize,
			ConfigValues:         upgradeConfig.configValues,
			ReleaseData:          release.GetReleaseData(),
			EndUserConfig:        upgradeConfig.endUserConfig,
			ClusterID:            upgradeConfig.clusterID,
			Mode:                 apitypes.ModeUpgrade,
			TargetVersion:        targetVersion,
			InitialVersion:       initialVersion,
			RequiresInfraUpgrade: upgradeConfig.requiresInfraUpgrade,

			LinuxConfig: apitypes.LinuxConfig{
				RuntimeConfig: rc,
			},
		},
		ManagerPort:     upgradeConfig.managerPort,
		WebMode:         web.ModeUpgrade,
		MetricsReporter: metricsReporter,
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Upgrade is not headless, so pass false for headless and empty socket path
	if err := startAPI(ctx, upgradeConfig.tlsCert, apiConfig, cancel); err != nil {
		return fmt.Errorf("failed to start api: %w", err)
	}

	logrus.Infof("\nVisit the %s manager to continue the upgrade: %s\n",
		appTitle,
		getManagerURL(upgradeConfig.tlsConfig.Hostname, upgradeConfig.managerPort))
	<-ctx.Done()

	return nil
}

// checkRequiresInfraUpgrade determines if an infrastructure upgrade is required by comparing
// the current installation's embedded cluster config with the target embedded cluster config.
func checkRequiresInfraUpgrade(ctx context.Context) (bool, error) {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return false, fmt.Errorf("create kubernetes client: %w", err)
	}

	// Get current embedded cluster config spec from the cluster
	currentInstallation, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		return false, fmt.Errorf("get current installation: %w", err)
	}
	currentSpec := currentInstallation.Spec.Config

	// Get target embedded cluster config spec from release data
	releaseData := release.GetReleaseData()
	if releaseData == nil || releaseData.EmbeddedClusterConfig == nil {
		return false, fmt.Errorf("release data or embedded cluster config not found")
	}
	targetSpec := releaseData.EmbeddedClusterConfig.Spec

	// Marshal both to JSON for comparison (this is the original logic)
	currentJSON, err := json.Marshal(currentSpec)
	if err != nil {
		return false, fmt.Errorf("marshal current config: %w", err)
	}

	targetJSON, err := json.Marshal(targetSpec)
	if err != nil {
		return false, fmt.Errorf("marshal target config: %w", err)
	}

	return !bytes.Equal(currentJSON, targetJSON), nil
}
