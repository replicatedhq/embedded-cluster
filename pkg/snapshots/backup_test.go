package snapshots

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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

func TestListReplicatedBackups(t *testing.T) {
	scheme := scheme.Scheme
	velerov1.AddToScheme(scheme)

	type args struct {
		cli client.Client
	}
	tests := []struct {
		name    string
		args    args
		want    []ReplicatedBackup
		wantErr bool
	}{
		{
			name: "no backups should return an empty list",
			args: args{
				cli: fake.NewClientBuilder().WithScheme(scheme).Build(),
			},
			want:    []ReplicatedBackup{},
			wantErr: false,
		},
		{
			name: "a mix of legacy and new backups should be grouped",
			args: args{
				cli: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
					&velerov1.Backup{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Backup",
							APIVersion: "velero.io/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "instance-abcd",
							Namespace: "velero",
							Labels: map[string]string{
								InstanceBackupNameLabel: "app-slug-abcd",
							},
							Annotations: map[string]string{
								BackupIsECAnnotation:          "true",
								InstanceBackupTypeAnnotation:  InstanceBackupTypeInfra,
								InstanceBackupCountAnnotation: "2",
							},
							CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)},
						},
					},
					&velerov1.Backup{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Backup",
							APIVersion: "velero.io/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "application-abcd",
							Namespace: "velero",
							Labels: map[string]string{
								InstanceBackupNameLabel: "app-slug-abcd",
							},
							Annotations: map[string]string{
								BackupIsECAnnotation:          "true",
								InstanceBackupTypeAnnotation:  InstanceBackupTypeApp,
								InstanceBackupCountAnnotation: "2",
							},
							CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 4, 0, 0, 0, 0, time.UTC)},
						},
					},
					&velerov1.Backup{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Backup",
							APIVersion: "velero.io/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "instance-efgh",
							Namespace: "velero",
							Annotations: map[string]string{
								BackupIsECAnnotation: "true",
								// legacy backups do not have the InstanceBackupTypeAnnotation
							},
							CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)},
						},
					},
					&velerov1.Backup{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Backup",
							APIVersion: "velero.io/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:        "not-ec",
							Namespace:   "velero",
							Annotations: map[string]string{
								// EC backups need the kots.io/embedded-cluster annotation
							},
							CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)},
						},
					},
				).Build(),
			},
			want: []ReplicatedBackup{
				{
					{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Backup",
							APIVersion: "velero.io/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "instance-efgh",
							Namespace: "velero",
							Annotations: map[string]string{
								BackupIsECAnnotation: "true",
							},
							CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)},
							ResourceVersion:   "999",
						},
					},
				},
				{
					{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Backup",
							APIVersion: "velero.io/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "application-abcd",
							Namespace: "velero",
							Labels: map[string]string{
								InstanceBackupNameLabel: "app-slug-abcd",
							},
							Annotations: map[string]string{
								BackupIsECAnnotation:          "true",
								InstanceBackupTypeAnnotation:  InstanceBackupTypeApp,
								InstanceBackupCountAnnotation: "2",
							},
							CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 4, 0, 0, 0, 0, time.UTC)},
							ResourceVersion:   "999",
						},
					},
					{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Backup",
							APIVersion: "velero.io/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "instance-abcd",
							Namespace: "velero",
							Labels: map[string]string{
								InstanceBackupNameLabel: "app-slug-abcd",
							},
							Annotations: map[string]string{
								BackupIsECAnnotation:          "true",
								InstanceBackupTypeAnnotation:  InstanceBackupTypeInfra,
								InstanceBackupCountAnnotation: "2",
							},
							CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)},
							ResourceVersion:   "999",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ListReplicatedBackups(context.Background(), tt.args.cli)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetReplicatedBackup(t *testing.T) {
	scheme := scheme.Scheme
	velerov1.AddToScheme(scheme)

	objects := []client.Object{
		&velerov1.Backup{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Backup",
				APIVersion: "velero.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "instance-abcd",
				Namespace: "velero",
				Labels: map[string]string{
					InstanceBackupNameLabel: "app-slug-abcd",
				},
				Annotations: map[string]string{
					BackupIsECAnnotation:          "true",
					InstanceBackupTypeAnnotation:  InstanceBackupTypeInfra,
					InstanceBackupCountAnnotation: "2",
				},
				CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)},
			},
		},
		&velerov1.Backup{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Backup",
				APIVersion: "velero.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "application-abcd",
				Namespace: "velero",
				Labels: map[string]string{
					InstanceBackupNameLabel: "app-slug-abcd",
				},
				Annotations: map[string]string{
					BackupIsECAnnotation:          "true",
					InstanceBackupTypeAnnotation:  InstanceBackupTypeApp,
					InstanceBackupCountAnnotation: "2",
				},
				CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 4, 0, 0, 0, 0, time.UTC)},
			},
		},
		&velerov1.Backup{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Backup",
				APIVersion: "velero.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "instance-efgh",
				Namespace: "velero",
				Annotations: map[string]string{
					BackupIsECAnnotation: "true",
					// legacy backups do not have the InstanceBackupTypeAnnotation
				},
				CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)},
			},
		},
		&velerov1.Backup{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Backup",
				APIVersion: "velero.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        "not-ec",
				Namespace:   "velero",
				Annotations: map[string]string{
					// EC backups need the kots.io/embedded-cluster annotation
				},
				CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)},
			},
		},
	}

	type args struct {
		cli             client.Client
		veleroNamespace string
		backupName      string
	}
	tests := []struct {
		name    string
		args    args
		want    ReplicatedBackup
		wantErr error
	}{
		{
			name: "legacy should return a single backup from metadata.name",
			args: args{
				cli:             fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build(),
				veleroNamespace: "velero",
				backupName:      "instance-efgh",
			},
			want: ReplicatedBackup{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "instance-efgh",
						Namespace: "velero",
						Annotations: map[string]string{
							BackupIsECAnnotation: "true",
							// legacy backups do not have the InstanceBackupTypeAnnotation
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "improved dr should return two backups from label",
			args: args{
				cli:             fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build(),
				veleroNamespace: "velero",
				backupName:      "app-slug-abcd",
			},
			want: ReplicatedBackup{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "application-abcd",
						Namespace: "velero",
						Labels: map[string]string{
							InstanceBackupNameLabel: "app-slug-abcd",
						},
						Annotations: map[string]string{
							BackupIsECAnnotation:          "true",
							InstanceBackupTypeAnnotation:  InstanceBackupTypeApp,
							InstanceBackupCountAnnotation: "2",
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 4, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "instance-abcd",
						Namespace: "velero",
						Labels: map[string]string{
							InstanceBackupNameLabel: "app-slug-abcd",
						},
						Annotations: map[string]string{
							BackupIsECAnnotation:          "true",
							InstanceBackupTypeAnnotation:  InstanceBackupTypeInfra,
							InstanceBackupCountAnnotation: "2",
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "not found should return an error",
			args: args{
				cli:             fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build(),
				veleroNamespace: "velero",
				backupName:      "not-exists",
			},
			want:    nil,
			wantErr: ErrBackupNotFound,
		},
		{
			name: "not a replicated backup should return an error",
			args: args{
				cli:             fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build(),
				veleroNamespace: "velero",
				backupName:      "not-ec",
			},
			want:    nil,
			wantErr: ErrBackupNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetReplicatedBackup(context.Background(), tt.args.cli, tt.args.veleroNamespace, tt.args.backupName)
			require.Equal(t, tt.wantErr, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReplicatedBackup_GetName(t *testing.T) {
	tests := []struct {
		name string
		b    ReplicatedBackup
		want string
	}{
		{
			name: "legacy backups should return the metadata.name of the backup",
			b: ReplicatedBackup{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "instance-efgh",
						Namespace: "velero",
						Annotations: map[string]string{
							BackupIsECAnnotation: "true",
							// legacy backups do not have the InstanceBackupTypeAnnotation
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
			},
			want: "instance-efgh",
		},
		{
			name: "improved dr backups should return the label name of the backup",
			b: ReplicatedBackup{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "application-abcd",
						Namespace: "velero",
						Labels: map[string]string{
							InstanceBackupNameLabel: "app-slug-abcd",
						},
						Annotations: map[string]string{
							BackupIsECAnnotation:          "true",
							InstanceBackupTypeAnnotation:  InstanceBackupTypeApp,
							InstanceBackupCountAnnotation: "2",
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 4, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "instance-abcd",
						Namespace: "velero",
						Labels: map[string]string{
							InstanceBackupNameLabel: "app-slug-abcd",
						},
						Annotations: map[string]string{
							BackupIsECAnnotation:          "true",
							InstanceBackupTypeAnnotation:  InstanceBackupTypeInfra,
							InstanceBackupCountAnnotation: "2",
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
			},
			want: "app-slug-abcd",
		},
		{
			name: "no backups should return an empty string",
			b:    ReplicatedBackup{},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.b.GetName()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReplicatedBackup_GetInfraBackup(t *testing.T) {
	tests := []struct {
		name string
		b    ReplicatedBackup
		want *velerov1.Backup
	}{
		{
			name: "legacy backups should return the legacy backup",
			b: ReplicatedBackup{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "instance-efgh",
						Namespace: "velero",
						Annotations: map[string]string{
							BackupIsECAnnotation: "true",
							// legacy backups do not have the InstanceBackupTypeAnnotation
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
			},
			want: &velerov1.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "instance-efgh",
					Namespace: "velero",
					Annotations: map[string]string{
						BackupIsECAnnotation: "true",
						// legacy backups do not have the InstanceBackupTypeAnnotation
					},
					CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)},
					ResourceVersion:   "999",
				},
			},
		},
		{
			name: "improved dr backups should return the infra backup",
			b: ReplicatedBackup{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "application-abcd",
						Namespace: "velero",
						Labels: map[string]string{
							InstanceBackupNameLabel: "app-slug-abcd",
						},
						Annotations: map[string]string{
							BackupIsECAnnotation:          "true",
							InstanceBackupTypeAnnotation:  InstanceBackupTypeApp,
							InstanceBackupCountAnnotation: "2",
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 4, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "instance-abcd",
						Namespace: "velero",
						Labels: map[string]string{
							InstanceBackupNameLabel: "app-slug-abcd",
						},
						Annotations: map[string]string{
							BackupIsECAnnotation:          "true",
							InstanceBackupTypeAnnotation:  InstanceBackupTypeInfra,
							InstanceBackupCountAnnotation: "2",
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
			},
			want: &velerov1.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "instance-abcd",
					Namespace: "velero",
					Labels: map[string]string{
						InstanceBackupNameLabel: "app-slug-abcd",
					},
					Annotations: map[string]string{
						BackupIsECAnnotation:          "true",
						InstanceBackupTypeAnnotation:  InstanceBackupTypeInfra,
						InstanceBackupCountAnnotation: "2",
					},
					CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)},
					ResourceVersion:   "999",
				},
			},
		},
		{
			name: "no backups should return nil",
			b:    ReplicatedBackup{},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.b.GetInfraBackup()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReplicatedBackup_GetAppBackup(t *testing.T) {
	tests := []struct {
		name string
		b    ReplicatedBackup
		want *velerov1.Backup
	}{
		{
			name: "legacy backups should return the legacy backup",
			b: ReplicatedBackup{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "instance-efgh",
						Namespace: "velero",
						Annotations: map[string]string{
							BackupIsECAnnotation: "true",
							// legacy backups do not have the InstanceBackupTypeAnnotation
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
			},
			want: &velerov1.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "instance-efgh",
					Namespace: "velero",
					Annotations: map[string]string{
						BackupIsECAnnotation: "true",
						// legacy backups do not have the InstanceBackupTypeAnnotation
					},
					CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)},
					ResourceVersion:   "999",
				},
			},
		},
		{
			name: "improved dr backups should return the infra backup",
			b: ReplicatedBackup{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "application-abcd",
						Namespace: "velero",
						Labels: map[string]string{
							InstanceBackupNameLabel: "app-slug-abcd",
						},
						Annotations: map[string]string{
							BackupIsECAnnotation:          "true",
							InstanceBackupTypeAnnotation:  InstanceBackupTypeApp,
							InstanceBackupCountAnnotation: "2",
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 4, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "instance-abcd",
						Namespace: "velero",
						Labels: map[string]string{
							InstanceBackupNameLabel: "app-slug-abcd",
						},
						Annotations: map[string]string{
							BackupIsECAnnotation:          "true",
							InstanceBackupTypeAnnotation:  InstanceBackupTypeInfra,
							InstanceBackupCountAnnotation: "2",
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
			},
			want: &velerov1.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "application-abcd",
					Namespace: "velero",
					Labels: map[string]string{
						InstanceBackupNameLabel: "app-slug-abcd",
					},
					Annotations: map[string]string{
						BackupIsECAnnotation:          "true",
						InstanceBackupTypeAnnotation:  InstanceBackupTypeApp,
						InstanceBackupCountAnnotation: "2",
					},
					CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 4, 0, 0, 0, 0, time.UTC)},
					ResourceVersion:   "999",
				},
			},
		},
		{
			name: "no backups should return nil",
			b:    ReplicatedBackup{},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.b.GetAppBackup()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReplicatedBackup_GetExpectedBackupCount(t *testing.T) {
	tests := []struct {
		name string
		b    ReplicatedBackup
		want int
	}{
		{
			name: "legacy backups should return 1",
			b: ReplicatedBackup{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "instance-efgh",
						Namespace: "velero",
						Annotations: map[string]string{
							BackupIsECAnnotation: "true",
							// legacy backups do not have the InstanceBackupTypeAnnotation
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
			},
			want: 1,
		},
		{
			name: "improved dr backups should return 2",
			b: ReplicatedBackup{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "application-abcd",
						Namespace: "velero",
						Labels: map[string]string{
							InstanceBackupNameLabel: "app-slug-abcd",
						},
						Annotations: map[string]string{
							BackupIsECAnnotation:          "true",
							InstanceBackupTypeAnnotation:  InstanceBackupTypeApp,
							InstanceBackupCountAnnotation: "2",
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 4, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "instance-abcd",
						Namespace: "velero",
						Labels: map[string]string{
							InstanceBackupNameLabel: "app-slug-abcd",
						},
						Annotations: map[string]string{
							BackupIsECAnnotation:          "true",
							InstanceBackupTypeAnnotation:  InstanceBackupTypeInfra,
							InstanceBackupCountAnnotation: "2",
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
			},
			want: 2,
		},
		{
			name: "improved dr backup without infra backup should return 2",
			b: ReplicatedBackup{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "application-abcd",
						Namespace: "velero",
						Labels: map[string]string{
							InstanceBackupNameLabel: "app-slug-abcd",
						},
						Annotations: map[string]string{
							BackupIsECAnnotation:          "true",
							InstanceBackupTypeAnnotation:  InstanceBackupTypeApp,
							InstanceBackupCountAnnotation: "2",
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 4, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
			},
			want: 2,
		},
		{
			name: "no backups should return 0",
			b:    ReplicatedBackup{},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.b.GetExpectedBackupCount()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReplicatedBackup_GetCreationTimestamp(t *testing.T) {
	tests := []struct {
		name string
		b    ReplicatedBackup
		want metav1.Time
	}{
		{
			name: "legacy backups should return the legacy backup creation timestamp",
			b: ReplicatedBackup{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "instance-efgh",
						Namespace: "velero",
						Annotations: map[string]string{
							BackupIsECAnnotation: "true",
							// legacy backups do not have the InstanceBackupTypeAnnotation
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
			},
			want: metav1.Time{Time: time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)},
		},
		{
			name: "improved dr backups should return the infra backup creation timestamp",
			b: ReplicatedBackup{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "application-abcd",
						Namespace: "velero",
						Labels: map[string]string{
							InstanceBackupNameLabel: "app-slug-abcd",
						},
						Annotations: map[string]string{
							BackupIsECAnnotation:          "true",
							InstanceBackupTypeAnnotation:  InstanceBackupTypeApp,
							InstanceBackupCountAnnotation: "2",
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 4, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "instance-abcd",
						Namespace: "velero",
						Labels: map[string]string{
							InstanceBackupNameLabel: "app-slug-abcd",
						},
						Annotations: map[string]string{
							BackupIsECAnnotation:          "true",
							InstanceBackupTypeAnnotation:  InstanceBackupTypeInfra,
							InstanceBackupCountAnnotation: "2",
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
			},
			want: metav1.Time{Time: time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)},
		},
		{
			name: "improved dr backup without infra backup should return the app backup creation timestamp",
			b: ReplicatedBackup{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "application-abcd",
						Namespace: "velero",
						Labels: map[string]string{
							InstanceBackupNameLabel: "app-slug-abcd",
						},
						Annotations: map[string]string{
							BackupIsECAnnotation:          "true",
							InstanceBackupTypeAnnotation:  InstanceBackupTypeApp,
							InstanceBackupCountAnnotation: "2",
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 4, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
			},
			want: metav1.Time{Time: time.Date(2022, 1, 4, 0, 0, 0, 0, time.UTC)},
		},
		{
			name: "no backups should return zero",
			b:    ReplicatedBackup{},
			want: metav1.Time{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.b.GetCreationTimestamp()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReplicatedBackup_GetAnnotation(t *testing.T) {
	type args struct {
		key string
	}
	tests := []struct {
		name  string
		b     ReplicatedBackup
		args  args
		want  string
		want1 bool
	}{
		{
			name: "legacy backups should return the legacy backup annotation",
			b: ReplicatedBackup{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "instance-efgh",
						Namespace: "velero",
						Annotations: map[string]string{
							BackupIsECAnnotation: "true",
							// legacy backups do not have the InstanceBackupTypeAnnotation
							"some-annotation": "some-value",
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
			},
			args: args{
				key: "some-annotation",
			},
			want:  "some-value",
			want1: true,
		},
		{
			name: "improved dr backups should return the infra backup annotation",
			b: ReplicatedBackup{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "application-abcd",
						Namespace: "velero",
						Labels: map[string]string{
							InstanceBackupNameLabel: "app-slug-abcd",
						},
						Annotations: map[string]string{
							BackupIsECAnnotation:          "true",
							InstanceBackupTypeAnnotation:  InstanceBackupTypeApp,
							InstanceBackupCountAnnotation: "2",
							"some-annotation":             "some-other-value",
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 4, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "instance-abcd",
						Namespace: "velero",
						Labels: map[string]string{
							InstanceBackupNameLabel: "app-slug-abcd",
						},
						Annotations: map[string]string{
							BackupIsECAnnotation:          "true",
							InstanceBackupTypeAnnotation:  InstanceBackupTypeInfra,
							InstanceBackupCountAnnotation: "2",
							"some-annotation":             "some-value",
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
			},
			args: args{
				key: "some-annotation",
			},
			want:  "some-value",
			want1: true,
		},
		{
			name: "not found annotation should return false",
			b: ReplicatedBackup{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "application-abcd",
						Namespace: "velero",
						Labels: map[string]string{
							InstanceBackupNameLabel: "app-slug-abcd",
						},
						Annotations: map[string]string{
							BackupIsECAnnotation:          "true",
							InstanceBackupTypeAnnotation:  InstanceBackupTypeApp,
							InstanceBackupCountAnnotation: "2",
							"some-annotation":             "some-other-value",
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 4, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "instance-abcd",
						Namespace: "velero",
						Labels: map[string]string{
							InstanceBackupNameLabel: "app-slug-abcd",
						},
						Annotations: map[string]string{
							BackupIsECAnnotation:          "true",
							InstanceBackupTypeAnnotation:  InstanceBackupTypeInfra,
							InstanceBackupCountAnnotation: "2",
							"some-annotation":             "some-value",
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
			},
			args: args{
				key: "some-other-annotation",
			},
			want:  "",
			want1: false,
		},
		{
			name: "improved dr backup without infra backup should return the app backup annotation",
			b: ReplicatedBackup{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "application-abcd",
						Namespace: "velero",
						Labels: map[string]string{
							InstanceBackupNameLabel: "app-slug-abcd",
						},
						Annotations: map[string]string{
							BackupIsECAnnotation:          "true",
							InstanceBackupTypeAnnotation:  InstanceBackupTypeApp,
							InstanceBackupCountAnnotation: "2",
							"some-annotation":             "some-other-value",
						},
						CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 4, 0, 0, 0, 0, time.UTC)},
						ResourceVersion:   "999",
					},
				},
			},
			args: args{
				key: "some-annotation",
			},
			want:  "some-other-value",
			want1: true,
		},
		{
			name: "no backups should return false",
			b:    ReplicatedBackup{},
			args: args{
				key: "some-annotation",
			},
			want:  "",
			want1: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := tt.b.GetAnnotation(tt.args.key)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.want1, got1)
		})
	}
}

func TestReplicatedBackup_GetRestore(t *testing.T) {
	tests := []struct {
		name    string
		b       ReplicatedBackup
		want    *velerov1.Restore
		wantErr bool
	}{
		{
			name: "has app backup with annotation should return restore",
			b: ReplicatedBackup{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "instance-abcd",
						Namespace: "velero",
						Labels: map[string]string{
							InstanceBackupNameLabel: "app-slug-abcd",
						},
						Annotations: map[string]string{
							BackupIsECAnnotation:               "true",
							InstanceBackupTypeAnnotation:       InstanceBackupTypeApp,
							InstanceBackupCountAnnotation:      "2",
							InstanceBackupResoreSpecAnnotation: `{"kind":"Restore","apiVersion":"velero.io/v1","metadata":{"name":"test-restore","creationTimestamp":null},"spec":{"backupName":"test-backup","hooks":{}},"status":{}}`,
						},
					},
				},
			},
			want: &velerov1.Restore{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Restore",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-restore",
				},
				Spec: velerov1.RestoreSpec{
					BackupName: "test-backup",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.b.GetRestore()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
