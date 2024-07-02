package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	veleroclientv1 "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/typed/velero/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	k8sconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
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
	ecRestoreStateCMName    = "embedded-cluster-restore-state"
	resourceModifiersCMName = "restore-resource-modifiers"
)

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

// getECRestoreState returns the current restore state.
func getECRestoreState(ctx context.Context) ecRestoreState {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return ecRestoreStateNew
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "embedded-cluster",
			Name:      ecRestoreStateCMName,
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
			Name:      ecRestoreStateCMName,
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
			Name:      ecRestoreStateCMName,
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
func getBackupFromRestoreState(ctx context.Context, isAirgap bool) (*velerov1.Backup, error) {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return nil, fmt.Errorf("unable to create kube client: %w", err)
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "embedded-cluster",
			Name:      ecRestoreStateCMName,
		},
	}
	if err := kcli.Get(ctx, types.NamespacedName{Namespace: cm.Namespace, Name: cm.Name}, cm); err != nil {
		return nil, fmt.Errorf("unable to get restore state: %w", err)
	}
	backupName, ok := cm.Data["backup-name"]
	if !ok || backupName == "" {
		return nil, nil
	}
	cfg, err := k8sconfig.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to get kubernetes config: %w", err)
	}
	veleroClient, err := veleroclientv1.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to create velero client: %w", err)
	}
	backup, err := veleroClient.Backups(defaults.VeleroNamespace).Get(ctx, backupName, metav1.GetOptions{})
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
	if restorable, reason := isBackupRestorable(backup, rel, isAirgap); !restorable {
		return nil, fmt.Errorf("backup %q %s", backup.Name, reason)
	}
	return backup, nil
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
	store.prefix = prompts.New().Input("Prefix (press Enter to skip):", "", false)
	store.accessKeyID = prompts.New().Input("Access key ID:", "", true)
	store.secretAccessKey = prompts.New().Password("Secret access key:")
	logrus.Info("")
	return store
}

// validateS3BackupStore validates the S3 backup store configuration.
// It tries to list objects in the bucket and prefix to ensure that the bucket exists and has backups.
func validateS3BackupStore(s *s3BackupStore) error {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(s.region),
		Endpoint:    aws.String(s.endpoint),
		Credentials: credentials.NewStaticCredentials(s.accessKeyID, s.secretAccessKey, ""),
	})
	if err != nil {
		return fmt.Errorf("unable to create s3 session: %v", err)
	}
	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(s.bucket),
		Delimiter: aws.String("/"),
		Prefix:    aws.String(fmt.Sprintf("%s/", filepath.Join(s.prefix, "backups"))),
	}
	svc := s3.New(sess)
	result, err := svc.ListObjectsV2(input)
	if err != nil {
		return fmt.Errorf("unable to list objects: %v", err)
	}
	if len(result.CommonPrefixes) == 0 {
		return fmt.Errorf("no backups found in %s", filepath.Join(s.bucket, s.prefix))
	}
	return nil
}

// RunHostPreflightsForRestore runs the host preflights we found embedded in the binary
// on all configured hosts. We attempt to read HostPreflights from all the
// embedded Helm Charts for restore operations.
func RunHostPreflightsForRestore(c *cli.Context) error {
	hpf, err := addons.NewApplier().HostPreflightsForRestore()
	if err != nil {
		return fmt.Errorf("unable to read host preflights: %w", err)
	}
	return runHostPreflights(c, hpf)
}

// ensureK0sConfigForRestore creates a new k0s.yaml configuration file for restore operations.
// The file is saved in the global location (as returned by defaults.PathToK0sConfig()).
// If a file already sits there, this function returns an error.
func ensureK0sConfigForRestore(c *cli.Context) error {
	cfgpath := defaults.PathToK0sConfig()
	if _, err := os.Stat(cfgpath); err == nil {
		return fmt.Errorf("configuration file already exists")
	}
	if err := os.MkdirAll(filepath.Dir(cfgpath), 0755); err != nil {
		return fmt.Errorf("unable to create directory: %w", err)
	}
	cfg := config.RenderK0sConfig()
	opts := []addons.Option{}
	if c.Bool("proxy") {
		opts = append(opts, addons.WithProxyFromEnv())
	}
	if err := config.UpdateHelmConfigsForRestore(cfg, opts...); err != nil {
		return fmt.Errorf("unable to update helm configs: %w", err)
	}
	var err error
	if cfg, err = applyUnsupportedOverrides(c, cfg); err != nil {
		return fmt.Errorf("unable to apply unsupported overrides: %w", err)
	}
	if c.String("airgap-bundle") != "" {
		// update the k0s config to install with airgap
		airgap.RemapHelm(cfg)
		airgap.SetAirgapConfig(cfg)
	}
	data, err := k8syaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("unable to marshal config: %w", err)
	}
	fp, err := os.OpenFile(cfgpath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("unable to create config file: %w", err)
	}
	defer fp.Close()
	if _, err := fp.Write(data); err != nil {
		return fmt.Errorf("unable to write config file: %w", err)
	}
	return nil
}

// runOutroForRestore calls Outro() in all enabled addons for restore operations by means of Applier.
func runOutroForRestore(c *cli.Context) error {
	return addons.NewApplier().OutroForRestore(c.Context)
}

func isBackupRestorable(backup *velerov1.Backup, rel *release.ChannelRelease, isAirgap bool) (bool, string) {
	if backup.Annotations["kots.io/embedded-cluster"] != "true" {
		return false, "is not an embedded cluster backup"
	}
	if v := strings.TrimPrefix(backup.Annotations["kots.io/embedded-cluster-version"], "v"); v != strings.TrimPrefix(defaults.Version, "v") {
		return false, fmt.Sprintf("has a different embedded cluster version (%q) than the current version (%q)", v, defaults.Version)
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
func waitForBackups(ctx context.Context, isAirgap bool) ([]velerov1.Backup, error) {
	loading := spinner.Start()
	defer loading.Close()
	loading.Infof("Waiting for backups to become available")

	cfg, err := k8sconfig.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to get kubernetes config: %w", err)
	}

	veleroClient, err := veleroclientv1.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to create velero client: %w", err)
	}

	rel, err := release.GetChannelRelease()
	if err != nil {
		return nil, fmt.Errorf("unable to get release from binary: %w", err)
	}
	if rel == nil {
		return nil, fmt.Errorf("no release found in binary")
	}

	for i := 0; i < 30; i++ {
		time.Sleep(5 * time.Second)

		backupList, err := veleroClient.Backups(defaults.VeleroNamespace).List(ctx, metav1.ListOptions{})
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
			restorable, reason := isBackupRestorable(&backup, rel, isAirgap)
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

// waitForVeleroRestoreCompleted waits for a Velero restore to complete.
func waitForVeleroRestoreCompleted(ctx context.Context, restoreName string) (*velerov1.Restore, error) {
	cfg, err := k8sconfig.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to get kubernetes config: %w", err)
	}

	veleroClient, err := veleroclientv1.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to create velero client: %w", err)
	}

	for {
		restore, err := veleroClient.Restores(defaults.VeleroNamespace).Get(ctx, restoreName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("unable to get restore: %w", err)
		}

		switch restore.Status.Phase {
		case velerov1.RestorePhaseCompleted:
			return restore, nil
		case velerov1.RestorePhaseFailed:
			return restore, fmt.Errorf("restore failed")
		case velerov1.RestorePhasePartiallyFailed:
			return restore, fmt.Errorf("restore partially failed")
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
	cfg, err := k8sconfig.GetConfig()
	if err != nil {
		return fmt.Errorf("unable to get kubernetes config: %w", err)
	}

	veleroClient, err := veleroclientv1.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("unable to create velero client: %w", err)
	}

	restoreName := fmt.Sprintf("%s.%s", backup.Name, string(drComponent))

	// check if a restore object already exists
	_, err = veleroClient.Restores(defaults.VeleroNamespace).Get(ctx, restoreName, metav1.GetOptions{})
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

		_, err := veleroClient.Restores(defaults.VeleroNamespace).Create(ctx, restore, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("unable to create restore: %w", err)
		}
	}

	// wait for restore to complete
	return waitForDRComponent(ctx, drComponent, restoreName)
}

// waitForAdditionalNodes waits for for user to add additional nodes to the cluster.
func waitForAdditionalNodes(ctx context.Context, highAvailability bool) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	successColor := "\033[32m"
	colorReset := "\033[0m"
	joinNodesMsg := fmt.Sprintf("\nVisit the Admin Console if you need to add nodes to the cluster: %s%s%s\n",
		successColor, adminconsole.GetURL(), colorReset,
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

var restoreCommand = &cli.Command{
	Name:  "restore",
	Usage: fmt.Sprintf("Restore a %s cluster", binName),
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:   "airgap-bundle",
			Usage:  "Path to the airgap bundle. If set, the restore will be completed without internet access.",
			Hidden: true,
		},
		&cli.StringFlag{
			Name:   "http-proxy",
			Usage:  "HTTP proxy to use for the restore",
			Hidden: false,
		},
		&cli.StringFlag{
			Name:   "https-proxy",
			Usage:  "HTTPS proxy to use for the restore",
			Hidden: false,
		},
		&cli.StringFlag{
			Name:   "no-proxy",
			Usage:  "Comma separated list of hosts to bypass the proxy for",
			Hidden: false,
		},
		&cli.BoolFlag{
			Name:   "proxy",
			Usage:  "Use the system proxy settings for the restore operation. These variables are currently only passed through to Velero.",
			Hidden: true,
		},
		&cli.BoolFlag{
			Name:  "skip-host-preflights",
			Usage: "Skip host preflight checks. This is not recommended unless you are sure your system is compatible.",
			Value: false,
		},
	},
	Before: func(c *cli.Context) error {
		if os.Getuid() != 0 {
			return fmt.Errorf("restore command must be run as root")
		}
		os.Setenv("KUBECONFIG", defaults.PathToKubeConfig())
		return nil
	},
	Action: func(c *cli.Context) error {
		logrus.Debugf("getting restore state")
		state := getECRestoreState(c.Context)
		logrus.Debugf("restore state is: %q", state)

		if state != ecRestoreStateNew {
			shouldResume := prompts.New().Confirm("A previous restore operation was detected. Would you like to resume?", true)
			logrus.Info("")
			if !shouldResume {
				state = ecRestoreStateNew
			}
		}
		if c.String("airgap-bundle") != "" {
			logrus.Debugf("checking airgap bundle matches binary")
			if err := checkAirgapMatches(c); err != nil {
				return err // we want the user to see the error message without a prefix
			}
		}

		// if the user wants to resume, check if a backup has already been picked.
		var backupToRestore *velerov1.Backup
		if state != ecRestoreStateNew {
			logrus.Debugf("getting backup from restore state")
			var err error
			backupToRestore, err = getBackupFromRestoreState(c.Context, c.String("airgap-bundle") != "")
			if err != nil {
				return fmt.Errorf("unable to resume: %w", err)
			}
			if backupToRestore != nil {
				completionTimestamp := backupToRestore.Status.CompletionTimestamp.Time.Format("2006-01-02 15:04:05 UTC")
				logrus.Infof("Resuming restore from backup %q (%s)\n", backupToRestore.Name, completionTimestamp)
			}
		}

		switch state {
		case ecRestoreStateNew:
			logrus.Debugf("checking if %s is already installed", binName)
			if installed, err := isAlreadyInstalled(); err != nil {
				return err
			} else if installed {
				logrus.Errorf("An installation has been detected on this machine.")
				logrus.Infof("If you want to restore you need to remove the existing installation")
				logrus.Infof("first. You can do this by running the following command:")
				logrus.Infof("\n  sudo ./%s reset\n", binName)
				return ErrNothingElseToAdd
			}

			logrus.Infof("You'll be guided through the process of restoring %s from a backup.\n", binName)
			logrus.Info("Enter information to configure access to your backup storage location.\n")
			s3Store := newS3BackupStore()

			logrus.Debugf("validating backup store configuration")
			if err := validateS3BackupStore(s3Store); err != nil {
				return fmt.Errorf("unable to validate backup store: %w", err)
			}

			logrus.Debugf("configuring network manager")
			if err := configureNetworkManager(c); err != nil {
				return fmt.Errorf("unable to configure network manager: %w", err)
			}
			logrus.Debugf("materializing binaries")
			if err := materializeFiles(c); err != nil {
				return fmt.Errorf("unable to materialize binaries: %w", err)
			}
			logrus.Debugf("running host preflights")
			if err := RunHostPreflightsForRestore(c); err != nil {
				return fmt.Errorf("unable to finish preflight checks: %w", err)
			}
			logrus.Debugf("creating k0s configuration file")
			if err := ensureK0sConfigForRestore(c); err != nil {
				return fmt.Errorf("unable to create config file: %w", err)
			}
			var proxy *Proxy
			if c.String("http-proxy") != "" || c.String("https-proxy") != "" || c.String("no-proxy") != "" {
				proxy = &Proxy{
					HTTPProxy:  c.String("http-proxy"),
					HTTPSProxy: c.String("https-proxy"),
					NoProxy:    strings.Join(append(defaults.DefaultNoProxy, c.String("no-proxy")), ","),
				}
			}
			logrus.Debugf("creating systemd unit files")
			if err := createSystemdUnitFiles(false, proxy); err != nil {
				return fmt.Errorf("unable to create systemd unit files: %w", err)
			}
			logrus.Debugf("installing k0s")
			if err := installK0s(); err != nil {
				return fmt.Errorf("unable update cluster: %w", err)
			}
			logrus.Debugf("waiting for k0s to be ready")
			if err := waitForK0s(); err != nil {
				return fmt.Errorf("unable to wait for node: %w", err)
			}

			kcli, err := kubeutils.KubeClient()
			if err != nil {
				return fmt.Errorf("unable to create kube client: %w", err)
			}
			errCh := kubeutils.WaitForKubernetes(c.Context, kcli)
			defer func() {
				for len(errCh) > 0 {
					err := <-errCh
					logrus.Error(fmt.Errorf("infrastructure failed to become ready: %w", err))
				}
			}()

			logrus.Debugf("running outro")
			if err := runOutroForRestore(c); err != nil {
				return fmt.Errorf("unable to run outro: %w", err)
			}

			logrus.Debugf("configuring velero backup storage location")
			if err := kotscli.VeleroConfigureOtherS3(kotscli.VeleroConfigureOtherS3Options{
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
			if err := setECRestoreState(c.Context, ecRestoreStateConfirmBackup, ""); err != nil {
				return fmt.Errorf("unable to set restore state: %w", err)
			}

			logrus.Debugf("waiting for backups to become available")
			backups, err := waitForBackups(c.Context, c.String("airgap-bundle") != "")
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
			if err := setECRestoreState(c.Context, ecRestoreStateRestoreECInstall, backupToRestore.Name); err != nil {
				return fmt.Errorf("unable to set restore state: %w", err)
			}
			logrus.Debugf("restoring embedded cluster installation from backup %q", backupToRestore.Name)
			if err := restoreFromBackup(c.Context, backupToRestore, disasterRecoveryComponentECInstall); err != nil {
				return err
			}
			fallthrough

		case ecRestoreStateRestoreAdminConsole:
			logrus.Debugf("setting restore state to %q", ecRestoreStateRestoreAdminConsole)
			if err := setECRestoreState(c.Context, ecRestoreStateRestoreAdminConsole, backupToRestore.Name); err != nil {
				return fmt.Errorf("unable to set restore state: %w", err)
			}
			logrus.Debugf("restoring admin console from backup %q", backupToRestore.Name)
			if err := restoreFromBackup(c.Context, backupToRestore, disasterRecoveryComponentAdminConsole); err != nil {
				return err
			}
			fallthrough

		case ecRestoreStateWaitForNodes:
			logrus.Debugf("setting restore state to %q", ecRestoreStateWaitForNodes)
			if err := setECRestoreState(c.Context, ecRestoreStateWaitForNodes, backupToRestore.Name); err != nil {
				return fmt.Errorf("unable to set restore state: %w", err)
			}
			logrus.Debugf("checking if backup is high availability")
			highAvailability, err := isHighAvailabilityBackup(backupToRestore)
			if err != nil {
				return err
			}
			logrus.Debugf("waiting for additional nodes to be added")
			if err := waitForAdditionalNodes(c.Context, highAvailability); err != nil {
				return err
			}
			fallthrough

		case ecRestoreStateRestoreSeaweedFS:
			// only restore seaweedfs in case of high availability and airgap
			highAvailability, err := isHighAvailabilityBackup(backupToRestore)
			if err != nil {
				return err
			}
			if highAvailability && c.String("airgap-bundle") != "" {
				logrus.Debugf("setting restore state to %q", ecRestoreStateRestoreSeaweedFS)
				if err := setECRestoreState(c.Context, ecRestoreStateRestoreSeaweedFS, backupToRestore.Name); err != nil {
					return fmt.Errorf("unable to set restore state: %w", err)
				}
				logrus.Debugf("restoring seaweedfs from backup %q", backupToRestore.Name)
				if err := restoreFromBackup(c.Context, backupToRestore, disasterRecoveryComponentSeaweedFS); err != nil {
					return err
				}
			}
			fallthrough

		case ecRestoreStateRestoreRegistry:
			// only restore registry in case of airgap
			if c.String("airgap-bundle") != "" {
				logrus.Debugf("setting restore state to %q", ecRestoreStateRestoreRegistry)
				if err := setECRestoreState(c.Context, ecRestoreStateRestoreRegistry, backupToRestore.Name); err != nil {
					return fmt.Errorf("unable to set restore state: %w", err)
				}
				logrus.Debugf("restoring embedded cluster registry from backup %q", backupToRestore.Name)
				if err := restoreFromBackup(c.Context, backupToRestore, disasterRecoveryComponentRegistry); err != nil {
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
			if err := setECRestoreState(c.Context, ecRestoreStateRestoreECO, backupToRestore.Name); err != nil {
				return fmt.Errorf("unable to set restore state: %w", err)
			}
			logrus.Debugf("restoring embedded cluster operator from backup %q", backupToRestore.Name)
			if err := restoreFromBackup(c.Context, backupToRestore, disasterRecoveryComponentECO); err != nil {
				return err
			}
			fallthrough

		case ecRestoreStateRestoreApp:
			logrus.Debugf("setting restore state to %q", ecRestoreStateRestoreApp)
			if err := setECRestoreState(c.Context, ecRestoreStateRestoreApp, backupToRestore.Name); err != nil {
				return fmt.Errorf("unable to set restore state: %w", err)
			}
			logrus.Debugf("restoring app from backup %q", backupToRestore.Name)
			if err := restoreFromBackup(c.Context, backupToRestore, disasterRecoveryComponentApp); err != nil {
				return err
			}
			logrus.Debugf("resetting restore state")
			if err := resetECRestoreState(c.Context); err != nil {
				return fmt.Errorf("unable to reset restore state: %w", err)
			}

		default:
			return fmt.Errorf("unknown restore state: %q", state)
		}

		return nil
	},
}
