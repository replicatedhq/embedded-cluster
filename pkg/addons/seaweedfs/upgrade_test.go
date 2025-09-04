package seaweedfs

import (
	"context"
	"testing"

	"github.com/Masterminds/semver/v3"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_lessThanECVersion273(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{
			name:    "version 2.7.2 is less than 2.7.3",
			version: "2.7.2+k8s-1.29-49-gf92daca6",
			want:    true,
		},
		{
			name:    "version 2.7.3 is not less than 2.7.3",
			version: "2.7.3+k8s-1.29-49-gf92daca6",
			want:    false,
		},
		{
			name:    "version 2.7.4 is not less than 2.7.3",
			version: "2.7.4+k8s-1.29-49-gf92daca6",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ver := semver.MustParse(tt.version)
			got := lessThanECVersion273(ver)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_needsScalingRestart(t *testing.T) {
	scheme := scheme.Scheme
	require.NoError(t, ecv1beta1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))

	tests := []struct {
		name    string
		objects []client.Object
		want    bool
	}{
		{
			name: "needs restart - pre 2.7.3 upgrade with 3 replicas not all ready",
			objects: []client.Object{
				&ecv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20241002205018", // Latest
					},
					Spec: ecv1beta1.InstallationSpec{
						Config: &ecv1beta1.ConfigSpec{
							Version: "2.7.3+k8s-1.29-49-gf92daca6",
						},
					},
				},
				&ecv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20241001205018", // Previous - pre 2.7.3
					},
					Spec: ecv1beta1.InstallationSpec{
						Config: &ecv1beta1.ConfigSpec{
							Version: "2.7.2+k8s-1.29-49-gf92daca6",
						},
					},
				},
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "seaweedfs-master",
						Namespace: "seaweedfs",
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: int32Ptr(3),
					},
					Status: appsv1.StatefulSetStatus{
						ReadyReplicas: 1, // Not all replicas ready
					},
				},
			},
			want: true,
		},
		{
			name: "no restart needed - all replicas ready",
			objects: []client.Object{
				&ecv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20241002205018", // Latest
					},
					Spec: ecv1beta1.InstallationSpec{
						Config: &ecv1beta1.ConfigSpec{
							Version: "2.7.3+k8s-1.29-49-gf92daca6",
						},
					},
				},
				&ecv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20241001205018", // Previous - pre 2.7.3
					},
					Spec: ecv1beta1.InstallationSpec{
						Config: &ecv1beta1.ConfigSpec{
							Version: "2.7.2+k8s-1.29-49-gf92daca6",
						},
					},
				},
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "seaweedfs-master",
						Namespace: "seaweedfs",
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: int32Ptr(3),
					},
					Status: appsv1.StatefulSetStatus{
						ReadyReplicas: 3, // All replicas ready
					},
				},
			},
			want: false,
		},
		{
			name: "restart needed - upgrading from pre-2.7.3 with 1 replica",
			objects: []client.Object{
				&ecv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20241002205018", // Latest
					},
					Spec: ecv1beta1.InstallationSpec{
						Config: &ecv1beta1.ConfigSpec{
							Version: "2.7.3+k8s-1.29-49-gf92daca6",
						},
					},
				},
				&ecv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20241001205018", // Previous - pre 2.7.3
					},
					Spec: ecv1beta1.InstallationSpec{
						Config: &ecv1beta1.ConfigSpec{
							Version: "2.7.2+k8s-1.29-49-gf92daca6",
						},
					},
				},
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "seaweedfs-master",
						Namespace: "seaweedfs",
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: int32Ptr(1), // 1 replica needs scaling
					},
					Status: appsv1.StatefulSetStatus{
						ReadyReplicas: 1,
					},
				},
			},
			want: true,
		},
		{
			name: "no restart needed - upgrading from post-2.7.3 version",
			objects: []client.Object{
				&ecv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20241002205018", // Latest
					},
					Spec: ecv1beta1.InstallationSpec{
						Config: &ecv1beta1.ConfigSpec{
							Version: "2.8.0+k8s-1.29-49-gf92daca6",
						},
					},
				},
				&ecv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20241001205018", // Previous - post 2.7.3
					},
					Spec: ecv1beta1.InstallationSpec{
						Config: &ecv1beta1.ConfigSpec{
							Version: "2.7.4+k8s-1.29-49-gf92daca6",
						},
					},
				},
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "seaweedfs-master",
						Namespace: "seaweedfs",
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: int32Ptr(1),
					},
					Status: appsv1.StatefulSetStatus{
						ReadyReplicas: 1,
					},
				},
			},
			want: false,
		},
		{
			name: "no restart needed - no previous installation",
			objects: []client.Object{
				&ecv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20241002205018", // Only installation
					},
					Spec: ecv1beta1.InstallationSpec{
						Config: &ecv1beta1.ConfigSpec{
							Version: "2.7.3+k8s-1.29-49-gf92daca6",
						},
					},
				},
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "seaweedfs-master",
						Namespace: "seaweedfs",
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: int32Ptr(3),
					},
					Status: appsv1.StatefulSetStatus{
						ReadyReplicas: 1,
					},
				},
			},
			want: false,
		},
		{
			name: "no restart needed - previous installation missing version",
			objects: []client.Object{
				&ecv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20241002205018", // Latest
					},
					Spec: ecv1beta1.InstallationSpec{
						Config: &ecv1beta1.ConfigSpec{
							Version: "2.7.3+k8s-1.29-49-gf92daca6",
						},
					},
				},
				&ecv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20241001205018", // Previous - no version
					},
					Spec: ecv1beta1.InstallationSpec{
						Config: &ecv1beta1.ConfigSpec{
							// Version missing
						},
					},
				},
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "seaweedfs-master",
						Namespace: "seaweedfs",
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: int32Ptr(3),
					},
					Status: appsv1.StatefulSetStatus{
						ReadyReplicas: 1,
					},
				},
			},
			want: false,
		},
		{
			name: "no restart needed - previous installation has nil config",
			objects: []client.Object{
				&ecv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20241002205018", // Latest
					},
					Spec: ecv1beta1.InstallationSpec{
						Config: &ecv1beta1.ConfigSpec{
							Version: "2.7.3+k8s-1.29-49-gf92daca6",
						},
					},
				},
				&ecv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20241001205018", // Previous - nil config
					},
					Spec: ecv1beta1.InstallationSpec{
						Config: nil,
					},
				},
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "seaweedfs-master",
						Namespace: "seaweedfs",
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: int32Ptr(3),
					},
					Status: appsv1.StatefulSetStatus{
						ReadyReplicas: 1,
					},
				},
			},
			want: false,
		},
		{
			name: "no restart needed - invalid previous version",
			objects: []client.Object{
				&ecv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20241002205018", // Latest
					},
					Spec: ecv1beta1.InstallationSpec{
						Config: &ecv1beta1.ConfigSpec{
							Version: "2.7.3+k8s-1.29-49-gf92daca6",
						},
					},
				},
				&ecv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20241001205018", // Previous - invalid version
					},
					Spec: ecv1beta1.InstallationSpec{
						Config: &ecv1beta1.ConfigSpec{
							Version: "invalid-version",
						},
					},
				},
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "seaweedfs-master",
						Namespace: "seaweedfs",
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: int32Ptr(3),
					},
					Status: appsv1.StatefulSetStatus{
						ReadyReplicas: 1,
					},
				},
			},
			want: false,
		},
		{
			name: "no restart needed - no installations",
			objects: []client.Object{
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "seaweedfs-master",
						Namespace: "seaweedfs",
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: int32Ptr(3),
					},
					Status: appsv1.StatefulSetStatus{
						ReadyReplicas: 1,
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...)

			cli := builder.Build()

			s := &SeaweedFS{}
			got := s.needsScalingRestart(context.Background(), cli)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Helper function for creating int32 pointers
func int32Ptr(i int32) *int32 {
	return &i
}
