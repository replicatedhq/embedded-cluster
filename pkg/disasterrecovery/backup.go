package disasterrecovery

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// BackupIsECAnnotation is the annotation used to store if the backup is from an EC install.
	BackupIsECAnnotation = "kots.io/embedded-cluster"

	// InstanceBackupAnnotation is the annotation used to indicate that a backup is a legacy
	// instance backup.
	InstanceBackupAnnotation = "kots.io/instance"

	// InstanceBackupVersionAnnotation is the annotation used to store the version of the backup
	// for an instance (DR) backup.
	InstanceBackupVersionAnnotation = "replicated.com/disaster-recovery-version"
	// InstanceBackupVersion1 indicates that the backup is of version 1.
	InstanceBackupVersion1 = "1"
	// InstanceBackupVersionCurrent is the current backup version. When future breaking changes are
	// introduced, we can increment this number on backup creation.
	InstanceBackupVersionCurrent = InstanceBackupVersion1

	// InstanceBackupNameLabel is the label used to store the name of the backup for an instance
	// backup. This property is used to group backups together.
	InstanceBackupNameLabel = "replicated.com/backup-name"
	// InstanceBackupTypeAnnotation is the annotation used to store the type of backup for an
	// instance backup. This can be either "infra", "app", or "legacy".
	InstanceBackupTypeAnnotation = "replicated.com/backup-type"
	// InstanceBackupCountAnnotation is the annotation used to store the expected number of backups
	// for an instance backup. This is expected to be 1 for legacy and otherwise 2.
	InstanceBackupCountAnnotation = "replicated.com/backup-count"
	// InstanceBackupResoreSpecAnnotation is the annotation used to store the restore spec for an
	// instance backup.
	InstanceBackupResoreSpecAnnotation = "replicated.com/restore-spec"

	// InstanceBackupTypeInfra indicates that the backup is of type infrastructure.
	InstanceBackupTypeInfra = "infra"
	// InstanceBackupTypeApp indicates that the backup is of type application.
	InstanceBackupTypeApp = "app"
	// InstanceBackupTypeLegacy indicates that the backup is of type legacy (combined infra + app).
	InstanceBackupTypeLegacy = "legacy"
)

var (
	// ErrBackupNotFound is returned from the GetReplicatedBackup function when the backup is not found.
	ErrBackupNotFound = errors.New("backup not found")
)

// ReplicatedBackups implements sort.Interface for []ReplicatedBackup based on the
// Status.StartTimestamp of the infrastructure backup.
type ReplicatedBackups []ReplicatedBackup

func (a ReplicatedBackups) Len() int      { return len(a) }
func (a ReplicatedBackups) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ReplicatedBackups) Less(i, j int) bool {
	var iTime, jTime time.Time
	if a[i].GetInfraBackup() != nil && a[i].GetInfraBackup().Status.StartTimestamp != nil {
		iTime = a[i].GetInfraBackup().Status.StartTimestamp.Time
	} else {
		for _, backup := range a[i] {
			if backup.Status.StartTimestamp != nil {
				iTime = backup.Status.StartTimestamp.Time
				break
			}
		}
	}
	if a[j].GetInfraBackup() != nil && a[j].GetInfraBackup().Status.StartTimestamp != nil {
		jTime = a[j].GetInfraBackup().Status.StartTimestamp.Time
	} else {
		for _, backup := range a[j] {
			if backup.Status.StartTimestamp != nil {
				jTime = backup.Status.StartTimestamp.Time
				break
			}
		}
	}
	return iTime.Before(jTime)
}

// ReplicatedBackup represents one or more velero backups that make up a single instance backup.
// Legacy backups will be represented as a single, combined backup. New backups will contain both a
// infrastructure backup as well as an application backup.
type ReplicatedBackup []velerov1.Backup

// ListReplicatedBackups returns a sorted list of ReplicatedBackup backups by creation timestamp.
func ListReplicatedBackups(ctx context.Context, cli client.Client) ([]ReplicatedBackup, error) {
	backups, err := listBackups(ctx, cli, constants.VeleroNamespace)
	if err != nil {
		return nil, err
	}
	replicatedBackups := groupBackupsByName(backups)
	sort.Sort(ReplicatedBackups(replicatedBackups))
	return replicatedBackups, nil
}

// GetReplicatedBackup returns a ReplicatedBackup object for the specified backup name.
func GetReplicatedBackup(ctx context.Context, cli client.Client, veleroNamespace string, backupName string) (ReplicatedBackup, error) {
	backups, err := getBackupsFromName(ctx, cli, veleroNamespace, backupName)
	if err != nil {
		return nil, err
	}
	replicatedBackups := groupBackupsByName(backups)
	if len(replicatedBackups) == 0 {
		return nil, ErrBackupNotFound
	} else if len(replicatedBackups) != 1 {
		return ReplicatedBackup{}, fmt.Errorf("expected 1 backup, got %d", len(replicatedBackups))
	}
	return replicatedBackups[0], nil
}

// GetName returns the name of the backup stored in the velero backup object label.
func (b ReplicatedBackup) GetName() string {
	var name string
	for _, a := range b {
		name = a.Name
		if val := a.Labels[InstanceBackupNameLabel]; val != "" {
			return val
		}
	}
	return name
}

// GetInfraBackup returns the infrastructure backup for the instance backup or nil if it does not
// exist.
func (b ReplicatedBackup) GetInfraBackup() *velerov1.Backup {
	for _, a := range b {
		if GetInstanceBackupType(a) == InstanceBackupTypeInfra || GetInstanceBackupType(a) == InstanceBackupTypeLegacy {
			return &a
		}
	}
	return nil
}

// GetAppBackup returns the application backup for the instance backup or nil if it does not exist.
func (b ReplicatedBackup) GetAppBackup() *velerov1.Backup {
	for _, a := range b {
		if GetInstanceBackupType(a) == InstanceBackupTypeApp || GetInstanceBackupType(a) == InstanceBackupTypeLegacy {
			return &a
		}
	}
	return nil
}

// GetExpectedBackupCount returns the expected number of backups for the instance backup. This is
// expected to be 1 for legacy backups and otherwise 2.
func (b ReplicatedBackup) GetExpectedBackupCount() int {
	backup := b.GetInfraBackup()
	if backup != nil {
		return getInstanceBackupCount(*backup)
	}
	// if the infra backup is not found, we return the count of the first backup
	for _, backup := range b {
		return getInstanceBackupCount(backup)
	}
	return 0
}

// GetRestore returns the restore CR from the annotation of the velero app backup.
func (b ReplicatedBackup) GetRestore() (*velerov1.Restore, error) {
	backup := b.GetAppBackup()
	if backup == nil {
		return nil, fmt.Errorf("no app backup found")
	}
	if val, ok := backup.Annotations[InstanceBackupResoreSpecAnnotation]; ok {
		decode := kubeutils.Codecs.UniversalDeserializer().Decode
		obj, gvk, err := decode([]byte(val), nil, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to decode restore spec: %w", err)
		}
		if gvk.String() != "velero.io/v1, Kind=Restore" {
			return nil, fmt.Errorf("unexpected gvk: %s", gvk.String())
		}
		return obj.(*velerov1.Restore), nil
	}
	return nil, fmt.Errorf("missing restore spec annotation")
}

// GetCompletionTimestamp returns the completion timestamp of the last velero backup to be
// completed or zero if the expected backup count is not met or any of the backups in the slice is
// not completed.
func (b ReplicatedBackup) GetCompletionTimestamp() metav1.Time {
	if b.GetExpectedBackupCount() != len(b) {
		return metav1.Time{}
	}
	var completionTimestamp metav1.Time
	for _, backup := range b {
		if backup.Status.CompletionTimestamp == nil {
			return metav1.Time{}
		} else if backup.Status.CompletionTimestamp.Time.After(completionTimestamp.Time) {
			completionTimestamp = *backup.Status.CompletionTimestamp
		}
	}
	return completionTimestamp
}

// GetAnnotation returns the value of the specified annotation key from the velero infra backup
// object or the first backup in the slice if the infra backup is not found.
func (b ReplicatedBackup) GetAnnotation(key string) (string, bool) {
	backup := b.GetInfraBackup()
	if backup != nil {
		val, ok := backup.Annotations[key]
		return val, ok
	}
	// if the infra backup is not found, we return the annotation value of the first backup
	for _, backup := range b {
		val, ok := backup.Annotations[key]
		return val, ok
	}
	return "", false
}

// IsInstanceBackup returns true if the backup is an instance backup.
func IsInstanceBackup(veleroBackup velerov1.Backup) bool {
	if GetInstanceBackupVersion(veleroBackup) != "" {
		return true
	}
	if val, ok := veleroBackup.GetAnnotations()[InstanceBackupAnnotation]; ok {
		return val == "true"
	}
	return false
}

// GetInstanceBackupVersion returns the version of the backup from the velero backup object
// annotation.
func GetInstanceBackupVersion(veleroBackup velerov1.Backup) string {
	if val, ok := veleroBackup.GetAnnotations()[InstanceBackupVersionAnnotation]; ok {
		return val
	}
	return ""
}

// GetInstanceBackupType returns the type of the backup from the velero backup object annotation.
// This can be either "infra", "app", or "legacy".
func GetInstanceBackupType(backup velerov1.Backup) string {
	if val, ok := backup.GetAnnotations()[InstanceBackupTypeAnnotation]; ok {
		return val
	}
	return InstanceBackupTypeLegacy
}

// getBackupName returns the name of the backup from the velero backup object label. This property
// is used to group backups together.
func getBackupName(backup velerov1.Backup) string {
	if val, ok := backup.GetLabels()[InstanceBackupNameLabel]; ok {
		return val
	}
	return backup.GetName()
}

// getInstanceBackupCount returns the expected number of backups from the velero backup object
// annotation. This is expected to be 1 for legacy backups and otherwise 2.
func getInstanceBackupCount(backup velerov1.Backup) int {
	if val, ok := backup.GetAnnotations()[InstanceBackupCountAnnotation]; ok {
		num, _ := strconv.Atoi(val)
		if num > 0 {
			return num
		}
	}
	return 1
}

func groupBackupsByName(backups []velerov1.Backup) []ReplicatedBackup {
	groupedBackups := []ReplicatedBackup{}
	for _, backup := range backups {
		// this is not a replicated backup
		if backup.Annotations[BackupIsECAnnotation] != "true" {
			continue
		}
		if !IsInstanceBackup(backup) {
			continue
		}
		found := false
		backupName := getBackupName(backup)
		for i := range groupedBackups {
			if groupedBackups[i].GetName() == backupName {
				groupedBackups[i] = append(groupedBackups[i], backup)
				found = true
				break
			}
		}
		if !found {
			groupedBackups = append(groupedBackups, ReplicatedBackup{backup})
		}
	}
	return groupedBackups
}

func listBackups(ctx context.Context, cli client.Client, veleroNamespace string) ([]velerov1.Backup, error) {
	backups := &velerov1.BackupList{}
	err := cli.List(ctx, backups, client.InNamespace(veleroNamespace))
	if err != nil {
		return nil, fmt.Errorf("unable to list backups: %w", err)
	}

	return backups.Items, nil
}

func getBackupsFromName(ctx context.Context, cli client.Client, veleroNamespace string, backupName string) ([]velerov1.Backup, error) {
	// first try to get the backup from the backup-name label
	backups := &velerov1.BackupList{}
	err := cli.List(ctx, backups,
		client.InNamespace(veleroNamespace),
		client.MatchingLabels{InstanceBackupNameLabel: backupName},
	)
	if err != nil {
		return nil, fmt.Errorf("unable to list backups: %w", err)
	}
	if len(backups.Items) > 0 {
		return backups.Items, nil
	}
	backup := &velerov1.Backup{}
	err = cli.Get(ctx, types.NamespacedName{Name: backupName, Namespace: veleroNamespace}, backup)
	if k8serrors.IsNotFound(err) {
		return nil, ErrBackupNotFound
	} else if err != nil {
		return nil, fmt.Errorf("unable to get backup: %w", err)
	}

	return []velerov1.Backup{*backup}, nil
}
