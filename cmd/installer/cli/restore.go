package cli

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
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
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/configutils"
	"github.com/replicatedhq/embedded-cluster/pkg/constants"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8snet "k8s.io/utils/net"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8syaml "sigs.k8s.io/yaml"
)

type ecRestoreState string

const (
	ecRestoreStateNew                 ecRestoreState = "new"
	ecRestoreStateConfirmBackup       ecRestoreState = "confirm-backup"
	ecRestoreStateRestoreECInstall    ecRestoreState = "restore-ec-install"
	ecRestoreStateRestoreAdminConsole ecRestoreState = "restore-admin-console"
	ecRestoreStateWaitForNodes        ecRestoreState = "wait-for-nodes"
	ecRestoreStateRestoreSeaweedFS    ecRestoreState = "restore-seaweedfs"
	ecRestoreStateRestoreRegistry     ecRestoreState = "restore-registry"
	ecRestoreStateRestoreECO          ecRestoreState = "restore-embedded-cluster-operator"
	ecRestoreStateRestoreApp          ecRestoreState = "restore-app"
)

var ecRestoreStates = []ecRestoreState{
	ecRestoreStateNew,
	ecRestoreStateConfirmBackup,
	ecRestoreStateRestoreECInstall,
	ecRestoreStateRestoreAdminConsole,
	ecRestoreStateWaitForNodes,
	ecRestoreStateRestoreSeaweedFS,
	ecRestoreStateRestoreRegistry,
	ecRestoreStateRestoreECO,
	ecRestoreStateRestoreApp,
}

const (
	resourceModifiersCMName = "restore-resource-modifiers"
)

func RestoreCmd(ctx context.Context, name string) *cobra.Command {
	var (
		airgapBundle            string
		dataDir                 string
		localArtifactMirrorPort int
		networkInterface        string
		noPrompt                bool
		skipHostPreflights      bool
		skipStoreValidation     bool

		proxy *ecv1beta1.ProxySpec
	)

	cmd := &cobra.Command{
		Use:   "restore",
		Short: fmt.Sprintf("Restore a %s cluster", name),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if os.Getuid() != 0 {
				return fmt.Errorf("restore command must be run as root")
			}

			var err error
			proxy, err = getProxySpecFromFlags(cmd)
			if err != nil {
				return fmt.Errorf("unable to get proxy spec from flags: %w", err)
			}

			proxy, err = includeLocalIPInNoProxy(cmd, proxy)
			if err != nil {
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
			err := configutils.WriteRuntimeConfig(provider.GetRuntimeConfig())
			if err != nil {
				return fmt.Errorf("unable to write runtime config: %w", err)
			}

			if channelRelease, err := release.GetChannelRelease(); err != nil {
				return fmt.Errorf("unable to read channel release data: %w", err)
			} else if channelRelease != nil && channelRelease.Airgap && airgapBundle == "" && !noPrompt {
				logrus.Warnf("You downloaded an air gap bundle but didn't provide it with --airgap-bundle.")
				logrus.Warnf("If you continue, the restore will not use an air gap bundle and will connect to the internet.")
				if !prompts.New().Confirm("Are you sure you want to restore without an air gap bundle?", false) {
					return ErrNothingElseToAdd
				}
			}

			logrus.Debugf("configuring sysctl")
			if err := configutils.ConfigureSysctl(provider); err != nil {
				return fmt.Errorf("unable to configure sysctl: %w", err)
			}

			logrus.Debugf("getting restore state")
			state := getECRestoreState(cmd.Context())
			logrus.Debugf("restore state is: %q", state)

			if state != ecRestoreStateNew {
				shouldResume := prompts.New().Confirm("A previous restore operation was detected. Would you like to resume?", true)
				logrus.Info("")
				if !shouldResume {
					state = ecRestoreStateNew
				}
			}

			if airgapBundle != "" {
				logrus.Debugf("checking airgap bundle matches binary")
				if err := checkAirgapMatches(airgapBundle); err != nil {
					return err // we want the user to see the error message without a prefix
				}
			}

			// if the user wants to resume, check if a backup has already been picked.
			var backupToRestore *velerov1.Backup
			if state != ecRestoreStateNew {
				logrus.Debugf("getting backup from restore state")
				var err error
				backupToRestore, err = getBackupFromRestoreState(cmd.Context(), provider, airgapBundle != "")
				if err != nil {
					return fmt.Errorf("unable to resume: %w", err)
				}
				if backupToRestore != nil {
					completionTimestamp := backupToRestore.Status.CompletionTimestamp.Time.Format("2006-01-02 15:04:05 UTC")
					logrus.Infof("Resuming restore from backup %q (%s)\n", backupToRestore.Name, completionTimestamp)

					rc, err := overrideRuntimeConfigFromBackup(localArtifactMirrorPort, provider.GetRuntimeConfig(), backupToRestore)
					if err != nil {
						return fmt.Errorf("unable to override runtime config from backup: %w", err)
					}
					provider.SetRuntimeConfig(rc)
				}
			}

			// If the installation is available, we can further augment the runtime config from the
			// installation.
			rc, err := getRuntimeConfigFromInstallation(cmd)
			if err != nil {
				logrus.Debugf(
					"Unable to get runtime config from installation, this is expected if the installation is not yet available (restore state=%s): %v",
					state, err,
				)
			} else {
				provider.SetRuntimeConfig(rc)
			}

			opts := addonsApplierOpts{
				noPrompt:     noPrompt,
				license:      "",
				airgapBundle: airgapBundle,
				overrides:    "",
				privateCAs:   nil,
				configValues: "",
			}
			applier, err := getAddonsApplier(cmd, opts, provider.GetRuntimeConfig(), "", proxy)
			if err != nil {
				return err
			}

			switch state {
			case ecRestoreStateNew:
				logrus.Debugf("checking if %s is already installed", name)
				installed, err := k0s.IsInstalled(name)
				if err != nil {
					return err
				}
				if installed {
					return ErrNothingElseToAdd
				}

				logrus.Infof("You'll be guided through the process of restoring %s from a backup.\n", name)
				logrus.Info("Enter information to configure access to your backup storage location.\n")

				s3Store := newS3BackupStore()

				skipStoreValidationFlag, err := cmd.Flags().GetBool("skip-store-validation")
				if err != nil {
					return fmt.Errorf("unable to get skip-store-validation flag: %w", err)
				}

				if !skipStoreValidationFlag {
					logrus.Debugf("validating backup store configuration")
					if err := validateS3BackupStore(s3Store); err != nil {
						return fmt.Errorf("unable to validate backup store: %w", err)
					}
				}

				logrus.Debugf("configuring network manager")
				if err := configureNetworkManager(cmd.Context(), provider); err != nil {
					return fmt.Errorf("unable to configure network manager: %w", err)
				}

				logrus.Debugf("materializing binaries")
				if err := materializeFiles(airgapBundle, provider); err != nil {
					return fmt.Errorf("unable to materialize binaries: %w", err)
				}

				logrus.Debugf("running host preflights")
				if err := RunHostPreflightsForRestore(cmd, provider, applier, proxy); err != nil {
					return fmt.Errorf("unable to finish preflight checks: %w", err)
				}

				cfg, err := installAndWaitForRestoredK0sNode(cmd, name, provider, applier)
				if err != nil {
					return err
				}

				kcli, err := kubeutils.KubeClient()
				if err != nil {
					return fmt.Errorf("unable to create kube client: %w", err)
				}
				errCh := kubeutils.WaitForKubernetes(cmd.Context(), kcli)
				defer func() {
					for len(errCh) > 0 {
						err := <-errCh
						logrus.Error(fmt.Errorf("infrastructure failed to become ready: %w", err))
					}
				}()

				logrus.Debugf("running outro")
				if err := runOutroForRestore(cmd, applier, cfg); err != nil {
					return fmt.Errorf("unable to run outro: %w", err)
				}

				logrus.Debugf("configuring velero backup storage location")
				if err := kotscli.VeleroConfigureOtherS3(provider, kotscli.VeleroConfigureOtherS3Options{
					Endpoint:        s3Store.endpoint,
					Region:          s3Store.region,
					Bucket:          s3Store.bucket,
					Path:            s3Store.prefix,
					AccessKeyID:     s3Store.accessKeyID,
					SecretAccessKey: s3Store.secretAccessKey,
					Namespace:       defaults.KotsadmNamespace,
				}); err != nil {
					return err
				}
				fallthrough

			case ecRestoreStateConfirmBackup:
				logrus.Debugf("setting restore state to %q", ecRestoreStateConfirmBackup)
				if err := setECRestoreState(cmd.Context(), ecRestoreStateConfirmBackup, ""); err != nil {
					return fmt.Errorf("unable to set restore state: %w", err)
				}

				logrus.Debugf("waiting for backups to become available")
				backups, err := waitForBackups(cmd.Context(), provider, airgapBundle != "")
				if err != nil {
					return err
				}

				logrus.Debugf("picking backup to restore")
				backupToRestore = pickBackupToRestore(backups)

				logrus.Info("")
				completionTimestamp := backupToRestore.Status.CompletionTimestamp.Time.Format("2006-01-02 15:04:05 UTC")
				shouldRestore := prompts.New().Confirm(fmt.Sprintf("Restore from backup %q (%s)?", backupToRestore.Name, completionTimestamp), true)
				logrus.Info("")
				if !shouldRestore {
					logrus.Infof("Aborting restore...")
					return nil
				}
				fallthrough

			case ecRestoreStateRestoreECInstall:
				logrus.Debugf("setting restore state to %q", ecRestoreStateRestoreECInstall)
				if err := setECRestoreState(cmd.Context(), ecRestoreStateRestoreECInstall, backupToRestore.Name); err != nil {
					return fmt.Errorf("unable to set restore state: %w", err)
				}

				logrus.Debugf("restoring embedded cluster installation from backup %q", backupToRestore.Name)
				if err := restoreFromBackup(cmd.Context(), backupToRestore, disasterRecoveryComponentECInstall); err != nil {
					return fmt.Errorf("unable to restore from backup: %w", err)
				}

				logrus.Debugf("updating installation from backup %q", backupToRestore.Name)
				if err := restoreReconcileInstallationFromRuntimeConfig(cmd.Context(), provider.GetRuntimeConfig()); err != nil {
					return fmt.Errorf("unable to update installation from backup: %w", err)
				}

				logrus.Debugf("updating local artifact mirror service from backup %q", backupToRestore.Name)
				if err := updateLocalArtifactMirrorService(provider); err != nil {
					return fmt.Errorf("unable to update local artifact mirror service from backup: %w", err)
				}

				fallthrough

			case ecRestoreStateRestoreAdminConsole:
				logrus.Debugf("setting restore state to %q", ecRestoreStateRestoreAdminConsole)
				if err := setECRestoreState(cmd.Context(), ecRestoreStateRestoreAdminConsole, backupToRestore.Name); err != nil {
					return fmt.Errorf("unable to set restore state: %w", err)
				}

				logrus.Debugf("restoring admin console from backup %q", backupToRestore.Name)
				if err := restoreFromBackup(cmd.Context(), backupToRestore, disasterRecoveryComponentAdminConsole); err != nil {
					return err
				}

				fallthrough

			case ecRestoreStateWaitForNodes:
				logrus.Debugf("setting restore state to %q", ecRestoreStateWaitForNodes)
				if err := setECRestoreState(cmd.Context(), ecRestoreStateWaitForNodes, backupToRestore.Name); err != nil {
					return fmt.Errorf("unable to set restore state: %w", err)
				}

				logrus.Debugf("checking if backup is high availability")
				highAvailability, err := isHighAvailabilityBackup(backupToRestore)
				if err != nil {
					return err
				}

				logrus.Debugf("waiting for additional nodes to be added")

				networkInterfaceFlag, err := cmd.Flags().GetString("network-interface")
				if err != nil {
					return fmt.Errorf("unable to get network-interface flag: %w", err)
				}

				if err := waitForAdditionalNodes(cmd.Context(), provider, highAvailability, networkInterfaceFlag); err != nil {
					return err
				}

				fallthrough

			case ecRestoreStateRestoreSeaweedFS:
				// only restore seaweedfs in case of high availability and airgap
				highAvailability, err := isHighAvailabilityBackup(backupToRestore)
				if err != nil {
					return err
				}

				if highAvailability && airgapBundle != "" {
					logrus.Debugf("setting restore state to %q", ecRestoreStateRestoreSeaweedFS)
					if err := setECRestoreState(cmd.Context(), ecRestoreStateRestoreSeaweedFS, backupToRestore.Name); err != nil {
						return fmt.Errorf("unable to set restore state: %w", err)
					}
					logrus.Debugf("restoring seaweedfs from backup %q", backupToRestore.Name)
					if err := restoreFromBackup(cmd.Context(), backupToRestore, disasterRecoveryComponentSeaweedFS); err != nil {
						return err
					}
				}

				fallthrough

			case ecRestoreStateRestoreRegistry:
				// only restore registry in case of airgap
				if airgapBundle != "" {
					logrus.Debugf("setting restore state to %q", ecRestoreStateRestoreRegistry)
					if err := setECRestoreState(cmd.Context(), ecRestoreStateRestoreRegistry, backupToRestore.Name); err != nil {
						return fmt.Errorf("unable to set restore state: %w", err)
					}

					logrus.Debugf("restoring embedded cluster registry from backup %q", backupToRestore.Name)
					if err := restoreFromBackup(cmd.Context(), backupToRestore, disasterRecoveryComponentRegistry); err != nil {
						return err
					}

					registryAddress, ok := backupToRestore.Annotations["kots.io/embedded-registry"]
					if !ok {
						return fmt.Errorf("unable to read registry address from backup")
					}

					if err := airgap.AddInsecureRegistry(registryAddress); err != nil {
						return fmt.Errorf("failed to add insecure registry: %w", err)
					}
				}
				fallthrough

			case ecRestoreStateRestoreECO:
				logrus.Debugf("setting restore state to %q", ecRestoreStateRestoreECO)
				if err := setECRestoreState(cmd.Context(), ecRestoreStateRestoreECO, backupToRestore.Name); err != nil {
					return fmt.Errorf("unable to set restore state: %w", err)
				}

				logrus.Debugf("restoring embedded cluster operator from backup %q", backupToRestore.Name)
				if err := restoreFromBackup(cmd.Context(), backupToRestore, disasterRecoveryComponentECO); err != nil {
					return err
				}

				fallthrough

			case ecRestoreStateRestoreApp:
				logrus.Debugf("setting restore state to %q", ecRestoreStateRestoreApp)
				if err := setECRestoreState(cmd.Context(), ecRestoreStateRestoreApp, backupToRestore.Name); err != nil {
					return fmt.Errorf("unable to set restore state: %w", err)
				}

				logrus.Debugf("restoring app from backup %q", backupToRestore.Name)
				if err := restoreFromBackup(cmd.Context(), backupToRestore, disasterRecoveryComponentApp); err != nil {
					return err
				}

				logrus.Debugf("resetting restore state")
				if err := resetECRestoreState(cmd.Context()); err != nil {
					return fmt.Errorf("unable to reset restore state: %w", err)
				}

			default:
				return fmt.Errorf("unknown restore state: %q", state)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&airgapBundle, "airgap-bundle", "", "Path to the air gap bundle. If set, the restore will complete without internet access.")
	cmd.Flags().StringVar(&dataDir, "data-dir", ecv1beta1.DefaultDataDir, "Path to the data directory")
	cmd.Flags().IntVar(&localArtifactMirrorPort, "local-artifact-mirror-port", ecv1beta1.DefaultLocalArtifactMirrorPort, "Local artifact mirror port")
	cmd.Flags().StringVar(&networkInterface, "network-interface", "", "The network interface to use for the cluster")
	cmd.Flags().BoolVar(&noPrompt, "no-prompt", false, "Disable interactive prompts. The Admin Console password will be set to password.")
	cmd.Flags().BoolVar(&skipHostPreflights, "skip-host-preflights", false, "Skip host preflight checks. This is not recommended.")
	cmd.Flags().BoolVar(&skipStoreValidation, "skip-store-validation", false, "Skip validation of the backup storage location")
	cmd.Flags().String("overrides", "", "File with an EmbeddedClusterConfig object to override the default configuration")

	addProxyFlags(cmd)

	cmd.Flags().String("pod-cidr", k0sv1beta1.DefaultNetwork().PodCIDR, "IP address range for Pods")
	cmd.Flags().MarkHidden("pod-cidr")
	cmd.Flags().String("service-cidr", k0sv1beta1.DefaultNetwork().ServiceCIDR, "IP address range for Services")
	cmd.Flags().MarkHidden("service-cidr")
	cmd.Flags().String("cidr", ecv1beta1.DefaultNetworkCIDR, "CIDR block of available private IP addresses (/16 or larger)")

	return cmd
}

// getECRestoreState returns the current restore state.
func getECRestoreState(ctx context.Context) ecRestoreState {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return ecRestoreStateNew
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "embedded-cluster",
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
			Name: "embedded-cluster",
		},
	}

	if err := kcli.Create(ctx, ns); err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("unable to create namespace: %w", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "embedded-cluster",
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
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("unable to create config map: %w", err)
	}

	if errors.IsAlreadyExists(err) {
		if err := kcli.Update(ctx, cm); err != nil {
			return fmt.Errorf("unable to update config map: %w", err)
		}
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
			Namespace: "embedded-cluster",
			Name:      constants.EcRestoreStateCMName,
		},
	}

	if err := kcli.Delete(ctx, cm); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("unable to delete config map: %w", err)
	}

	return nil
}

// getBackupFromRestoreState gets the backup defined in the restore state.
// If no backup is defined in the restore state, it returns nil.
// It returns an error if a backup is defined in the restore state but:
//   - is not found by Velero anymore.
//   - is not restorable by the current binary.
func getBackupFromRestoreState(ctx context.Context, provider *defaults.Provider, isAirgap bool) (*velerov1.Backup, error) {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return nil, fmt.Errorf("unable to create kube client: %w", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "embedded-cluster",
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

	backup := velerov1api.Backup{}
	err = kcli.Get(ctx, types.NamespacedName{Name: backupName, Namespace: defaults.VeleroNamespace}, &backup)
	if err != nil {
		return nil, fmt.Errorf("unable to get backup: %w", err)
	}

	rel, err := release.GetChannelRelease()
	if err != nil {
		return nil, fmt.Errorf("unable to get release from binary: %w", err)
	}

	if rel == nil {
		return nil, fmt.Errorf("no release found in binary")
	}

	k0sCfg, err := getK0sConfigFromDisk()
	if err != nil {
		return nil, fmt.Errorf("unable to get k0s config from disk: %w", err)
	}

	if restorable, reason := isBackupRestorable(&backup, provider, rel, isAirgap, k0sCfg); !restorable {
		return nil, fmt.Errorf("backup %q %s", backup.Name, reason)
	}

	return &backup, nil
}

// newS3BackupStore prompts the user for S3 backup store configuration.
func newS3BackupStore() *s3BackupStore {
	store := &s3BackupStore{}

	for {
		store.endpoint = prompts.New().Input("S3 endpoint:", "", true)
		if strings.HasPrefix(store.endpoint, "http://") || strings.HasPrefix(store.endpoint, "https://") {
			break
		}
		logrus.Info("Endpoint must start with http:// or https://")
	}

	store.region = prompts.New().Input("Region:", "", true)
	store.bucket = prompts.New().Input("Bucket:", "", true)
	store.prefix = strings.TrimPrefix(prompts.New().Input("Prefix (press Enter to skip):", "", false), "/")
	store.accessKeyID = prompts.New().Input("Access key ID:", "", true)
	store.secretAccessKey = prompts.New().Password("Secret access key:")
	logrus.Info("")

	return store
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

// RunHostPreflightsForRestore runs the host preflights we found embedded in the binary
// on all configured hosts. We attempt to read HostPreflights from all the
// embedded Helm Charts for restore operations.
func RunHostPreflightsForRestore(cmd *cobra.Command, provider *defaults.Provider, applier *addons.Applier, proxy *ecv1beta1.ProxySpec) error {
	hpf, err := applier.HostPreflightsForRestore()
	if err != nil {
		return fmt.Errorf("unable to read host preflights: %w", err)
	}

	return runHostPreflights(cmd, provider, hpf, proxy)
}

// ensureK0sConfigForRestore creates a new k0s.yaml configuration file for restore operations.
// The file is saved in the global location (as returned by defaults.PathToK0sConfig()).
// If a file already sits there, this function returns an error.
func ensureK0sConfigForRestore(cmd *cobra.Command, provider *defaults.Provider, applier *addons.Applier) (*k0sv1beta1.ClusterConfig, error) {
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

	if err := config.UpdateHelmConfigsForRestore(applier, cfg); err != nil {
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

// runOutroForRestore calls Outro() in all enabled addons for restore operations by means of Applier.
func runOutroForRestore(cmd *cobra.Command, applier *addons.Applier, cfg *k0sv1beta1.ClusterConfig) error {
	return applier.OutroForRestore(cmd.Context(), cfg)
}

func isBackupRestorable(backup *velerov1.Backup, provider *defaults.Provider, rel *release.ChannelRelease, isAirgap bool, k0sCfg *k0sv1beta1.ClusterConfig) (bool, string) {
	if backup.Annotations["kots.io/embedded-cluster"] != "true" {
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

	if v := backup.Annotations["kots.io/embedded-cluster-data-dir"]; v != "" && v != provider.EmbeddedClusterHomeDirectory() {
		return false, fmt.Sprintf("has a different data directory than the current cluster. Please rerun with '--data-dir %s'.", v)
	}

	return true, ""
}

func isHighAvailabilityBackup(backup *velerov1.Backup) (bool, error) {
	ha, ok := backup.Annotations["kots.io/embedded-cluster-is-ha"]
	if !ok {
		return false, fmt.Errorf("high availability annotation not found in backup")
	}

	return ha == "true", nil
}

// waitForBackups waits for backups to become available.
// It returns a list of restorable backups, or an error if none are found.
func waitForBackups(ctx context.Context, provider *defaults.Provider, isAirgap bool) ([]velerov1.Backup, error) {
	loading := spinner.Start()
	defer loading.Close()
	loading.Infof("Waiting for backups to become available")

	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return nil, fmt.Errorf("unable to create kube client: %w", err)
	}

	rel, err := release.GetChannelRelease()
	if err != nil {
		return nil, fmt.Errorf("unable to get release from binary: %w", err)
	}

	if rel == nil {
		return nil, fmt.Errorf("no release found in binary")
	}

	k0sCfg, err := getK0sConfigFromDisk()
	if err != nil {
		return nil, fmt.Errorf("unable to get k0s config from disk: %w", err)
	}

	for i := 0; i < 30; i++ {
		time.Sleep(5 * time.Second)

		backupList := velerov1api.BackupList{}
		err = kcli.List(ctx, &backupList, client.InNamespace(defaults.VeleroNamespace))

		if err != nil {
			return nil, fmt.Errorf("unable to list backups: %w", err)
		}
		if len(backupList.Items) == 0 {
			logrus.Debugf("No backups found yet...")
			continue
		}

		validBackups := []velerov1.Backup{}
		invalidBackups := []velerov1.Backup{}
		invalidReasons := []string{}

		for _, backup := range backupList.Items {
			restorable, reason := isBackupRestorable(&backup, provider, rel, isAirgap, k0sCfg)
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

		if len(validBackups) == 1 {
			loading.Infof("Found 1 restorable backup!")
		} else {
			loading.Infof("Found %d restorable backups!", len(validBackups))
		}
		return validBackups, nil
	}

	return nil, fmt.Errorf("timed out waiting for backups to become available")
}

// pickBackupToRestore picks a backup to restore from a list of backups.
// Currently, it picks the latest backup.
func pickBackupToRestore(backups []velerov1.Backup) *velerov1.Backup {
	var latestBackup *velerov1.Backup
	for _, b := range backups {
		if latestBackup == nil {
			latestBackup = &b
			continue
		}
		if b.Status.CompletionTimestamp.After(latestBackup.Status.CompletionTimestamp.Time) {
			latestBackup = &b
		}
	}
	return latestBackup
}

// getK0sConfigFromDisk reads and returns the k0s config from disk.
func getK0sConfigFromDisk() (*k0sv1beta1.ClusterConfig, error) {
	cfgBytes, err := os.ReadFile(defaults.PathToK0sConfig())
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
		restore := velerov1api.Restore{}
		err = kcli.Get(ctx, types.NamespacedName{Name: restoreName, Namespace: defaults.VeleroNamespace}, &restore)
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
			Namespace: defaults.VeleroNamespace,
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

	if err := kcli.Create(ctx, cm); err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("unable to create config map: %w", err)
	}

	return nil
}

// waitForDRComponent waits for a disaster recovery component to be restored.
func waitForDRComponent(ctx context.Context, drComponent disasterRecoveryComponent, restoreName string) error {
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

		if err := adminconsole.WaitForReady(ctx, kcli, defaults.KotsadmNamespace, loading); err != nil {
			return fmt.Errorf("unable to wait for admin console: %w", err)
		}
	} else if drComponent == disasterRecoveryComponentSeaweedFS {
		// wait for seaweedfs to be ready
		kcli, err := kubeutils.KubeClient()
		if err != nil {
			return fmt.Errorf("unable to create kube client: %w", err)
		}

		if err := seaweedfs.WaitForReady(ctx, kcli, defaults.SeaweedFSNamespace, nil); err != nil {
			return fmt.Errorf("unable to wait for seaweedfs to be ready: %w", err)
		}
	} else if drComponent == disasterRecoveryComponentRegistry {
		// wait for registry to be ready
		kcli, err := kubeutils.KubeClient()
		if err != nil {
			return fmt.Errorf("unable to create kube client: %w", err)
		}

		if err := kubeutils.WaitForDeployment(ctx, kcli, defaults.RegistryNamespace, "registry"); err != nil {
			return fmt.Errorf("unable to wait for registry to be ready: %w", err)
		}
	} else if drComponent == disasterRecoveryComponentECO {
		// wait for embedded cluster operator to reconcile the installation
		kcli, err := kubeutils.KubeClient()
		if err != nil {
			return fmt.Errorf("unable to create kube client: %w", err)
		}

		if err := kubeutils.WaitForInstallation(ctx, kcli, loading); err != nil {
			return fmt.Errorf("unable to wait for installation to be ready: %w", err)
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

// restoreFromBackup restores a disaster recovery component from a backup.
func restoreFromBackup(ctx context.Context, backup *velerov1.Backup, drComponent disasterRecoveryComponent) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	restoreName := fmt.Sprintf("%s.%s", backup.Name, string(drComponent))

	// check if a restore object already exists
	rest := velerov1api.Restore{}
	err = kcli.Get(ctx, types.NamespacedName{Name: restoreName, Namespace: defaults.VeleroNamespace}, &rest)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("unable to get restore: %w", err)
	}

	// create a new restore object if it doesn't exist
	if errors.IsNotFound(err) {
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
				Namespace: defaults.VeleroNamespace,
				Name:      restoreName,
				Annotations: map[string]string{
					"kots.io/embedded-cluster": "true",
				},
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

		// ensure restore resource modifiers first
		if err := ensureRestoreResourceModifiers(ctx, backup); err != nil {
			return fmt.Errorf("unable to ensure restore resource modifiers: %w", err)
		}

		err = kcli.Create(ctx, restore)
		if err != nil {
			return fmt.Errorf("unable to create restore: %w", err)
		}
	}

	// wait for restore to complete
	return waitForDRComponent(ctx, drComponent, restoreName)
}

// waitForAdditionalNodes waits for for user to add additional nodes to the cluster.
func waitForAdditionalNodes(ctx context.Context, provider *defaults.Provider, highAvailability bool, networkInterface string) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	successColor := "\033[32m"
	colorReset := "\033[0m"
	joinNodesMsg := fmt.Sprintf("\nVisit the Admin Console if you need to add nodes to the cluster: %s%s%s\n",
		successColor, adminconsole.GetURL(networkInterface, provider.AdminConsolePort()), colorReset,
	)
	logrus.Info(joinNodesMsg)

	for {
		p := prompts.New().Input("Type 'continue' when you are done adding nodes:", "", false)
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

func installAndWaitForRestoredK0sNode(cmd *cobra.Command, name string, provider *defaults.Provider, applier *addons.Applier) (*k0sv1beta1.ClusterConfig, error) {
	loading := spinner.Start()
	defer loading.Close()

	loading.Infof("Installing %s node", name)
	logrus.Debugf("creating k0s configuration file")

	cfg, err := ensureK0sConfigForRestore(cmd, provider, applier)
	if err != nil {
		return nil, fmt.Errorf("unable to create config file: %w", err)
	}

	proxy, err := getProxySpecFromFlags(cmd)
	if err != nil {
		return nil, fmt.Errorf("unable to get proxy spec from flags: %w", err)
	}

	logrus.Debugf("creating systemd unit files")
	if err := createSystemdUnitFiles(provider, false, proxy); err != nil {
		return nil, fmt.Errorf("unable to create systemd unit files: %w", err)
	}

	logrus.Debugf("installing k0s")
	networkInterface, err := cmd.Flags().GetString("network-interface")
	if err != nil {
		return nil, fmt.Errorf("unable to get network-interface flag: %w", err)
	}

	if err := k0s.Install(networkInterface, provider); err != nil {
		return nil, fmt.Errorf("unable to install cluster: %w", err)
	}

	loading.Infof("Waiting for %s node to be ready", name)
	logrus.Debugf("waiting for k0s to be ready")
	if err := waitForK0s(); err != nil {
		return nil, fmt.Errorf("unable to wait for node: %w", err)
	}

	loading.Infof("Node installation finished!")

	return cfg, nil
}

// restoreReconcileInstallationFromRuntimeConfig will update the installation to match the runtime
// config from the original installation.
func restoreReconcileInstallationFromRuntimeConfig(ctx context.Context, runtimeConfig *ecv1beta1.RuntimeConfigSpec) error {
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

	// We allow the user to override the port with a flag to the restore command.
	in.Spec.RuntimeConfig.LocalArtifactMirror.Port = runtimeConfig.LocalArtifactMirror.Port

	if err := kcli.Update(ctx, in); err != nil {
		return fmt.Errorf("update installation: %w", err)
	}

	in.Status.State = ecv1beta1.InstallationStateKubernetesInstalled

	if err := kcli.Status().Update(ctx, in); err != nil {
		return fmt.Errorf("update installation status: %w", err)
	}

	return nil
}

// overrideRuntimeConfigFromBackup will update the runtime config from the backup. These values may
// be used during the install and set in the Installation object via the
// restoreReconcileInstallationFromRuntimeConfig function.
func overrideRuntimeConfigFromBackup(localArtifactMirrorPort int, runtimeConfig *ecv1beta1.RuntimeConfigSpec, backup *velerov1.Backup) (*ecv1beta1.RuntimeConfigSpec, error) {
	if localArtifactMirrorPort != 0 {
		if val := backup.Annotations["kots.io/embedded-cluster-local-artifact-mirror-port"]; val != "" {
			port, err := k8snet.ParsePort(val, false)
			if err != nil {
				return nil, fmt.Errorf("parse local artifact mirror port: %w", err)
			}
			logrus.Debugf("updating local artifact mirror port to %d from backup %q", port, backup.Name)
			runtimeConfig.LocalArtifactMirror.Port = port
		}
	}

	return runtimeConfig, nil
}

// getRuntimeConfigFromInstallation returns the runtime config from the latest installation.
func getRuntimeConfigFromInstallation(cmd *cobra.Command) (*ecv1beta1.RuntimeConfigSpec, error) {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return nil, fmt.Errorf("unable to create kube client: %w", err)
	}

	in, err := kubeutils.GetLatestInstallation(cmd.Context(), kcli)
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
	invalidBackups []velerov1.Backup
	invalidReasons []string
}

func (e *invalidBackupsError) Error() string {
	reasons := []string{}
	for i, backup := range e.invalidBackups {
		reasons = append(reasons, fmt.Sprintf("%q %s", backup.Name, e.invalidReasons[i]))
	}

	if len(e.invalidBackups) == 1 {
		return fmt.Sprintf("\nFound 1 backup, but it is not restorable:\n%s\n", strings.Join(reasons, "\n"))
	}

	return fmt.Sprintf("\nFound %d backups, but none are restorable:\n%s\n", len(e.invalidBackups), strings.Join(reasons, "\n"))
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