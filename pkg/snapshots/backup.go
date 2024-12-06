package snapshots

import (
	"context"
	"fmt"
	"strconv"

	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// InstanceBackupNameLabel is the label used to store the name of the backup for an instance
	// backup. This property is used to group backups together.
	InstanceBackupNameLabel = "replicated.com/backup-name"
	// InstanceBackupTypeAnnotation is the annotation used to store the type of backup for an
	// instance backup. This can be either "infra", "app", or "legacy".
	InstanceBackupTypeAnnotation = "replicated.com/backup-type"
	// InstanceBackupCountAnnotation is the annotation used to store the expected number of backups
	// for an instance backup. This is expected to be 1 for legacy and otherwise 2.
	InstanceBackupCountAnnotation = "replicated.com/backup-count"

	// InstanceBackupTypeInfra indicates that the backup is of type infrastructure.
	InstanceBackupTypeInfra = "infra"
	// InstanceBackupTypeApp indicates that the backup is of type application.
	InstanceBackupTypeApp = "app"
	// InstanceBackupTypeLegacy indicates that the backup is of type legacy (combined infra + app).
	InstanceBackupTypeLegacy = "legacy"
)

// ReplicatedBackup represents one or more velero backups that make up a single instance backup.
// Legacy backups will be represented as a single, combined backup. New backups will contain both a
// infrastructure backup as well as an application backup.
type ReplicatedBackup []velerov1.Backup

// ListReplicatedBackups returns a list of ReplicatedBackup backups.
func ListReplicatedBackups(ctx context.Context, kcli client.Client) ([]ReplicatedBackup, error) {
	backups, err := listBackups(ctx, kcli, runtimeconfig.VeleroNamespace, "")
	if err != nil {
		return nil, err
	}
	return groupBackupsByName(backups), nil
}

// GetReplicatedBackup returns a ReplicatedBackup object for the specified backup name.
func GetReplicatedBackup(ctx context.Context, cli client.Client, veleroNamespace string, backupName string) (ReplicatedBackup, error) {
	backups, err := getBackupsFromName(ctx, cli, veleroNamespace, backupName)
	if err != nil {
		return nil, err
	}
	replicatedBackups := groupBackupsByName(backups)
	if len(replicatedBackups) != 1 {
		return ReplicatedBackup{}, fmt.Errorf("expected 1 backup, got %d", len(replicatedBackups))
	}
	return replicatedBackups[0], nil
}

// GetName returns the name of the backup stored in the velero backup object label.
func (b ReplicatedBackup) GetName() string {
	var name string
	for _, a := range b {
		name = a.Name
		if val := a.Annotations[InstanceBackupNameLabel]; val != "" {
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
	if backup == nil {
		return 1
	}
	return getInstanceBackupCount(*backup)
}

// GetCreationTimestamp returns the creation timestamp of the velero infra backup object.
func (b ReplicatedBackup) GetCreationTimestamp() metav1.Time {
	backup := b.GetInfraBackup()
	if backup == nil {
		return metav1.Time{}
	}
	return backup.GetCreationTimestamp()
}

// GetAnnotation returns the value of the specified annotation key from the velero infra backup
// object.
func (b ReplicatedBackup) GetAnnotation(key string) (string, bool) {
	backup := b.GetInfraBackup()
	if backup == nil {
		return "", false
	}
	val, ok := backup.Annotations[key]
	return val, ok
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
		if backup.Annotations["kots.io/embedded-cluster"] != "true" {
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

func listBackups(ctx context.Context, cli client.Client, veleroNamespace string, backupName string) ([]velerov1.Backup, error) {
	// try to get the restore from the backup name label
	backups := &velerov1api.BackupList{}
	err := cli.List(ctx, backups, client.InNamespace(veleroNamespace))
	if err != nil {
		return nil, fmt.Errorf("unable to list backups: %w", err)
	}

	return backups.Items, nil
}

func getBackupsFromName(ctx context.Context, cli client.Client, veleroNamespace string, backupName string) ([]velerov1.Backup, error) {
	// try to get the restore from the backup name label
	backups := &velerov1api.BackupList{}
	err := cli.List(ctx, backups, client.InNamespace(veleroNamespace))
	if err != nil {
		return nil, fmt.Errorf("unable to list backups: %w", err)
	}
	if len(backups.Items) > 0 {
		return backups.Items, nil
	}
	backup := &velerov1api.Backup{}
	err = cli.Get(ctx, types.NamespacedName{Name: backupName, Namespace: veleroNamespace}, backup)
	if err != nil {
		return nil, fmt.Errorf("unable to get backup: %w", err)
	}

	return []velerov1.Backup{*backup}, nil
}
