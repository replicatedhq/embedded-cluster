package cli

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	addontypes "github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/disasterrecovery"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/metadata"
	k8snet "k8s.io/utils/net"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8syaml "sigs.k8s.io/yaml"
)

type ecRestoreState string

const (
	ecRestoreStateNew                  ecRestoreState = "new"
	ecRestoreStateConfirmBackup        ecRestoreState = "confirm-backup"
	ecRestoreStateRestoreECInstall     ecRestoreState = "restore-ec-install"
	ecRestoreStateRestoreAdminConsole  ecRestoreState = "restore-admin-console"
	ecRestoreStateWaitForNodes         ecRestoreState = "wait-for-nodes"
	ecRestoreStateRestoreSeaweedFS     ecRestoreState = "restore-seaweedfs"
	ecRestoreStateRestoreRegistry      ecRestoreState = "restore-registry"
	ecRestoreStateAdminConsoleEnableHA ecRestoreState = "admin-console-enable-ha"
	ecRestoreStateRestoreECO           ecRestoreState = "restore-embedded-cluster-operator"
	ecRestoreStateRestoreExtensions    ecRestoreState = "restore-extensions"
	ecRestoreStateRestoreApp           ecRestoreState = "restore-app"
)

var ecRestoreStates = []ecRestoreState{
	ecRestoreStateNew,
	ecRestoreStateConfirmBackup,
	ecRestoreStateRestoreECInstall,
	ecRestoreStateRestoreAdminConsole,
	ecRestoreStateWaitForNodes,
	ecRestoreStateRestoreSeaweedFS,
	ecRestoreStateRestoreRegistry,
	ecRestoreStateAdminConsoleEnableHA,
	ecRestoreStateRestoreECO,
	ecRestoreStateRestoreExtensions,
	ecRestoreStateRestoreApp,
}

const (
	resourceModifiersCMName = "restore-resource-modifiers"
)

func RestoreCmd(ctx context.Context, appSlug, appTitle string) *cobra.Command {
	var flags InstallCmdFlags

	var s3Store s3BackupStore
	var skipStoreValidation bool

	rc := runtimeconfig.New(nil)
	ki := kubernetesinstallation.New(nil)

	cmd := &cobra.Command{
		Use:   "restore",
		Short: fmt.Sprintf("Restore %s from a backup", appTitle),
		PostRun: func(cmd *cobra.Command, args []string) {
			rc.Cleanup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := preRunInstall(cmd, &flags, rc, ki); err != nil {
				return err
			}

			_ = rc.SetEnv()

			if err := runRestore(cmd.Context(), appSlug, appTitle, flags, rc, s3Store, skipStoreValidation); err != nil {
				return err
			}

			return nil
		},
	}

	addS3Flags(cmd, &s3Store)
	cmd.Flags().BoolVar(&skipStoreValidation, "skip-store-validation", false, "Skip validation of the backup storage location")

	mustAddInstallFlags(cmd, &flags)

	return cmd
}

func runRestore(ctx context.Context, appSlug, appTitle string, flags InstallCmdFlags, rc runtimeconfig.RuntimeConfig, s3Store s3BackupStore, skipStoreValidation bool) error {
	err := verifyChannelRelease("restore", flags.isAirgap, flags.assumeYes)
	if err != nil {
		return err
	}

	if flags.isAirgap {
		logrus.Debugf("checking airgap bundle matches binary")
		if err := checkAirgapMatches(flags.airgapBundle); err != nil {
			return err // we want the user to see the error message without a prefix
		}
	}

	logrus.Debugf("getting restore state")
	state := getECRestoreState(ctx)
	logrus.Debugf("restore state is: %q", state)

	if state != ecRestoreStateNew {
		shouldResume, err := prompts.New().Confirm("A previous restore operation was detected. Would you like to resume?", true)
		if err != nil {
			return fmt.Errorf("failed to get confirmation: %w", err)
		}
		logrus.Info("")
		if !shouldResume {
			state = ecRestoreStateNew
		}
	}

	// if the user wants to resume, check if a backup has already been picked.
	var backupToRestore *disasterrecovery.ReplicatedBackup
	if state != ecRestoreStateNew {
		logrus.Debugf("getting backup from restore state")
		var err error
		backupToRestore, err = getBackupFromRestoreState(ctx, flags.isAirgap, rc)
		if err != nil {
			return fmt.Errorf("unable to resume: %w", err)
		}
		if backupToRestore != nil {
			completionTimestamp := backupToRestore.GetCompletionTimestamp().Format("2006-01-02 15:04:05 UTC")
			logrus.Infof("Resuming restore from backup %q (%s)\n", backupToRestore.GetName(), completionTimestamp)

			if err := overrideRuntimeConfigFromBackup(flags.localArtifactMirrorPort, *backupToRestore, rc); err != nil {
				return fmt.Errorf("unable to override runtime config from backup: %w", err)
			}
		}
	}

	// If the installation is available, we can further augment the runtime config from the installation.
	rcSpec, err := getRuntimeConfigFromInstallation(ctx)
	if err != nil {
		logrus.Debugf(
			"Unable to get runtime config from installation, this is expected if the installation is not yet available (restore state=%s): %v",
			state, err,
		)
	} else {
		rc.Set(rcSpec)

		if err := rc.WriteToDisk(); err != nil {
			return fmt.Errorf("unable to write runtime config to disk: %w", err)
		}
	}

	_ = rc.SetEnv()

	switch state {
	case ecRestoreStateNew:
		err = runRestoreStepNew(ctx, appSlug, appTitle, flags, rc, &s3Store, skipStoreValidation)
		if err != nil {
			return err
		}

		fallthrough

	case ecRestoreStateConfirmBackup:
		logrus.Debugf("setting restore state to %q", ecRestoreStateConfirmBackup)
		err := setECRestoreState(ctx, ecRestoreStateConfirmBackup, "")
		if err != nil {
			return fmt.Errorf("unable to set restore state: %w", err)
		}

		backup, ok, err := runRestoreStepConfirmBackup(ctx, flags, rc)
		if err != nil {
			return err
		} else if !ok {
			return nil
		}
		backupToRestore = backup

		fallthrough

	case ecRestoreStateRestoreECInstall:
		logrus.Debugf("setting restore state to %q", ecRestoreStateRestoreECInstall)
		err := setECRestoreState(ctx, ecRestoreStateRestoreECInstall, backupToRestore.GetName())
		if err != nil {
			return fmt.Errorf("unable to set restore state: %w", err)
		}

		err = runRestoreECInstall(ctx, rc, backupToRestore)
		if err != nil {
			return err
		}

		fallthrough

	case ecRestoreStateRestoreAdminConsole:
		logrus.Debugf("setting restore state to %q", ecRestoreStateRestoreAdminConsole)
		err := setECRestoreState(ctx, ecRestoreStateRestoreAdminConsole, backupToRestore.GetName())
		if err != nil {
			return fmt.Errorf("unable to set restore state: %w", err)
		}

		err = runRestoreAdminConsole(ctx, backupToRestore)
		if err != nil {
			return err
		}

		fallthrough

	case ecRestoreStateWaitForNodes:
		logrus.Debugf("setting restore state to %q", ecRestoreStateWaitForNodes)
		err := setECRestoreState(ctx, ecRestoreStateWaitForNodes, backupToRestore.GetName())
		if err != nil {
			return fmt.Errorf("unable to set restore state: %w", err)
		}

		err = runRestoreWaitForNodes(ctx, flags, rc, backupToRestore)
		if err != nil {
			return err
		}

		fallthrough

	case ecRestoreStateRestoreSeaweedFS:
		logrus.Debugf("setting restore state to %q", ecRestoreStateRestoreSeaweedFS)
		err := setECRestoreState(ctx, ecRestoreStateRestoreSeaweedFS, backupToRestore.GetName())
		if err != nil {
			return fmt.Errorf("unable to set restore state: %w", err)
		}

		err = runRestoreSeaweedFS(ctx, flags, backupToRestore)
		if err != nil {
			return err
		}

		fallthrough

	case ecRestoreStateRestoreRegistry:
		logrus.Debugf("setting restore state to %q", ecRestoreStateRestoreRegistry)
		err := setECRestoreState(ctx, ecRestoreStateRestoreRegistry, backupToRestore.GetName())
		if err != nil {
			return fmt.Errorf("unable to set restore state: %w", err)
		}

		err = runRestoreRegistry(ctx, flags, backupToRestore)
		if err != nil {
			return err
		}

		fallthrough

	case ecRestoreStateAdminConsoleEnableHA:
		logrus.Debugf("setting restore state to %q", ecRestoreStateAdminConsoleEnableHA)
		err := setECRestoreState(ctx, ecRestoreStateAdminConsoleEnableHA, backupToRestore.GetName())
		if err != nil {
			return fmt.Errorf("unable to set restore state: %w", err)
		}

		err = runRestoreEnableAdminConsoleHA(ctx, flags, rc, backupToRestore)
		if err != nil {
			return err
		}

		fallthrough

	case ecRestoreStateRestoreECO:
		logrus.Debugf("setting restore state to %q", ecRestoreStateRestoreECO)
		err := setECRestoreState(ctx, ecRestoreStateRestoreECO, backupToRestore.GetName())
		if err != nil {
			return fmt.Errorf("unable to set restore state: %w", err)
		}

		err = runRestoreECO(ctx, backupToRestore)
		if err != nil {
			return err
		}

		fallthrough

	case ecRestoreStateRestoreExtensions:
		logrus.Debugf("setting restore state to %q", ecRestoreStateRestoreExtensions)
		err := setECRestoreState(ctx, ecRestoreStateRestoreExtensions, backupToRestore.GetName())
		if err != nil {
			return fmt.Errorf("unable to set restore state: %w", err)
		}

		err = runRestoreExtensions(ctx, flags, rc)
		if err != nil {
			return err
		}

		fallthrough

	case ecRestoreStateRestoreApp:
		logrus.Debugf("setting restore state to %q", ecRestoreStateRestoreApp)
		err := setECRestoreState(ctx, ecRestoreStateRestoreApp, backupToRestore.GetName())
		if err != nil {
			return fmt.Errorf("unable to set restore state: %w", err)
		}

		err = runRestoreApp(ctx, backupToRestore)
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("unknown restore state: %q", state)
	}

	return nil
}

func runRestoreStepNew(ctx context.Context, appSlug, appTitle string, flags InstallCmdFlags, rc runtimeconfig.RuntimeConfig, s3Store *s3BackupStore, skipStoreValidation bool) error {
	logrus.Debugf("checking if k0s is already installed")
	err := verifyNoInstallation(appSlug, "restore")
	if err != nil {
		return err
	}

	if !s3BackupStoreHasData(s3Store) {
		logrus.Infof("You'll be guided through the process of restoring %s from a backup.\n", appTitle)
		logrus.Info("Enter information to configure access to your backup storage location.\n")

		if err := promptForS3BackupStore(s3Store); err != nil {
			return fmt.Errorf("failed to prompt for backup store: %w", err)
		}
	}
	s3Store.prefix = strings.TrimPrefix(s3Store.prefix, "/")

	if !skipStoreValidation {
		logrus.Debugf("validating backup store configuration")
		if err := validateS3BackupStore(s3Store); err != nil {
			return fmt.Errorf("unable to validate backup store: %w", err)
		}
	}

	logrus.Debugf("configuring host")
	if err := hostutils.ConfigureHost(ctx, rc, hostutils.InitForInstallOptions{
		AirgapBundle: flags.airgapBundle,
	}); err != nil {
		return fmt.Errorf("configure host: %w", err)
	}

	logrus.Debugf("running install preflights")
	if err := runInstallPreflights(ctx, flags, rc, nil); err != nil {
		if errors.Is(err, preflights.ErrPreflightsHaveFail) {
			return NewErrorNothingElseToAdd(err)
		}
		return fmt.Errorf("unable to run install preflights: %w", err)
	}

	_, err = installAndStartCluster(ctx, flags, rc, nil)
	if err != nil {
		return err
	}

	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	mcli, err := kubeutils.MetadataClient()
	if err != nil {
		return fmt.Errorf("unable to create metadata client: %w", err)
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

	errCh := kubeutils.WaitForKubernetes(ctx, kcli)
	defer logKubernetesErrors(errCh)

	// TODO (@salah): update installation status to reflect what's happening

	logrus.Debugf("installing addons")
	if err := installAddonsForRestore(ctx, kcli, mcli, hcli, rc, flags); err != nil {
		return err
	}

	logrus.Debugf("configuring velero backup storage location")
	if err := kotscli.VeleroConfigureOtherS3(kotscli.VeleroConfigureOtherS3Options{
		RuntimeConfig:   rc,
		Endpoint:        s3Store.endpoint,
		Region:          s3Store.region,
		Bucket:          s3Store.bucket,
		Path:            s3Store.prefix,
		AccessKeyID:     s3Store.accessKeyID,
		SecretAccessKey: s3Store.secretAccessKey,
		Namespace:       constants.KotsadmNamespace,
	}); err != nil {
		return err
	}

	return nil
}

func installAddonsForRestore(ctx context.Context, kcli client.Client, mcli metadata.Interface, hcli helm.Client, rc runtimeconfig.RuntimeConfig, flags InstallCmdFlags) error {
	embCfg := release.GetEmbeddedClusterConfig()
	var embCfgSpec *ecv1beta1.ConfigSpec
	if embCfg != nil {
		embCfgSpec = &embCfg.Spec
	}

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

	if err := addOns.Restore(ctx, addons.RestoreOptions{
		EmbeddedConfigSpec: embCfgSpec,
		EndUserConfigSpec:  nil, // TODO: support for end user config overrides
		ProxySpec:          rc.ProxySpec(),
		HostCABundlePath:   rc.HostCABundlePath(),
		DataDir:            rc.EmbeddedClusterHomeDirectory(),
		OpenEBSDataDir:     rc.EmbeddedClusterOpenEBSLocalSubDir(),
		K0sDataDir:         rc.EmbeddedClusterK0sSubDir(),
	}); err != nil {
		return fmt.Errorf("install addons: %w", err)
	}

	return nil
}

func runRestoreStepConfirmBackup(ctx context.Context, flags InstallCmdFlags, rc runtimeconfig.RuntimeConfig) (*disasterrecovery.ReplicatedBackup, bool, error) {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return nil, false, fmt.Errorf("unable to create kube client: %w", err)
	}

	k0sCfg, err := getK0sConfigFromDisk()
	if err != nil {
		return nil, false, fmt.Errorf("unable to get k0s config from disk: %w", err)
	}

	logrus.Debugf("waiting for backups to become available")
	backups, err := waitForBackups(ctx, os.Stdout, kcli, k0sCfg, rc, flags.isAirgap)
	if err != nil {
		return nil, false, err
	}

	logrus.Debugf("picking backup to restore")
	backupToRestore := pickBackupToRestore(backups)
	logrus.Debugf("backup to restore: %s", backupToRestore.GetName())

	logrus.Info("")
	completionTimestamp := backupToRestore.GetCompletionTimestamp().Format("2006-01-02 15:04:05 UTC")
	shouldRestore, err := prompts.New().Confirm(fmt.Sprintf("Restore from backup %q (%s)?", backupToRestore.GetName(), completionTimestamp), true)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get confirmation: %w", err)
	}
	logrus.Info("")
	if !shouldRestore {
		logrus.Infof("Aborting restore...")
		return nil, false, nil
	}

	return backupToRestore, true, nil
}

func runRestoreECInstall(ctx context.Context, rc runtimeconfig.RuntimeConfig, backupToRestore *disasterrecovery.ReplicatedBackup) error {
	logrus.Debugf("restoring embedded cluster installation from backup %q", backupToRestore.GetName())
	if err := restoreFromReplicatedBackup(ctx, *backupToRestore, disasterRecoveryComponentECInstall, true); err != nil {
		return fmt.Errorf("unable to restore from backup: %w", err)
	}

	logrus.Debugf("updating installation from backup %q", backupToRestore.GetName())
	if err := restoreReconcileInstallationFromRuntimeConfig(ctx, rc); err != nil {
		return fmt.Errorf("unable to update installation from backup: %w", err)
	}

	logrus.Debugf("updating local artifact mirror service from backup %q", backupToRestore.GetName())
	if err := updateLocalArtifactMirrorService(rc); err != nil {
		return fmt.Errorf("unable to update local artifact mirror service from backup: %w", err)
	}

	return nil
}

func runRestoreAdminConsole(ctx context.Context, backupToRestore *disasterrecovery.ReplicatedBackup) error {
	logrus.Debugf("restoring admin console from backup %q", backupToRestore.GetName())
	if err := restoreFromReplicatedBackup(ctx, *backupToRestore, disasterRecoveryComponentAdminConsole, true); err != nil {
		return err
	}

	return nil
}

func runRestoreWaitForNodes(ctx context.Context, flags InstallCmdFlags, rc runtimeconfig.RuntimeConfig, backupToRestore *disasterrecovery.ReplicatedBackup) error {
	logrus.Debugf("checking if backup is high availability")
	highAvailability, err := isHighAvailabilityReplicatedBackup(*backupToRestore)
	if err != nil {
		return err
	}

	logrus.Debugf("waiting for additional nodes to be added")

	if err := waitForAdditionalNodes(ctx, highAvailability, flags.networkInterface, rc); err != nil {
		return err
	}

	return nil
}

func runRestoreEnableAdminConsoleHA(ctx context.Context, flags InstallCmdFlags, rc runtimeconfig.RuntimeConfig, backupToRestore *disasterrecovery.ReplicatedBackup) error {
	highAvailability, err := isHighAvailabilityReplicatedBackup(*backupToRestore)
	if err != nil {
		return err
	} else if !highAvailability {
		return nil
	}

	loading := spinner.Start()
	defer loading.Close()

	loading.Infof("Enabling high availability for the Admin Console")

	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	mcli, err := kubeutils.MetadataClient()
	if err != nil {
		return fmt.Errorf("unable to create metadata client: %w", err)
	}

	in, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		return fmt.Errorf("get latest installation: %w", err)
	}

	airgapChartsPath := ""
	if flags.isAirgap {
		airgapChartsPath = rc.EmbeddedClusterChartsSubDir()
	}

	euCfg, err := helpers.ParseEndUserConfig(flags.overrides)
	if err != nil {
		return fmt.Errorf("parse end user config: %w", err)
	}
	var euCfgSpec *ecv1beta1.ConfigSpec
	if euCfg != nil {
		euCfgSpec = &euCfg.Spec
	}

	hcli, err := helm.NewClient(helm.HelmOptions{
		KubeConfig: rc.PathToKubeConfig(),
		K0sVersion: versions.K0sVersion,
		AirgapPath: airgapChartsPath,
	})
	if err != nil {
		return fmt.Errorf("create helm client: %w", err)
	}
	defer hcli.Close()

	addOns := addons.New(
		addons.WithLogFunc(logrus.Debugf),
		addons.WithKubernetesClient(kcli),
		addons.WithMetadataClient(mcli),
		addons.WithHelmClient(hcli),
		addons.WithDomains(getDomains()),
	)

	opts := addons.EnableHAOptions{
		AdminConsolePort:   rc.AdminConsolePort(),
		IsAirgap:           in.Spec.AirGap,
		IsMultiNodeEnabled: in.Spec.LicenseInfo != nil && in.Spec.LicenseInfo.IsMultiNodeEnabled,
		EmbeddedConfigSpec: in.Spec.Config,
		EndUserConfigSpec:  euCfgSpec,
		ProxySpec:          rc.ProxySpec(),
		HostCABundlePath:   rc.HostCABundlePath(),
		DataDir:            rc.EmbeddedClusterHomeDirectory(),
		K0sDataDir:         rc.EmbeddedClusterK0sSubDir(),
		SeaweedFSDataDir:   rc.EmbeddedClusterSeaweedFSSubDir(),
		ServiceCIDR:        rc.ServiceCIDR(),
	}

	err = addOns.EnableAdminConsoleHA(ctx, opts)
	if err != nil {
		return err
	}

	loading.Infof("High availability enabled for the Admin Console!")

	return nil
}

func runRestoreSeaweedFS(ctx context.Context, flags InstallCmdFlags, backupToRestore *disasterrecovery.ReplicatedBackup) error {
	highAvailability, err := isHighAvailabilityReplicatedBackup(*backupToRestore)
	if err != nil {
		return err
	} else if !flags.isAirgap || !highAvailability {
		// only restore seaweedfs in case of high availability and airgap
		return nil
	}

	logrus.Debugf("restoring seaweedfs from backup %q", backupToRestore.GetName())
	if err := restoreFromReplicatedBackup(ctx, *backupToRestore, disasterRecoveryComponentSeaweedFS, true); err != nil {
		return err
	}

	return nil
}

func runRestoreRegistry(ctx context.Context, flags InstallCmdFlags, backupToRestore *disasterrecovery.ReplicatedBackup) error {
	// only restore registry in case of airgap
	if !flags.isAirgap {
		return nil
	}

	logrus.Debugf("restoring embedded cluster registry from backup %q", backupToRestore.GetName())
	if err := restoreFromReplicatedBackup(ctx, *backupToRestore, disasterRecoveryComponentRegistry, true); err != nil {
		return err
	}

	registryAddress, ok := backupToRestore.GetAnnotation("kots.io/embedded-registry")
	if !ok {
		return fmt.Errorf("unable to read registry address from backup")
	}

	if err := hostutils.AddInsecureRegistry(registryAddress); err != nil {
		return fmt.Errorf("failed to add insecure registry: %w", err)
	}

	return nil
}

func runRestoreECO(ctx context.Context, backupToRestore *disasterrecovery.ReplicatedBackup) error {
	logrus.Debugf("restoring embedded cluster operator from backup %q", backupToRestore.GetName())
	if err := restoreFromReplicatedBackup(ctx, *backupToRestore, disasterRecoveryComponentECO, true); err != nil {
		return err
	}

	return nil
}

func runRestoreExtensions(ctx context.Context, flags InstallCmdFlags, rc runtimeconfig.RuntimeConfig) error {
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

	logrus.Debugf("installing extensions")
	if err := installExtensions(ctx, hcli); err != nil {
		return fmt.Errorf("unable to install extensions: %w", err)
	}

	return nil
}

func runRestoreApp(ctx context.Context, backupToRestore *disasterrecovery.ReplicatedBackup) error {
	logrus.Debugf("setting installation status to installed")
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("create kube client: %w", err)
	}

	in, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		return fmt.Errorf("get latest installation: %w", err)
	}

	err = kubeutils.SetInstallationState(ctx, kcli, in, ecv1beta1.InstallationStateInstalled, "Installed")
	if err != nil {
		return fmt.Errorf("update installation status: %w", err)
	}

	logrus.Debugf("restoring app from backup %q", backupToRestore.GetName())
	if err := restoreFromReplicatedBackup(ctx, *backupToRestore, disasterRecoveryComponentApp, true); err != nil {
		return err
	}

	logrus.Debugf("resetting restore state")
	if err := resetECRestoreState(ctx); err != nil {
		return fmt.Errorf("unable to reset restore state: %w", err)
	}

	return nil
}

// addS3Flags adds the s3 flags to the restore command. These flags are used only for ease of
// development and are marked as hidden for now.
func addS3Flags(cmd *cobra.Command, store *s3BackupStore) {
	cmd.Flags().StringVar(&store.endpoint, "s3-endpoint", "", "S3 endpoint")
	if err := cmd.Flags().MarkHidden("s3-endpoint"); err != nil {
		panic(err)
	}
	cmd.Flags().StringVar(&store.region, "s3-region", "", "S3 region")
	if err := cmd.Flags().MarkHidden("s3-region"); err != nil {
		panic(err)
	}
	cmd.Flags().StringVar(&store.bucket, "s3-bucket", "", "S3 bucket")
	if err := cmd.Flags().MarkHidden("s3-bucket"); err != nil {
		panic(err)
	}
	cmd.Flags().StringVar(&store.prefix, "s3-prefix", "", "S3 prefix")
	if err := cmd.Flags().MarkHidden("s3-prefix"); err != nil {
		panic(err)
	}
	cmd.Flags().StringVar(&store.accessKeyID, "s3-access-key-id", "", "S3 access key ID")
	if err := cmd.Flags().MarkHidden("s3-access-key-id"); err != nil {
		panic(err)
	}
	cmd.Flags().StringVar(&store.secretAccessKey, "s3-secret-access-key", "", "S3 secret access key")
	if err := cmd.Flags().MarkHidden("s3-secret-access-key"); err != nil {
		panic(err)
	}
}

// getECRestoreState returns the current restore state.
func getECRestoreState(ctx context.Context) ecRestoreState {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return ecRestoreStateNew
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: constants.EmbeddedClusterNamespace,
			Name:      constants.EcRestoreStateCMName,
		},
	}

	if err := kcli.Get(ctx, types.NamespacedName{Namespace: cm.Namespace, Name: cm.Name}, cm); err != nil {
		return ecRestoreStateNew
	}

	state, ok := cm.Data["state"]
	if !ok {
		return ecRestoreStateNew
	}

	for _, s := range ecRestoreStates {
		if s == ecRestoreState(state) {
			return s
		}
	}

	return ecRestoreStateNew
}

// setECRestoreState sets the current restore state.
func setECRestoreState(ctx context.Context, state ecRestoreState, backupName string) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: constants.EmbeddedClusterNamespace,
		},
	}

	if err := kcli.Create(ctx, ns); err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("unable to create namespace: %w", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: constants.EmbeddedClusterNamespace,
			Name:      constants.EcRestoreStateCMName,
		},
		Data: map[string]string{
			"state": string(state),
		},
	}

	if backupName != "" {
		cm.Data["backup-name"] = backupName
	}

	err = kcli.Create(ctx, cm)
	if k8serrors.IsAlreadyExists(err) {
		if err := kcli.Update(ctx, cm); err != nil {
			return fmt.Errorf("unable to update config map: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("unable to create config map: %w", err)
	}

	return nil
}

// resetECRestoreState resets the restore state.
func resetECRestoreState(ctx context.Context) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: constants.EmbeddedClusterNamespace,
			Name:      constants.EcRestoreStateCMName,
		},
	}

	if err := kcli.Delete(ctx, cm); err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("unable to delete config map: %w", err)
	}

	return nil
}

// getBackupFromRestoreState gets the backup defined in the restore state.
// If no backup is defined in the restore state, it returns nil.
// It returns an error if a backup is defined in the restore state but:
//   - is not found by Velero anymore.
//   - is not restorable by the current binary.
func getBackupFromRestoreState(ctx context.Context, isAirgap bool, rc runtimeconfig.RuntimeConfig) (*disasterrecovery.ReplicatedBackup, error) {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return nil, fmt.Errorf("unable to create kube client: %w", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: constants.EmbeddedClusterNamespace,
			Name:      constants.EcRestoreStateCMName,
		},
	}

	if err := kcli.Get(ctx, types.NamespacedName{Namespace: cm.Namespace, Name: cm.Name}, cm); err != nil {
		return nil, fmt.Errorf("unable to get restore state: %w", err)
	}

	backupName, ok := cm.Data["backup-name"]
	if !ok || backupName == "" {
		return nil, nil
	}

	backup, err := disasterrecovery.GetReplicatedBackup(ctx, kcli, constants.VeleroNamespace, backupName)
	if err != nil {
		return nil, err
	}

	rel := release.GetChannelRelease()

	if rel == nil {
		return nil, fmt.Errorf("no release found in binary")
	}

	k0sCfg, err := getK0sConfigFromDisk()
	if err != nil {
		return nil, fmt.Errorf("unable to get k0s config from disk: %w", err)
	}

	if restorable, reason := isReplicatedBackupRestorable(backup, rel, isAirgap, k0sCfg, rc); !restorable {
		return nil, fmt.Errorf("backup %q %s", backup.GetName(), reason)
	}

	return &backup, nil
}

// s3BackupStoreHasData checks if the store already has data from flags.
func s3BackupStoreHasData(store *s3BackupStore) bool {
	// store.prefix not required
	return store.endpoint != "" && store.region != "" && store.bucket != "" && store.accessKeyID != "" && store.secretAccessKey != ""
}

// promptForS3BackupStore prompts the user for S3 backup store configuration.
func promptForS3BackupStore(store *s3BackupStore) error {
	for {
		input, err := prompts.New().Input("S3 endpoint:", store.endpoint, true)
		if err != nil {
			return fmt.Errorf("failed to get input: %w", err)
		}
		store.endpoint = strings.TrimSpace(input)
		if strings.HasPrefix(store.endpoint, "http://") || strings.HasPrefix(store.endpoint, "https://") {
			break
		}
		logrus.Info("Endpoint must start with http:// or https://")
	}

	input, err := prompts.New().Input("Region:", store.region, true)
	if err != nil {
		return fmt.Errorf("failed to get input: %w", err)
	}
	store.region = strings.TrimSpace(input)

	input, err = prompts.New().Input("Bucket:", store.bucket, true)
	if err != nil {
		return fmt.Errorf("failed to get input: %w", err)
	}
	store.bucket = strings.TrimSpace(input)

	input, err = prompts.New().Input("Prefix (press Enter to skip):", store.prefix, false)
	if err != nil {
		return fmt.Errorf("failed to get input: %w", err)
	}
	store.prefix = strings.TrimSpace(input)

	input, err = prompts.New().Input("Access key ID:", store.accessKeyID, true)
	if err != nil {
		return fmt.Errorf("failed to get input: %w", err)
	}
	store.accessKeyID = strings.TrimSpace(input)

	password, err := prompts.New().Password("Secret access key:")
	if err != nil {
		return fmt.Errorf("failed to get password: %w", err)
	}
	store.secretAccessKey = strings.TrimSpace(password)

	logrus.Info("")
	return nil
}

// validateS3BackupStore validates the S3 backup store configuration.
// It tries to list objects in the bucket and prefix to ensure that the bucket exists and has backups.
func validateS3BackupStore(s *s3BackupStore) error {
	u, err := url.Parse(s.endpoint)
	if err != nil {
		return fmt.Errorf("parse endpoint: %v", err)
	}

	isAWS := strings.HasSuffix(u.Hostname(), ".amazonaws.com")
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String(s.region),
		Endpoint:         aws.String(s.endpoint),
		Credentials:      credentials.NewStaticCredentials(s.accessKeyID, s.secretAccessKey, ""),
		S3ForcePathStyle: aws.Bool(!isAWS),
	})
	if err != nil {
		return fmt.Errorf("create s3 session: %v", err)
	}

	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(s.bucket),
		Delimiter: aws.String("/"),
		Prefix:    aws.String(fmt.Sprintf("%s/", filepath.Join(s.prefix, "backups"))),
	}
	svc := s3.New(sess)
	result, err := svc.ListObjectsV2(input)
	if err != nil {
		return fmt.Errorf("list objects: %v", err)
	}

	if len(result.CommonPrefixes) == 0 {
		return fmt.Errorf("no backups found in %s", filepath.Join(s.bucket, s.prefix))
	}

	return nil
}

func isReplicatedBackupRestorable(backup disasterrecovery.ReplicatedBackup, rel *release.ChannelRelease, isAirgap bool, k0sCfg *k0sv1beta1.ClusterConfig, rc runtimeconfig.RuntimeConfig) (bool, string) {
	if backup.GetExpectedBackupCount() != len(backup) {
		return false, fmt.Sprintf("has a different number of backups (%d) than the expected number (%d)", len(backup), backup.GetExpectedBackupCount())
	}

	improvedDR := usesImprovedDR()

	appBackup := backup.GetAppBackup()
	if appBackup == nil {
		return false, "missing app backup"
	}
	if disasterrecovery.GetInstanceBackupType(*appBackup) == disasterrecovery.InstanceBackupTypeApp && !improvedDR {
		return false, "app backup found but improved dr is not enabled"
	} else if disasterrecovery.GetInstanceBackupType(*appBackup) == disasterrecovery.InstanceBackupTypeLegacy && improvedDR {
		return false, "legacy backup found but improved dr is enabled"
	}

	for _, b := range backup {
		restorable, reason := isBackupRestorable(&b, rel, isAirgap, k0sCfg, rc)
		if !restorable {
			return false, reason
		}
	}
	return true, ""
}

func isBackupRestorable(backup *velerov1.Backup, rel *release.ChannelRelease, isAirgap bool, k0sCfg *k0sv1beta1.ClusterConfig, rc runtimeconfig.RuntimeConfig) (bool, string) {
	if backup.Annotations[disasterrecovery.BackupIsECAnnotation] != "true" {
		return false, "is not an embedded cluster backup"
	}

	if v := strings.TrimPrefix(backup.Annotations["kots.io/embedded-cluster-version"], "v"); v != strings.TrimPrefix(versions.Version, "v") {
		return false, fmt.Sprintf("has a different embedded cluster version (%q) than the current version (%q)", v, versions.Version)
	}

	if backup.Status.Phase != velerov1.BackupPhaseCompleted {
		return false, fmt.Sprintf("has a status of %q", backup.Status.Phase)
	}

	if _, ok := backup.Annotations["kots.io/apps-versions"]; !ok {
		return false, "is missing the kots.io/apps-versions annotation"
	}

	appsVersions := map[string]string{}
	if err := json.Unmarshal([]byte(backup.Annotations["kots.io/apps-versions"]), &appsVersions); err != nil {
		return false, "unable to json parse kots.io/apps-versions annotation"
	}

	if len(appsVersions) == 0 {
		return false, "has no applications"
	}

	if len(appsVersions) > 1 {
		return false, "has more than one application"
	}

	if _, ok := appsVersions[rel.AppSlug]; !ok {
		return false, fmt.Sprintf("does not contain the %q application", rel.AppSlug)
	}

	if versionLabel := appsVersions[rel.AppSlug]; versionLabel != rel.VersionLabel {
		return false, fmt.Sprintf("has a different app version (%q) than the current version (%q)", versionLabel, rel.VersionLabel)
	}

	if _, ok := backup.Annotations["kots.io/is-airgap"]; !ok {
		return false, "is missing the kots.io/is-airgap annotation"
	}

	airgapLabelValue := backup.Annotations["kots.io/is-airgap"]
	if isAirgap {
		if airgapLabelValue != "true" {
			return false, "is not an airgap backup, but the restore is configured to be airgap"
		}
	} else {
		if airgapLabelValue != "false" {
			return false, "is an airgap backup, but the restore is configured to be online"
		}
	}

	if _, ok := backup.Annotations["kots.io/embedded-cluster-pod-cidr"]; ok {
		// kots.io/embedded-cluster-pod-cidr and kots.io/embedded-cluster-service-cidr should both exist if one does
		podCIDR := backup.Annotations["kots.io/embedded-cluster-pod-cidr"]
		serviceCIDR := backup.Annotations["kots.io/embedded-cluster-service-cidr"]

		if k0sCfg != nil && k0sCfg.Spec != nil && k0sCfg.Spec.Network != nil {
			if k0sCfg.Spec.Network.PodCIDR != "" || k0sCfg.Spec.Network.ServiceCIDR != "" {
				if podCIDR != k0sCfg.Spec.Network.PodCIDR || serviceCIDR != k0sCfg.Spec.Network.ServiceCIDR {
					if adjacent, supernet, _ := netutils.NetworksAreAdjacentAndSameSize(podCIDR, serviceCIDR); adjacent {
						return false, fmt.Sprintf("has a different network configuration than the current cluster. Please rerun with '--cidr %s'.", supernet)
					}
					return false, fmt.Sprintf("has a different network configuration than the current cluster. Please rerun with '--pod-cidr %s --service-cidr %s'.", podCIDR, serviceCIDR)
				}
			}
		}
	}

	if v := backup.Annotations["kots.io/embedded-cluster-data-dir"]; v != "" && v != rc.EmbeddedClusterHomeDirectory() {
		return false, fmt.Sprintf("has a different data directory than the current cluster. Please rerun with '--data-dir %s'.", v)
	}

	return true, ""
}

func isHighAvailabilityReplicatedBackup(backup disasterrecovery.ReplicatedBackup) (bool, error) {
	ha, ok := backup.GetAnnotation("kots.io/embedded-cluster-is-ha")
	if !ok {
		return false, fmt.Errorf("high availability annotation not found in backup")
	}

	return ha == "true", nil
}

// waitForBackups waits for backups to become available.
// It returns a list of restorable backups, or an error if none are found.
func waitForBackups(ctx context.Context, out io.Writer, kcli client.Client, k0sCfg *k0sv1beta1.ClusterConfig, rc runtimeconfig.RuntimeConfig, isAirgap bool) ([]disasterrecovery.ReplicatedBackup, error) {
	loading := spinner.Start(spinner.WithWriter(func(format string, a ...any) (int, error) {
		return fmt.Fprintf(out, format, a...)
	}))

	defer loading.Close()
	loading.Infof("Waiting for backups to become available")

	rel := release.GetChannelRelease()

	if rel == nil {
		return nil, fmt.Errorf("no release found in binary")
	}

	replicatedBackups, err := listBackupsWithTimeout(ctx, kcli, 30, 5*time.Second)
	if err != nil {
		return nil, err
	}

	validBackups := []disasterrecovery.ReplicatedBackup{}
	invalidBackups := []disasterrecovery.ReplicatedBackup{}
	invalidReasons := []string{}

	for _, backup := range replicatedBackups {
		restorable, reason := isReplicatedBackupRestorable(backup, rel, isAirgap, k0sCfg, rc)
		if restorable {
			validBackups = append(validBackups, backup)
		} else {
			invalidBackups = append(invalidBackups, backup)
			invalidReasons = append(invalidReasons, reason)
		}
	}

	if len(validBackups) == 0 {
		return nil, &invalidBackupsError{
			invalidBackups: invalidBackups,
			invalidReasons: invalidReasons,
		}
	}

	logrus.Debugf("Found %d restorable backup(s)", len(validBackups))
	if len(validBackups) == 1 {
		loading.Infof("Found 1 restorable backup!")
	} else {
		loading.Infof("Found %d restorable backups!", len(validBackups))
	}
	return validBackups, nil
}

func listBackupsWithTimeout(ctx context.Context, kcli client.Client, tries int, sleep time.Duration) ([]disasterrecovery.ReplicatedBackup, error) {
	if tries == 0 {
		tries = 1
	}
	for i := 0; i < tries; i++ {
		backups, err := disasterrecovery.ListReplicatedBackups(ctx, kcli)
		if err != nil {
			return nil, fmt.Errorf("unable to list backups: %w", err)
		}
		if len(backups) > 0 {
			logrus.Debugf("Found %d backups", len(backups))
			return backups, nil
		}

		logrus.Debugf("No backups found yet...")
		time.Sleep(sleep)
	}

	return nil, fmt.Errorf("timed out waiting for backups to become available")
}

// pickBackupToRestore picks a backup to restore from a list of backups.
// Currently, it picks the latest backup.
func pickBackupToRestore(backups []disasterrecovery.ReplicatedBackup) *disasterrecovery.ReplicatedBackup {
	var latestBackup *disasterrecovery.ReplicatedBackup
	for _, b := range backups {
		if latestBackup == nil {
			latestBackup = &b
			continue
		}
		// Should this use Status.StartTimestamp instead of Status.CompletionTimestamp?
		if b.GetCompletionTimestamp().After(latestBackup.GetCompletionTimestamp().Time) {
			latestBackup = &b
		}
	}
	return latestBackup
}

// getK0sConfigFromDisk reads and returns the k0s config from disk.
func getK0sConfigFromDisk() (*k0sv1beta1.ClusterConfig, error) {
	cfgBytes, err := os.ReadFile(runtimeconfig.K0sConfigPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read k0s config file: %w", err)
	}

	cfg := &k0sv1beta1.ClusterConfig{}
	if err := k8syaml.Unmarshal(cfgBytes, cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal k0s config: %w", err)
	}

	return cfg, nil
}

// waitForVeleroRestoreCompleted waits for a Velero restore to complete.
func waitForVeleroRestoreCompleted(ctx context.Context, restoreName string) (*velerov1.Restore, error) {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return nil, fmt.Errorf("unable to create kube client: %w", err)
	}

	for {
		restore := velerov1.Restore{}
		err = kcli.Get(ctx, types.NamespacedName{Name: restoreName, Namespace: constants.VeleroNamespace}, &restore)
		if err != nil {
			return nil, fmt.Errorf("unable to get restore: %w", err)
		}

		switch restore.Status.Phase {
		case velerov1.RestorePhaseCompleted:
			return &restore, nil
		case velerov1.RestorePhaseFailed:
			return &restore, fmt.Errorf("restore failed")
		case velerov1.RestorePhasePartiallyFailed:
			return &restore, fmt.Errorf("restore partially failed")
		default:
			// in progress
		}

		time.Sleep(time.Second)
	}
}

// getRegistryIPFromBackup gets the registry service IP from a backup.
// It returns an empty string if the backup is not airgapped.
func getRegistryIPFromBackup(backup *velerov1.Backup) (string, error) {
	isAirgap, ok := backup.Annotations["kots.io/is-airgap"]
	if !ok {
		return "", fmt.Errorf("unable to get airgap status from backup")
	}

	if isAirgap != "true" {
		return "", nil
	}

	registryServiceHost, ok := backup.Annotations["kots.io/embedded-registry"]
	if !ok {
		return "", fmt.Errorf("embedded registry service IP annotation not found in backup")
	}

	return strings.Split(registryServiceHost, ":")[0], nil
}

// getSeaweedFSS3ServiceIPFromBackup gets the seaweedfs s3 service IP from a backup.
// It returns an empty string if the backup is not airgapped or not high availability.
func getSeaweedFSS3ServiceIPFromBackup(backup *velerov1.Backup) (string, error) {
	isAirgap, ok := backup.Annotations["kots.io/is-airgap"]
	if !ok {
		return "", fmt.Errorf("unable to get airgap status from backup")
	}

	if isAirgap != "true" {
		return "", nil
	}

	highAvailability, err := isHighAvailabilityBackup(backup)
	if err != nil {
		return "", fmt.Errorf("unable to check high availability status: %w", err)
	}

	if !highAvailability {
		return "", nil
	}

	swIP, ok := backup.Annotations["kots.io/embedded-cluster-seaweedfs-s3-ip"]
	if !ok {
		return "", fmt.Errorf("unable to get seaweedfs s3 service IP from backup")
	}

	return swIP, nil
}

func isHighAvailabilityBackup(backup *velerov1.Backup) (bool, error) {
	ha, ok := backup.Annotations["kots.io/embedded-cluster-is-ha"]
	if !ok {
		return false, fmt.Errorf("high availability annotation not found in backup")
	}

	return ha == "true", nil
}

// ensureRestoreResourceModifiers ensures the necessary restore resource modifiers.
// Velero resource modifiers are used to modify the resources during a Velero restore by specifying json patches.
// The json patches are applied to the resources before they are restored.
// The json patches are specified in a configmap and the configmap is referenced in the restore object.
func ensureRestoreResourceModifiers(ctx context.Context, backup *velerov1.Backup) error {
	registryServiceIP, err := getRegistryIPFromBackup(backup)
	if err != nil {
		return fmt.Errorf("unable to get registry service IP from backup: %w", err)
	}

	seaweedFSS3ServiceIP, err := getSeaweedFSS3ServiceIPFromBackup(backup)
	if err != nil {
		return fmt.Errorf("unable to get seaweedfs s3 service IP from backup: %w", err)
	}

	modifiersYAML := strings.Replace(resourceModifiersYAML, "__REGISTRY_SERVICE_IP__", registryServiceIP, 1)
	modifiersYAML = strings.Replace(modifiersYAML, "__SEAWEEDFS_S3_SERVICE_IP__", seaweedFSS3ServiceIP, 1)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: constants.VeleroNamespace,
			Name:      resourceModifiersCMName,
		},
		Data: map[string]string{
			"resource-modifiers.yaml": modifiersYAML,
		},
	}
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	if err := kcli.Create(ctx, cm); err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("unable to create config map: %w", err)
	}

	return nil
}

// waitForDRComponent waits for a disaster recovery component to be restored.
func waitForDRComponent(ctx context.Context, drComponent disasterRecoveryComponent, restoreName string, isV2 bool) error {
	loading := spinner.Start()
	defer loading.Close()

	switch drComponent {
	case disasterRecoveryComponentECInstall:
		loading.Infof("Restoring cluster state")
	case disasterRecoveryComponentAdminConsole:
		loading.Infof("Restoring the Admin Console")
	case disasterRecoveryComponentSeaweedFS:
		loading.Infof("Restoring registry data")
	case disasterRecoveryComponentRegistry:
		loading.Infof("Restoring registry")
	case disasterRecoveryComponentECO:
		loading.Infof("Restoring embedded cluster operator")
	case disasterRecoveryComponentApp:
		loading.Infof("Restoring application")
	}

	// wait for velero restore to complete
	restore, err := waitForVeleroRestoreCompleted(ctx, restoreName)
	if err != nil {
		if restore != nil {
			return fmt.Errorf("restore failed with %d errors and %d warnings: %w", restore.Status.Errors, restore.Status.Warnings, err)
		}

		return fmt.Errorf("unable to wait for velero restore to complete: %w", err)
	}

	if drComponent == disasterRecoveryComponentAdminConsole {
		// wait for admin console to be ready
		kcli, err := kubeutils.KubeClient()
		if err != nil {
			return fmt.Errorf("unable to create kube client: %w", err)
		}

		if err := restoreWaitForAdminConsoleReady(ctx, kcli, constants.KotsadmNamespace, loading); err != nil {
			return fmt.Errorf("unable to wait for admin console: %w", err)
		}
	} else if drComponent == disasterRecoveryComponentSeaweedFS {
		// wait for seaweedfs to be ready
		kcli, err := kubeutils.KubeClient()
		if err != nil {
			return fmt.Errorf("unable to create kube client: %w", err)
		}

		if err := restoreWaitForSeaweedfsReady(ctx, kcli, constants.SeaweedFSNamespace, nil); err != nil {
			return fmt.Errorf("unable to wait for seaweedfs to be ready: %w", err)
		}
	} else if drComponent == disasterRecoveryComponentRegistry {
		// wait for registry to be ready
		kcli, err := kubeutils.KubeClient()
		if err != nil {
			return fmt.Errorf("unable to create kube client: %w", err)
		}

		if err := kubeutils.WaitForDeployment(ctx, kcli, constants.RegistryNamespace, "registry", nil); err != nil {
			return fmt.Errorf("unable to wait for registry to be ready: %w", err)
		}
	} else if drComponent == disasterRecoveryComponentECO {
		// wait for embedded cluster operator to reconcile the installation
		kcli, err := kubeutils.KubeClient()
		if err != nil {
			return fmt.Errorf("unable to create kube client: %w", err)
		}

		if isV2 {
			if err := kubeutils.WaitForDeployment(ctx, kcli, constants.EmbeddedClusterNamespace, "embedded-cluster-operator", nil); err != nil {
				return fmt.Errorf("unable to wait for embedded cluster operator to be ready: %w", err)
			}
		} else {
			if err := kubeutils.WaitForInstallation(ctx, kcli, loading); err != nil {
				return fmt.Errorf("unable to wait for installation to be ready: %w", err)
			}
		}
	}

	switch drComponent {
	case disasterRecoveryComponentECInstall:
		loading.Infof("Cluster state restored!")
	case disasterRecoveryComponentAdminConsole:
		loading.Infof("Admin Console restored!")
	case disasterRecoveryComponentSeaweedFS:
		loading.Infof("Registry data restored!")
	case disasterRecoveryComponentRegistry:
		loading.Infof("Registry restored!")
	case disasterRecoveryComponentECO:
		loading.Infof("Embedded cluster operator restored!")
	case disasterRecoveryComponentApp:
		loading.Infof("Application restored!")
	}

	return nil
}

// restoreFromReplicatedBackup restores a disaster recovery component from a backup.
func restoreFromReplicatedBackup(ctx context.Context, backup disasterrecovery.ReplicatedBackup, drComponent disasterRecoveryComponent, isV2 bool) error {
	if drComponent == disasterRecoveryComponentApp {
		isImprovedDR := usesImprovedDR()
		// If the app is using improved dr, we need to restore the app using the spec provided by
		// the vendor. Otherwise, we use the "replicated.com/disaster-recovery" label to discover
		// the application resources in the cluster.
		if isImprovedDR {
			b := backup.GetAppBackup()
			if b == nil {
				return fmt.Errorf("unable to find app backup")
			}
			r, err := backup.GetRestore()
			if err != nil {
				return fmt.Errorf("failed to get restore resource from backup: %w", err)
			}
			err = restoreAppFromBackup(ctx, b, r, isV2)
			if err != nil {
				return fmt.Errorf("failed to restore app from backup: %w", err)
			}
			return nil
		}
	}
	b := backup.GetInfraBackup()
	if b == nil {
		return fmt.Errorf("unable to find infra backup")
	}
	err := restoreFromBackup(ctx, b, drComponent, isV2)
	if err != nil {
		return fmt.Errorf("failed to restore infra from backup: %w", err)
	}
	return nil
}

func usesImprovedDR() bool {
	backup := release.GetVeleroBackup()
	restore := release.GetVeleroRestore()
	return backup != nil && restore != nil
}

// restoreAppFromBackup will either restore using the spec provided by the vendor as part of the
// improved dr support.
func restoreAppFromBackup(ctx context.Context, backup *velerov1.Backup, restore *velerov1.Restore, isV2 bool) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	restoreName := fmt.Sprintf("%s.restore", backup.Name)

	// check if a restore object already exists
	rest := velerov1.Restore{}
	err = kcli.Get(ctx, types.NamespacedName{Name: restoreName, Namespace: constants.VeleroNamespace}, &rest)
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("unable to get restore: %w", err)
	}

	// create a new restore object if it doesn't exist
	if k8serrors.IsNotFound(err) {
		restore.Namespace = constants.VeleroNamespace
		restore.Name = restoreName
		if restore.Annotations == nil {
			restore.Annotations = map[string]string{}
		}
		restore.Annotations[disasterrecovery.BackupIsECAnnotation] = "true"

		ensureImprovedDrMetadata(restore, backup)

		restore.Spec.BackupName = backup.Name

		logrus.Debugf("creating restore %s", restoreName)

		err = kcli.Create(ctx, restore)
		if err != nil {
			return fmt.Errorf("unable to create restore: %w", err)
		}
	}

	// wait for restore to complete
	return waitForDRComponent(ctx, disasterRecoveryComponentApp, restoreName, isV2)
}

// restoreFromBackup will use the "replicated.com/disaster-recovery" label value provided to create
// a velero restore object which will restore one set of resources to the cluster.
func restoreFromBackup(ctx context.Context, backup *velerov1.Backup, drComponent disasterRecoveryComponent, isV2 bool) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	restoreName := fmt.Sprintf("%s.%s", backup.Name, string(drComponent))

	// check if a restore object already exists
	rest := velerov1.Restore{}
	err = kcli.Get(ctx, types.NamespacedName{Name: restoreName, Namespace: constants.VeleroNamespace}, &rest)
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("unable to get restore: %w", err)
	}

	// create a new restore object if it doesn't exist
	if k8serrors.IsNotFound(err) {
		restoreLabels := map[string]string{}
		switch drComponent {
		case disasterRecoveryComponentAdminConsole, disasterRecoveryComponentECO:
			restoreLabels["replicated.com/disaster-recovery-chart"] = string(drComponent)
		case disasterRecoveryComponentECInstall, disasterRecoveryComponentApp:
			restoreLabels["replicated.com/disaster-recovery"] = string(drComponent)
		case disasterRecoveryComponentSeaweedFS:
			restoreLabels["app.kubernetes.io/name"] = "seaweedfs"
		case disasterRecoveryComponentRegistry:
			restoreLabels["app"] = "docker-registry"
		default:
			return fmt.Errorf("unknown disaster recovery component: %q", drComponent)
		}

		restore := &velerov1.Restore{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: constants.VeleroNamespace,
				Name:      restoreName,
				Annotations: map[string]string{
					disasterrecovery.BackupIsECAnnotation: "true",
				},
				Labels: map[string]string{},
			},
			Spec: velerov1.RestoreSpec{
				BackupName: backup.Name,
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: restoreLabels,
				},
				RestorePVs:              ptr.To(true),
				IncludeClusterResources: ptr.To(true),
				ResourceModifier: &corev1.TypedLocalObjectReference{
					Kind: "ConfigMap",
					Name: resourceModifiersCMName,
				},
			},
		}

		ensureImprovedDrMetadata(restore, backup)

		// ensure restore resource modifiers first
		if err := ensureRestoreResourceModifiers(ctx, backup); err != nil {
			return fmt.Errorf("unable to ensure restore resource modifiers: %w", err)
		}

		logrus.Debugf("creating restore %s", restoreName)

		err = kcli.Create(ctx, restore)
		if err != nil {
			return fmt.Errorf("unable to create restore: %w", err)
		}
	}

	// wait for restore to complete
	return waitForDRComponent(ctx, drComponent, restoreName, isV2)
}

func ensureImprovedDrMetadata(restore *velerov1.Restore, backup *velerov1.Backup) {
	if restore.Labels == nil {
		restore.Labels = map[string]string{}
	}
	if restore.Annotations == nil {
		restore.Annotations = map[string]string{}
	}
	if val, ok := backup.Labels[disasterrecovery.InstanceBackupNameLabel]; ok {
		restore.Labels[disasterrecovery.InstanceBackupNameLabel] = val
	}
	if val, ok := backup.Annotations[disasterrecovery.InstanceBackupTypeAnnotation]; ok {
		restore.Annotations[disasterrecovery.InstanceBackupTypeAnnotation] = val
	}
	if val, ok := backup.Annotations[disasterrecovery.InstanceBackupCountAnnotation]; ok {
		restore.Annotations[disasterrecovery.InstanceBackupCountAnnotation] = val
	}
}

// waitForAdditionalNodes waits for for user to add additional nodes to the cluster.
func waitForAdditionalNodes(ctx context.Context, highAvailability bool, networkInterface string, rc runtimeconfig.RuntimeConfig) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	adminConsoleURL := getAdminConsoleURL("", networkInterface, rc.AdminConsolePort())

	successColor := "\033[32m"
	colorReset := "\033[0m"
	joinNodesMsg := fmt.Sprintf("\nVisit the Admin Console if you need to add nodes to the cluster: %s%s%s\n",
		successColor, adminConsoleURL, colorReset,
	)
	logrus.Info(joinNodesMsg)

	for {
		p, err := prompts.New().Input("Type 'continue' when you are done adding nodes:", "", false)
		if err != nil {
			return fmt.Errorf("failed to get confirmation: %w", err)
		}
		if p != "continue" {
			logrus.Info("Please type 'continue' to proceed")
			continue
		}
		if highAvailability {
			ncps, err := kubeutils.NumOfControlPlaneNodes(ctx, kcli)
			if err != nil {
				return fmt.Errorf("unable to check control plane nodes: %w", err)
			}

			if ncps < 3 {
				logrus.Infof("You are restoring a high-availability cluster, which requires at least 3 controller nodes. You currently have %d. Please add more controller nodes.", ncps)
				continue
			}
		}
		break
	}

	loading := spinner.Start()
	loading.Infof("Waiting for all nodes to be ready")
	if err := kubeutils.WaitForNodes(ctx, kcli); err != nil {
		loading.Close()
		return fmt.Errorf("unable to wait for nodes: %w", err)
	}

	loading.Closef("All nodes are ready!")

	return nil
}

// restoreReconcileInstallationFromRuntimeConfig will update the installation to match the runtime
// config from the original installation.
func restoreReconcileInstallationFromRuntimeConfig(ctx context.Context, rc runtimeconfig.RuntimeConfig) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("create kube client: %w", err)
	}

	in, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		return fmt.Errorf("get latest installation: %w", err)
	}

	if in.Spec.RuntimeConfig == nil {
		in.Spec.RuntimeConfig = &ecv1beta1.RuntimeConfigSpec{}
	}

	err = kubeutils.UpdateInstallation(ctx, kcli, in, func(in *ecv1beta1.Installation) {
		in.Spec.RuntimeConfig.LocalArtifactMirror.Port = rc.LocalArtifactMirrorPort()
	})
	if err != nil {
		return fmt.Errorf("update installation: %w", err)
	}

	err = kubeutils.SetInstallationState(ctx, kcli, in, ecv1beta1.InstallationStateKubernetesInstalled, "Kubernetes installed")
	if err != nil {
		return fmt.Errorf("update installation status: %w", err)
	}

	return nil
}

// overrideRuntimeConfigFromBackup will update the runtime config from the backup. These values may
// be used during the install and set in the Installation object via the
// restoreReconcileInstallationFromRuntimeConfig function.
func overrideRuntimeConfigFromBackup(localArtifactMirrorPort int, backup disasterrecovery.ReplicatedBackup, rc runtimeconfig.RuntimeConfig) error {
	if localArtifactMirrorPort != 0 {
		if val, _ := backup.GetAnnotation("kots.io/embedded-cluster-local-artifact-mirror-port"); val != "" {
			port, err := k8snet.ParsePort(val, false)
			if err != nil {
				return fmt.Errorf("parse local artifact mirror port: %w", err)
			}
			logrus.Debugf("updating local artifact mirror port to %d from backup %q", port, backup.GetName())
			rc.SetLocalArtifactMirrorPort(port)
		}
	}

	return nil
}

// getRuntimeConfigFromInstallation returns the runtime config from the latest installation.
func getRuntimeConfigFromInstallation(ctx context.Context) (*ecv1beta1.RuntimeConfigSpec, error) {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return nil, fmt.Errorf("unable to create kube client: %w", err)
	}

	in, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		return nil, fmt.Errorf("unable to get latest installation: %w", err)
	}

	return in.Spec.RuntimeConfig, nil
}

//go:embed assets/resource-modifiers.yaml
var resourceModifiersYAML string

type s3BackupStore struct {
	endpoint        string
	region          string
	bucket          string
	prefix          string
	accessKeyID     string
	secretAccessKey string
}

type disasterRecoveryComponent string

const (
	disasterRecoveryComponentECInstall    disasterRecoveryComponent = "ec-install"
	disasterRecoveryComponentAdminConsole disasterRecoveryComponent = "admin-console"
	disasterRecoveryComponentSeaweedFS    disasterRecoveryComponent = "seaweedfs"
	disasterRecoveryComponentRegistry     disasterRecoveryComponent = "registry"
	disasterRecoveryComponentECO          disasterRecoveryComponent = "embedded-cluster-operator"
	disasterRecoveryComponentApp          disasterRecoveryComponent = "app"
)

type invalidBackupsError struct {
	invalidBackups []disasterrecovery.ReplicatedBackup
	invalidReasons []string
}

func (e *invalidBackupsError) Error() string {
	reasons := []string{}
	for i, backup := range e.invalidBackups {
		reasons = append(reasons, fmt.Sprintf("%q %s", backup.GetName(), e.invalidReasons[i]))
	}

	if len(e.invalidBackups) == 1 {
		return fmt.Sprintf("\nFound 1 backup, but it is not restorable:\n%s\n", strings.Join(reasons, "\n"))
	}

	return fmt.Sprintf("\nFound %d backups, but none are restorable:\n%s\n", len(e.invalidBackups), strings.Join(reasons, "\n"))
}

// updateLocalArtifactMirrorService updates the port on which the local artifact mirror is served.
func updateLocalArtifactMirrorService(rc runtimeconfig.RuntimeConfig) error {
	if err := hostutils.WriteLocalArtifactMirrorDropInFile(rc); err != nil {
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

func restoreWaitForSeaweedfsReady(ctx context.Context, cli client.Client, ns string, writer *spinner.MessageWriter) error {
	backoff := wait.Backoff{Steps: 60, Duration: 5 * time.Second, Factor: 1.0, Jitter: 0.1}
	var lasterr error
	if err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		var count int
		ready, err := kubeutils.IsStatefulSetReady(ctx, cli, ns, "seaweedfs-filer")
		if err != nil {
			lasterr = fmt.Errorf("check status of seaweedfs-filer: %v", err)
			return false, nil
		}
		if ready {
			count++
		}
		ready, err = kubeutils.IsStatefulSetReady(ctx, cli, ns, "seaweedfs-master")
		if err != nil {
			lasterr = fmt.Errorf("check status of seaweedfs-master: %v", err)
			return false, nil
		}
		if ready {
			count++
		}
		ready, err = kubeutils.IsStatefulSetReady(ctx, cli, ns, "seaweedfs-volume")
		if err != nil {
			lasterr = fmt.Errorf("check status of seaweedfs-volume: %v", err)
			return false, nil
		}
		if ready {
			count++
		}
		if writer != nil {
			writer.Infof("Waiting for SeaweedFS to deploy: %d/3 ready", count)
		}
		return count == 3, nil
	}); err != nil {
		if lasterr == nil {
			lasterr = err
		}
		return lasterr
	}
	return nil
}

func restoreWaitForAdminConsoleReady(ctx context.Context, cli client.Client, ns string, writer *spinner.MessageWriter) error {
	backoff := wait.Backoff{Steps: 60, Duration: 5 * time.Second, Factor: 1.0, Jitter: 0.1}
	var lasterr error
	if err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		var count int
		ready, err := kubeutils.IsDeploymentReady(ctx, cli, ns, "kotsadm")
		if err != nil {
			lasterr = fmt.Errorf("check status of kotsadm: %v", err)
			return false, nil
		}
		if ready {
			count++
		}
		ready, err = kubeutils.IsStatefulSetReady(ctx, cli, ns, "kotsadm-rqlite")
		if err != nil {
			lasterr = fmt.Errorf("check status of kotsadm-rqlite: %v", err)
			return false, nil
		}
		if ready {
			count++
		}
		if writer != nil {
			writer.Infof("Waiting for the Admin Console to deploy: %d/2 ready", count)
		}
		return count == 2, nil
	}); err != nil {
		if lasterr == nil {
			lasterr = err
		}
		return lasterr
	}
	return nil
}
