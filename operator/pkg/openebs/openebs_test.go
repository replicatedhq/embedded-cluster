package openebs

import (
	"context"
	"testing"

	"github.com/replicatedhq/embedded-cluster/operator/pkg/testutils"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCleanupStatefulPods(t *testing.T) {
	tests := []struct {
		name            string
		initRuntimeObjs []runtime.Object
		assertRuntime   func(t *testing.T, cli client.Client)
		wantErr         bool
	}{
		{
			name: "basic",
			initRuntimeObjs: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "seaweedfs",
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "found-node",
					},
				},
				&corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "seaweedfs",
						Name:      "data-seaweedfs-volume-0",
						Annotations: map[string]string{
							"volume.kubernetes.io/storage-provisioner": "openebs.io/local",
							"volume.kubernetes.io/selected-node":       "missing-node",
						},
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						StorageClassName: ptr.To("openebs-hostpath"),
						VolumeMode:       ptr.To(corev1.PersistentVolumeFilesystem),
						VolumeName:       "pvc-95d2229d-0dba-477f-a2d7-e0f7c3f51e3b",
					},
				},
				&corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "seaweedfs",
						Name:      "data-seaweedfs-volume-1",
						Annotations: map[string]string{
							"volume.kubernetes.io/storage-provisioner": "openebs.io/local",
							"volume.kubernetes.io/selected-node":       "found-node",
						},
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						StorageClassName: ptr.To("openebs-hostpath"),
						VolumeMode:       ptr.To(corev1.PersistentVolumeFilesystem),
						VolumeName:       "pvc-45abaad6-abd6-4b45-97c1-b7a6ee07fd93",
					},
				},
				&corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pvc-95d2229d-0dba-477f-a2d7-e0f7c3f51e3b",
					},
				},
				&corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pvc-45abaad6-abd6-4b45-97c1-b7a6ee07fd93",
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "seaweedfs",
						Name:      "seaweedfs-volume-0",
					},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "data",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: "data-seaweedfs-volume-0",
									},
								},
							},
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodPending,
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "seaweedfs",
						Name:      "seaweedfs-volume-1",
					},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "data",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: "data-seaweedfs-volume-1",
									},
								},
							},
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodPending,
					},
				},
			},
			assertRuntime: func(t *testing.T, cli client.Client) {
				namespace := &corev1.Namespace{}
				err := cli.Get(context.Background(), client.ObjectKey{Name: "seaweedfs"}, namespace)
				require.NoError(t, err)

				pod := &corev1.Pod{}
				err = cli.Get(context.Background(), client.ObjectKey{Namespace: "seaweedfs", Name: "seaweedfs-volume-0"}, pod)
				require.EqualError(t, err, "pods \"seaweedfs-volume-0\" not found")

				pod = &corev1.Pod{}
				err = cli.Get(context.Background(), client.ObjectKey{Namespace: "seaweedfs", Name: "seaweedfs-volume-1"}, pod)
				require.NoError(t, err)

				pvc := &corev1.PersistentVolumeClaim{}
				err = cli.Get(context.Background(), client.ObjectKey{Namespace: "seaweedfs", Name: "data-seaweedfs-volume-0"}, pvc)
				require.EqualError(t, err, "persistentvolumeclaims \"data-seaweedfs-volume-0\" not found")

				pvc = &corev1.PersistentVolumeClaim{}
				err = cli.Get(context.Background(), client.ObjectKey{Namespace: "seaweedfs", Name: "data-seaweedfs-volume-1"}, pvc)
				require.NoError(t, err)

				pv := &corev1.PersistentVolume{}
				err = cli.Get(context.Background(), client.ObjectKey{Name: "pvc-95d2229d-0dba-477f-a2d7-e0f7c3f51e3b"}, pv)
				require.EqualError(t, err, "persistentvolumes \"pvc-95d2229d-0dba-477f-a2d7-e0f7c3f51e3b\" not found")

				pv = &corev1.PersistentVolume{}
				err = cli.Get(context.Background(), client.ObjectKey{Name: "pvc-45abaad6-abd6-4b45-97c1-b7a6ee07fd93"}, pv)
				require.NoError(t, err)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := fake.NewClientBuilder().
				WithScheme(testutils.Scheme(t)).
				WithRuntimeObjects(tt.initRuntimeObjs...).
				Build()

			err := CleanupStatefulPods(context.Background(), cli)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			tt.assertRuntime(t, cli)
		})
	}
}
