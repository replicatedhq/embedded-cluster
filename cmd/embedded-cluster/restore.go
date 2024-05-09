package main

import (
	"context"
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
	"k8s.io/utils/ptr"
	k8sconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	k8syaml "sigs.k8s.io/yaml"
)

type s3BackupStore struct {
	endpoint        string
	region          string
	bucket          string
	prefix          string
	accessKeyID     string
	secretAccessKey string
}

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
	store.accessKeyID = prompts.New().Password("Access key ID:")
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
	os.Setenv("KUBECONFIG", defaults.PathToKubeConfig())
	opts := []addons.Option{}
	return addons.NewApplier(opts...).OutroForRestore(c.Context)
}

func isBackupRestorable(backup *velerov1.Backup, rel *release.ChannelRelease, isAirgap bool) (bool, string) {
	if backup.Annotations["kots.io/embedded-cluster"] != "true" {
		return false, "is not an embedded cluster backup"
	}
	if v := backup.Annotations["kots.io/embedded-cluster-version"]; v != defaults.Version {
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

// waitForBackups waits for backups to become available.
// It returns a list of restorable backups, or an error if none are found.
func waitForBackups(c *cli.Context) ([]velerov1.Backup, error) {
	ctx := c.Context

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
			restorable, reason := isBackupRestorable(&backup, rel, c.String("airgap-bundle") != "")
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

// waitForRestoreCompleted waits for a Velero restore to complete.
func waitForRestoreCompleted(ctx context.Context, restoreName string) (*velerov1.Restore, error) {
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

type DisasterRecoveryComponent string

const (
	DisasterRecoveryComponentInfra     DisasterRecoveryComponent = "infra"
	DisasterRecoveryComponentECInstall DisasterRecoveryComponent = "ec-install"
	DisasterRecoveryComponentApp       DisasterRecoveryComponent = "app"
	DisasterRecoveryComponentChart     DisasterRecoveryComponent = "chart"
)

// restoreFromBackup restores a disaster recovery component from a backup.
func restoreFromBackup(ctx context.Context, backup *velerov1.Backup, drComponent DisasterRecoveryComponent, chartName string, restoreLabelSelector *metav1.LabelSelector) error {
	loading := spinner.Start()
	defer loading.Close()

	switch drComponent {
	case DisasterRecoveryComponentInfra:
		loading.Infof("Restoring infrastructure")
	case DisasterRecoveryComponentECInstall:
		loading.Infof("Restoring cluster state")
	case DisasterRecoveryComponentApp:
		loading.Infof("Restoring application")
	case DisasterRecoveryComponentChart:
		loading.Infof("Restoring %s", chartName)
	}

	cfg, err := k8sconfig.GetConfig()
	if err != nil {
		return fmt.Errorf("unable to get kubernetes config: %w", err)
	}

	veleroClient, err := veleroclientv1.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("unable to create velero client: %w", err)
	}

	if drComponent != DisasterRecoveryComponentChart {
		restoreLabelSelector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"replicated.com/disaster-recovery": string(drComponent),
			},
		}
	}

	// define the restore object
	restore := &velerov1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: defaults.VeleroNamespace,
			Name:      fmt.Sprintf("%s.%s", backup.Name, string(drComponent)),
			Annotations: map[string]string{
				"kots.io/embedded-cluster": "true",
			},
		},
		Spec: velerov1.RestoreSpec{
			BackupName:              backup.Name,
			LabelSelector:           restoreLabelSelector,
			RestorePVs:              ptr.To(true),
			IncludeClusterResources: ptr.To(true),
		},
	}

	// delete existing restore object (if exists)
	err = veleroClient.Restores(defaults.VeleroNamespace).Delete(ctx, restore.Name, metav1.DeleteOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("unable to delete restore %s: %w", restore.Name, err)
	}

	// create new restore object
	restore, err = veleroClient.Restores(defaults.VeleroNamespace).Create(ctx, restore, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("unable to create restore: %w", err)
	}

	// wait for restore to complete
	restore, err = waitForRestoreCompleted(ctx, restore.Name)
	if err != nil {
		if restore != nil {
			return fmt.Errorf("restore failed with %d errors and %d warnings.: %w", restore.Status.Errors, restore.Status.Warnings, err)
		}
		return fmt.Errorf("unable to wait for velero restore to complete: %w", err)
	}

	// wait for embedded cluster installation to reconcile
	if drComponent == DisasterRecoveryComponentECInstall {
		kcli, err := kubeutils.KubeClient()
		if err != nil {
			return fmt.Errorf("unable to create kube client: %w", err)
		}
		if err := kubeutils.WaitForInstallation(ctx, kcli, loading); err != nil {
			return fmt.Errorf("unable to wait for installation to be ready: %w", err)
		}
	}

	switch drComponent {
	case DisasterRecoveryComponentInfra:
		loading.Infof("Infrastructure restored!")
	case DisasterRecoveryComponentECInstall:
		loading.Infof("Cluster state restored!")
	case DisasterRecoveryComponentApp:
		loading.Infof("Application restored!")
	case DisasterRecoveryComponentChart:
		loading.Infof("%s restored!", strings.ToUpper(chartName[:1])+chartName[1:])
	}

	return nil
}

func waitForAdditionalNodes(ctx context.Context) error {
	// the admin console detects this config map and redirects the user to the cluster management page
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}
	waitForNodesCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "embedded-cluster-wait-for-nodes",
			Namespace: "embedded-cluster",
		},
		Data: map[string]string{},
	}
	if err := kcli.Create(ctx, waitForNodesCM); err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("unable to create wait-for-nodes config map: %w", err)
	}
	defer func() {
		if err := kcli.Delete(ctx, waitForNodesCM); err != nil && !errors.IsNotFound(err) {
			logrus.Errorf("unable to delete wait-for-nodes config map: %v", err)
		}
	}()

	loading := spinner.Start()
	loading.Infof("Waiting for Admin Console to deploy")
	if err := adminconsole.WaitForReady(ctx, kcli, defaults.KotsadmNamespace, loading); err != nil {
		loading.Close()
		return fmt.Errorf("unable to wait for admin console: %w", err)
	}
	loading.Closef("Admin Console is ready!")

	successColor := "\033[32m"
	colorReset := "\033[0m"
	joinNodesMsg := fmt.Sprintf("\nVisit the admin console if you need to add nodes to the cluster: %s%s%s\n",
		successColor, adminconsole.GetURL(), colorReset,
	)
	logrus.Info(joinNodesMsg)

	for {
		p := prompts.New().Input("Type 'continue' when you are done adding nodes:", "", false)
		if p == "continue" {
			logrus.Info("")
			break
		}
		logrus.Info("Please type 'continue' to proceed")
	}

	loading = spinner.Start()
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
			Usage:  "Path to the airgap bundle. If set, the installation will be completed without internet access.",
			Hidden: true,
		},
	},
	Before: func(c *cli.Context) error {
		if os.Getuid() != 0 {
			return fmt.Errorf("restore command must be run as root")
		}
		return nil
	},
	Action: func(c *cli.Context) error {
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
		if c.String("airgap-bundle") != "" {
			logrus.Debugf("checking airgap bundle matches binary")
			if err := checkAirgapMatches(c); err != nil {
				return err // we want the user to see the error message without a prefix
			}
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
		logrus.Debugf("installing k0s")
		if err := installK0s(); err != nil {
			return fmt.Errorf("unable update cluster: %w", err)
		}
		logrus.Debugf("running post install")
		if err := runPostInstall(); err != nil {
			return fmt.Errorf("unable to run post install: %w", err)
		}
		logrus.Debugf("waiting for k0s to be ready")
		if err := waitForK0s(); err != nil {
			return fmt.Errorf("unable to wait for node: %w", err)
		}
		logrus.Debugf("running outro")
		if err := runOutroForRestore(c); err != nil {
			return fmt.Errorf("unable to run outro: %w", err)
		}

		logrus.Debugf("configuring backup storage location")
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

		logrus.Debugf("waiting for backups to become available")
		backups, err := waitForBackups(c)
		if err != nil {
			return err
		}

		logrus.Debugf("picking backup to restore")
		backup := pickBackupToRestore(backups)
		if backup == nil {
			return fmt.Errorf("no backups are candidates for restore")
		}

		logrus.Info("")
		completionTimestamp := backup.Status.CompletionTimestamp.Time.Format("2006-01-02 15:04:05 UTC")
		shouldRestore := prompts.New().Confirm(fmt.Sprintf("Restore from backup %q (%s)?", backup.Name, completionTimestamp), true)
		logrus.Info("")
		if !shouldRestore {
			logrus.Infof("Aborting restore...")
			return nil
		}

		logrus.Debugf("restoring infra from backup %q", backup.Name)
		if err := restoreFromBackup(c.Context, backup, DisasterRecoveryComponentInfra, "", nil); err != nil {
			return err
		}

		if c.String("airgap-bundle") != "" {
			logrus.Debugf("restoring embedded cluster registry from backup %q", backup.Name)
			err := restoreFromBackup(c.Context, backup, DisasterRecoveryComponentChart, "registry", &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "docker-registry",
				},
			})
			if err != nil {
				return err
			}
		}

		logrus.Debugf("restoring embedded cluster installation from backup %q", backup.Name)
		if err := restoreFromBackup(c.Context, backup, DisasterRecoveryComponentECInstall, "", nil); err != nil {
			return err
		}

		logrus.Debugf("waiting for additional nodes to be added")
		if err := waitForAdditionalNodes(c.Context); err != nil {
			return err
		}

		logrus.Debugf("restoring app from backup %q", backup.Name)
		if err := restoreFromBackup(c.Context, backup, DisasterRecoveryComponentApp, "", nil); err != nil {
			return err
		}

		return nil
	},
}
