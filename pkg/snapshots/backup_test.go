package snapshots

import (
	"testing"
	"time"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReplicatedBackups_Less(t *testing.T) {
	newBackup := func() velerov1.Backup {
		return velerov1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "backup",
				Annotations: map[string]string{
					InstanceBackupTypeAnnotation: InstanceBackupTypeInfra,
				},
			},
		}
	}

	withInfraType := func(backup velerov1.Backup, infraType string) velerov1.Backup {
		backup.Annotations[InstanceBackupTypeAnnotation] = infraType
		return backup
	}

	withCreationTimestamp := func(backup velerov1.Backup, t time.Time) velerov1.Backup {
		backup.CreationTimestamp = metav1.Time{Time: t}
		return backup
	}

	tests := []struct {
		name string
		a    ReplicatedBackups
		want bool
	}{
		{
			name: "greater",
			a: ReplicatedBackups{
				{
					withCreationTimestamp(newBackup(), time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)),
				},
				{
					withCreationTimestamp(newBackup(), time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
			},
			want: false,
		},
		{
			name: "less",
			a: ReplicatedBackups{
				{
					withCreationTimestamp(newBackup(), time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
				{
					withCreationTimestamp(newBackup(), time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)),
				},
			},
			want: true,
		},
		{
			name: "equal",
			a: ReplicatedBackups{
				{
					withCreationTimestamp(newBackup(), time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
				{
					withCreationTimestamp(newBackup(), time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
			},
			want: false,
		},
		{
			name: "greater no infra",
			a: ReplicatedBackups{
				{
					withCreationTimestamp(withInfraType(newBackup(), InstanceBackupTypeApp), time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)),
				},
				{
					withCreationTimestamp(newBackup(), time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
			},
			want: false,
		},
		{
			name: "less no infra",
			a: ReplicatedBackups{
				{
					withCreationTimestamp(withInfraType(newBackup(), InstanceBackupTypeApp), time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
				{
					withCreationTimestamp(newBackup(), time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)),
				},
			},
			want: true,
		},
		{
			name: "i no backups should be less",
			a: ReplicatedBackups{
				{},
				{
					withCreationTimestamp(newBackup(), time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
			},
			want: true,
		},
		{
			name: "j no backups should be greater",
			a: ReplicatedBackups{
				{
					withCreationTimestamp(newBackup(), time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
				{},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Less(0, 1); got != tt.want {
				t.Errorf("ReplicatedBackups.Less() = %v, want %v", got, tt.want)
			}
		})
	}
}
