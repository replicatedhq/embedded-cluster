package cli

import (
	"context"
	"embed"
	"io"
	"io/fs"
	"testing"
	"time"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	clitesting "github.com/replicatedhq/embedded-cluster/cmd/installer/cli/testing"
	"github.com/replicatedhq/embedded-cluster/pkg/disasterrecovery"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_isReplicatedBackupRestorable(t *testing.T) {
	appendCommonAnnotations := func(annotations map[string]string) map[string]string {
		annotations["kots.io/embedded-cluster-version"] = "v0.0.0"
		annotations["kots.io/apps-versions"] = `{"app-slug":"1.0.0"}`
		annotations["kots.io/is-airgap"] = "false"
		return annotations
	}

	infraBackup := velerov1.Backup{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Backup",
			APIVersion: "velero.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "instance-abcd",
			Namespace: "velero",
			Labels: map[string]string{
				disasterrecovery.InstanceBackupNameLabel: "app-slug-abcd",
			},
			Annotations: appendCommonAnnotations(map[string]string{
				disasterrecovery.BackupIsECAnnotation:            "true",
				disasterrecovery.InstanceBackupVersionAnnotation: disasterrecovery.InstanceBackupVersionCurrent,
				disasterrecovery.InstanceBackupTypeAnnotation:    disasterrecovery.InstanceBackupTypeInfra,
				disasterrecovery.InstanceBackupCountAnnotation:   "2",
			}),
			CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 3, 0, 0, 0, 0, time.UTC)},
		},
		Status: velerov1.BackupStatus{
			Phase: velerov1.BackupPhaseCompleted,
		},
	}
	appBackup := velerov1.Backup{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Backup",
			APIVersion: "velero.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "application-abcd",
			Namespace: "velero",
			Labels: map[string]string{
				disasterrecovery.InstanceBackupNameLabel: "app-slug-abcd",
			},
			Annotations: appendCommonAnnotations(map[string]string{
				disasterrecovery.BackupIsECAnnotation:            "true",
				disasterrecovery.InstanceBackupVersionAnnotation: disasterrecovery.InstanceBackupVersionCurrent,
				disasterrecovery.InstanceBackupTypeAnnotation:    disasterrecovery.InstanceBackupTypeApp,
				disasterrecovery.InstanceBackupCountAnnotation:   "2",
			}),
			CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 4, 0, 0, 0, 0, time.UTC)},
		},
		Status: velerov1.BackupStatus{
			Phase: velerov1.BackupPhaseCompleted,
		},
	}
	legacyBackup := velerov1.Backup{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Backup",
			APIVersion: "velero.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "instance-efgh",
			Namespace: "velero",
			Annotations: appendCommonAnnotations(map[string]string{
				disasterrecovery.BackupIsECAnnotation:     "true",
				disasterrecovery.InstanceBackupAnnotation: "true",
				// legacy backups do not have the InstanceBackupTypeAnnotation
			}),
			CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)},
		},
		Status: velerov1.BackupStatus{
			Phase: velerov1.BackupPhaseCompleted,
		},
	}

	type args struct {
		backup   disasterrecovery.ReplicatedBackup
		rel      *release.ChannelRelease
		isAirgap bool
		k0sCfg   *k0sv1beta1.ClusterConfig
	}
	tests := []struct {
		name      string
		releaseFS embed.FS
		args      args
		want      bool
		want1     string
	}{
		{
			name:      "wrong backup count should fail",
			releaseFS: clitesting.RestoreReleaseDataNewDR,
			args: args{
				backup: disasterrecovery.ReplicatedBackup{
					appBackup,
				},
				rel: &release.ChannelRelease{
					VersionLabel: "1.0.0",
					AppSlug:      "app-slug",
				},
				isAirgap: false,
				k0sCfg:   &k0sv1beta1.ClusterConfig{},
			},
			want:  false,
			want1: "has a different number of backups (1) than the expected number (2)",
		},
		{
			name:      "app backup found but uses legacy dr should fail",
			releaseFS: clitesting.RestoreReleaseDataLegacyDR,
			args: args{
				backup: disasterrecovery.ReplicatedBackup{
					infraBackup,
					appBackup,
				},
				rel: &release.ChannelRelease{
					VersionLabel: "1.0.0",
					AppSlug:      "app-slug",
				},
				isAirgap: false,
				k0sCfg:   &k0sv1beta1.ClusterConfig{},
			},
			want:  false,
			want1: "app backup found but improved dr is not enabled",
		},
		{
			name:      "legacy backup found but uses improved dr should fail",
			releaseFS: clitesting.RestoreReleaseDataNewDR,
			args: args{
				backup: disasterrecovery.ReplicatedBackup{
					legacyBackup,
				},
				rel: &release.ChannelRelease{
					VersionLabel: "1.0.0",
					AppSlug:      "app-slug",
				},
				isAirgap: false,
				k0sCfg:   &k0sv1beta1.ClusterConfig{},
			},
			want:  false,
			want1: "legacy backup found but improved dr is enabled",
		},
		{
			name:      "valid improved dr backup should return true",
			releaseFS: clitesting.RestoreReleaseDataNewDR,
			args: args{
				backup: disasterrecovery.ReplicatedBackup{
					infraBackup,
					appBackup,
				},
				rel: &release.ChannelRelease{
					VersionLabel: "1.0.0",
					AppSlug:      "app-slug",
				},
				isAirgap: false,
				k0sCfg:   &k0sv1beta1.ClusterConfig{},
			},
			want:  true,
			want1: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := embedFSToMap(t, tt.releaseFS)
			release.SetReleaseDataForTests(files)
			rc := runtimeconfig.New(nil)

			got, got1 := isReplicatedBackupRestorable(tt.args.backup, tt.args.rel, tt.args.isAirgap, tt.args.k0sCfg, rc)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.want1, got1)
		})
	}
}

func Test_waitForBackups(t *testing.T) {
	scheme := kubeutils.Scheme

	appendCommonAnnotations := func(annotations map[string]string) map[string]string {
		annotations["kots.io/embedded-cluster-version"] = "v0.0.0"
		annotations["kots.io/apps-versions"] = `{"app-slug":"1.0.0"}`
		annotations["kots.io/is-airgap"] = "false"
		return annotations
	}

	infraBackup := velerov1.Backup{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Backup",
			APIVersion: "velero.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "instance-abcd",
			Namespace: "velero",
			Labels: map[string]string{
				disasterrecovery.InstanceBackupNameLabel: "app-slug-abcd",
			},
			Annotations: appendCommonAnnotations(map[string]string{
				disasterrecovery.BackupIsECAnnotation:            "true",
				disasterrecovery.InstanceBackupVersionAnnotation: disasterrecovery.InstanceBackupVersionCurrent,
				disasterrecovery.InstanceBackupTypeAnnotation:    disasterrecovery.InstanceBackupTypeInfra,
				disasterrecovery.InstanceBackupCountAnnotation:   "2",
			}),
			CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 3, 0, 0, 0, 0, time.Local)},
		},
		Status: velerov1.BackupStatus{
			Phase: velerov1.BackupPhaseCompleted,
		},
	}
	appBackup := velerov1.Backup{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Backup",
			APIVersion: "velero.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "application-abcd",
			Namespace: "velero",
			Labels: map[string]string{
				disasterrecovery.InstanceBackupNameLabel: "app-slug-abcd",
			},
			Annotations: appendCommonAnnotations(map[string]string{
				disasterrecovery.BackupIsECAnnotation:            "true",
				disasterrecovery.InstanceBackupVersionAnnotation: disasterrecovery.InstanceBackupVersionCurrent,
				disasterrecovery.InstanceBackupTypeAnnotation:    disasterrecovery.InstanceBackupTypeApp,
				disasterrecovery.InstanceBackupCountAnnotation:   "2",
			}),
			CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 4, 0, 0, 0, 0, time.Local)},
		},
		Status: velerov1.BackupStatus{
			Phase: velerov1.BackupPhaseCompleted,
		},
	}
	legacyBackup := velerov1.Backup{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Backup",
			APIVersion: "velero.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "instance-efgh",
			Namespace: "velero",
			Annotations: appendCommonAnnotations(map[string]string{
				disasterrecovery.BackupIsECAnnotation:     "true",
				disasterrecovery.InstanceBackupAnnotation: "true",
				// legacy backups do not have the InstanceBackupTypeAnnotation
			}),
			CreationTimestamp: metav1.Time{Time: time.Date(2022, 1, 2, 0, 0, 0, 0, time.Local)},
		},
		Status: velerov1.BackupStatus{
			Phase: velerov1.BackupPhaseCompleted,
		},
	}

	type args struct {
		kcli     client.Client
		k0sCfg   *k0sv1beta1.ClusterConfig
		isAirgap bool
	}
	tests := []struct {
		name      string
		releaseFS embed.FS
		args      args
		want      []disasterrecovery.ReplicatedBackup
		wantErr   bool
	}{
		{
			name:      "legacy release data should return valid legacy backup",
			releaseFS: clitesting.RestoreReleaseDataLegacyDR,
			args: args{
				kcli: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
					&infraBackup,
					&appBackup,
					&legacyBackup,
				).Build(),
				k0sCfg:   &k0sv1beta1.ClusterConfig{},
				isAirgap: false,
			},
			want: []disasterrecovery.ReplicatedBackup{
				{
					legacyBackup,
				},
			},
			wantErr: false,
		},
		{
			name:      "new dr release data should return valid backup",
			releaseFS: clitesting.RestoreReleaseDataNewDR,
			args: args{
				kcli: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
					&infraBackup,
					&appBackup,
					&legacyBackup,
				).Build(),
				k0sCfg:   &k0sv1beta1.ClusterConfig{},
				isAirgap: false,
			},
			want: []disasterrecovery.ReplicatedBackup{
				{
					appBackup,
					infraBackup,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := embedFSToMap(t, tt.releaseFS)
			release.SetReleaseDataForTests(files)
			rc := runtimeconfig.New(nil)

			got, err := waitForBackups(context.Background(), io.Discard, tt.args.kcli, tt.args.k0sCfg, rc, tt.args.isAirgap)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_ensureImprovedDrMetadata(t *testing.T) {
	type args struct {
		restore *velerov1.Restore
		backup  *velerov1.Backup
	}
	tests := []struct {
		name            string
		args            args
		wantLabels      map[string]string
		wantAnnotations map[string]string
	}{
		{
			name: "legacy dr should not append labels and annotations",
			args: args{
				restore: &velerov1.Restore{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "restore",
						Namespace: "velero",
						Labels: map[string]string{
							"some-label": "some-value",
						},
						Annotations: map[string]string{
							"some-annotation": "some-value",
						},
					},
					Spec: velerov1.RestoreSpec{
						BackupName: "backup",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app.kubernetes.io/name": "embedded-cluster",
							},
						},
					},
				},
				backup: &velerov1.Backup{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "backup",
						Namespace: "velero",
						Labels: map[string]string{
							"some-other-label": "some-other-value",
						},
						Annotations: map[string]string{
							disasterrecovery.BackupIsECAnnotation:     "true",
							disasterrecovery.InstanceBackupAnnotation: "true",
							"some-other-annotation":                   "some-other-value",
						},
					},
				},
			},
			wantLabels: map[string]string{
				"some-label": "some-value",
			},
			wantAnnotations: map[string]string{
				"some-annotation": "some-value",
			},
		},
		{
			name: "new dr should append labels and annotations",
			args: args{
				restore: &velerov1.Restore{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "restore",
						Namespace: "velero",
						Labels: map[string]string{
							"some-label": "some-value",
						},
						Annotations: map[string]string{
							"some-annotation": "some-value",
						},
					},
					Spec: velerov1.RestoreSpec{
						BackupName: "backup",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app.kubernetes.io/name": "embedded-cluster",
							},
						},
						IncludeClusterResources: ptr.To(true),
					},
				},
				backup: &velerov1.Backup{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Backup",
						APIVersion: "velero.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "backup",
						Namespace: "velero",
						Labels: map[string]string{
							disasterrecovery.InstanceBackupNameLabel: "app-slug-abcd",
							"some-other-label":                       "some-other-value",
						},
						Annotations: map[string]string{
							disasterrecovery.BackupIsECAnnotation:            "true",
							disasterrecovery.InstanceBackupVersionAnnotation: disasterrecovery.InstanceBackupVersionCurrent,
							disasterrecovery.InstanceBackupTypeAnnotation:    disasterrecovery.InstanceBackupTypeApp,
							disasterrecovery.InstanceBackupCountAnnotation:   "2",
							"some-other-annotation":                          "some-other-value",
						},
					},
				},
			},
			wantLabels: map[string]string{
				"some-label":                             "some-value",
				disasterrecovery.InstanceBackupNameLabel: "app-slug-abcd",
			},
			wantAnnotations: map[string]string{
				"some-annotation": "some-value",
				disasterrecovery.InstanceBackupTypeAnnotation:  disasterrecovery.InstanceBackupTypeApp,
				disasterrecovery.InstanceBackupCountAnnotation: "2",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ensureImprovedDrMetadata(tt.args.restore, tt.args.backup)
			assert.Equal(t, tt.wantLabels, tt.args.restore.Labels)
			assert.Equal(t, tt.wantAnnotations, tt.args.restore.Annotations)
		})
	}
}

func embedFSToMap(t *testing.T, f embed.FS) map[string][]byte {
	files := map[string][]byte{}
	err := fs.WalkDir(f, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := f.ReadFile(path)
		if err != nil {
			return err
		}
		files[d.Name()] = data
		return nil
	})
	if err != nil {
		t.Fatalf("fail to walk embed fs: %v", err)
	}
	return files
}
