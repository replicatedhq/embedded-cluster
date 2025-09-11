package kubeutils

import (
	"context"
	"testing"
	"time"

	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetPreviousInstallation(t *testing.T) {
	scheme := scheme.Scheme
	embeddedclusterv1beta1.AddToScheme(scheme)

	tests := []struct {
		name    string
		in      *embeddedclusterv1beta1.Installation
		want    *embeddedclusterv1beta1.Installation
		wantErr bool
		objects []client.Object
	}{
		{
			name: "no installations at all",
			in: &embeddedclusterv1beta1.Installation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "20241002205018",
				},
				Spec: embeddedclusterv1beta1.InstallationSpec{
					Config: &embeddedclusterv1beta1.ConfigSpec{
						Version: "1.13.0+k8s-1.29-49-gf92daca6",
					},
				},
			},
			want:    nil,
			wantErr: true,
			objects: []client.Object{},
		},
		{
			name: "no previous installation",
			in: &embeddedclusterv1beta1.Installation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "20241002205018",
				},
				Spec: embeddedclusterv1beta1.InstallationSpec{
					Config: &embeddedclusterv1beta1.ConfigSpec{
						Version: "1.13.0+k8s-1.29-49-gf92daca6",
					},
				},
			},
			want:    nil,
			wantErr: true,
			objects: []client.Object{
				&embeddedclusterv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20241002205018",
					},
					Spec: embeddedclusterv1beta1.InstallationSpec{
						Config: &embeddedclusterv1beta1.ConfigSpec{
							Version: "1.13.0+k8s-1.29-49-gf92daca6",
						},
					},
				},
			},
		},
		{
			name: "multiple previous installations",
			in: &embeddedclusterv1beta1.Installation{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Installation",
					APIVersion: "embeddedcluster.replicated.com/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "20241002205018",
				},
				Spec: embeddedclusterv1beta1.InstallationSpec{
					Config: &embeddedclusterv1beta1.ConfigSpec{
						Version: "1.13.0+k8s-1.29-49-gf92daca6",
					},
				},
			},
			want: &embeddedclusterv1beta1.Installation{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Installation",
					APIVersion: "embeddedcluster.replicated.com/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "20230000000000",
					ResourceVersion: "1000",
				},
				Spec: embeddedclusterv1beta1.InstallationSpec{
					Config: &embeddedclusterv1beta1.ConfigSpec{
						Version: "1.12.0+k8s-1.29-49-gf92daca6",
					},
					SourceType: embeddedclusterv1beta1.InstallationSourceTypeCRD,
					RuntimeConfig: &embeddedclusterv1beta1.RuntimeConfigSpec{
						K0sDataDirOverride:     "/var/lib/k0s",
						OpenEBSDataDirOverride: "/var/openebs",
					},
				},
			},
			wantErr: false,
			objects: []client.Object{
				&embeddedclusterv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20220000000000",
					},
					Spec: embeddedclusterv1beta1.InstallationSpec{
						Config: &embeddedclusterv1beta1.ConfigSpec{
							Version: "1.11.0+k8s-1.29-49-gf92daca6",
						},
					},
				},
				&embeddedclusterv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20241002205018",
					},
					Spec: embeddedclusterv1beta1.InstallationSpec{
						Config: &embeddedclusterv1beta1.ConfigSpec{
							Version: "1.13.0+k8s-1.29-49-gf92daca6",
						},
					},
				},
				&embeddedclusterv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20230000000000",
					},
					Spec: embeddedclusterv1beta1.InstallationSpec{
						Config: &embeddedclusterv1beta1.ConfigSpec{
							Version: "1.12.0+k8s-1.29-49-gf92daca6",
						},
					},
				},
				&embeddedclusterv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20210000000000",
					},
					Spec: embeddedclusterv1beta1.InstallationSpec{
						Config: &embeddedclusterv1beta1.ConfigSpec{
							Version: "1.10.0+k8s-1.29-49-gf92daca6",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.objects...).Build()

			got, err := GetPreviousInstallation(context.Background(), cli, tt.in)
			if tt.wantErr {
				req.Error(err)
				return
			}
			req.NoError(err)
			req.Equal(tt.want, got)
		})
	}
}

func TestGetInstallation(t *testing.T) {
	scheme := scheme.Scheme
	embeddedclusterv1beta1.AddToScheme(scheme)

	type args struct {
		name string
	}
	tests := []struct {
		name    string
		args    args
		want    *embeddedclusterv1beta1.Installation
		wantErr bool
		objects []client.Object
	}{
		{
			name: "migrates data dirs for previous versions prior to 1.15",
			args: args{
				name: "20241002205018",
			},
			want: &embeddedclusterv1beta1.Installation{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Installation",
					APIVersion: "embeddedcluster.replicated.com/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "20241002205018",
					ResourceVersion: "1000",
				},
				Spec: embeddedclusterv1beta1.InstallationSpec{
					Config: &embeddedclusterv1beta1.ConfigSpec{
						Version: "1.15.0+k8s-1.29-49-gf92daca6",
					},
					SourceType: embeddedclusterv1beta1.InstallationSourceTypeCRD,
					RuntimeConfig: &embeddedclusterv1beta1.RuntimeConfigSpec{
						K0sDataDirOverride:     "/var/lib/k0s",
						OpenEBSDataDirOverride: "/var/openebs",
					},
				},
			},
			wantErr: false,
			objects: []client.Object{
				&embeddedclusterv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20241002205018",
					},
					Spec: embeddedclusterv1beta1.InstallationSpec{
						Config: &embeddedclusterv1beta1.ConfigSpec{
							Version: "1.15.0+k8s-1.29-49-gf92daca6",
						},
					},
				},
				&embeddedclusterv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20231002205018",
					},
					Spec: embeddedclusterv1beta1.InstallationSpec{
						Config: &embeddedclusterv1beta1.ConfigSpec{
							Version: "1.14.0+k8s-1.29-49-gf92daca6",
						},
					},
				},
			},
		},
		{
			name: "does not migrate data dirs for previous version 1.15 or greater",
			args: args{
				name: "20241002205018",
			},
			want: &embeddedclusterv1beta1.Installation{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Installation",
					APIVersion: "embeddedcluster.replicated.com/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "20241002205018",
					ResourceVersion: "999",
				},
				Spec: embeddedclusterv1beta1.InstallationSpec{
					Config: &embeddedclusterv1beta1.ConfigSpec{
						Version: "1.15.1+k8s-1.29-49-gf92daca6",
					},
					SourceType: embeddedclusterv1beta1.InstallationSourceTypeCRD,
				},
			},
			wantErr: false,
			objects: []client.Object{
				&embeddedclusterv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20241002205018",
					},
					Spec: embeddedclusterv1beta1.InstallationSpec{
						Config: &embeddedclusterv1beta1.ConfigSpec{
							Version: "1.15.1+k8s-1.29-49-gf92daca6",
						},
					},
				},
				&embeddedclusterv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20231002205018",
					},
					Spec: embeddedclusterv1beta1.InstallationSpec{
						Config: &embeddedclusterv1beta1.ConfigSpec{
							Version: "1.15.0+k8s-1.29-49-gf92daca6",
						},
					},
				},
			},
		},
		{
			name: "does not migrate data dirs if no previous installation",
			args: args{
				name: "20241002205018",
			},
			want: &embeddedclusterv1beta1.Installation{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Installation",
					APIVersion: "embeddedcluster.replicated.com/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "20241002205018",
					ResourceVersion: "999",
				},
				Spec: embeddedclusterv1beta1.InstallationSpec{
					Config: &embeddedclusterv1beta1.ConfigSpec{
						Version: "1.15.0+k8s-1.29-49-gf92daca6",
					},
					SourceType: embeddedclusterv1beta1.InstallationSourceTypeCRD,
				},
			},
			wantErr: false,
			objects: []client.Object{
				&embeddedclusterv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "embeddedcluster.replicated.com/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20241002205018",
					},
					Spec: embeddedclusterv1beta1.InstallationSpec{
						Config: &embeddedclusterv1beta1.ConfigSpec{
							Version: "1.15.0+k8s-1.29-49-gf92daca6",
						},
						SourceType: embeddedclusterv1beta1.InstallationSourceTypeCRD,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.objects...).Build()

			got, err := GetInstallation(context.Background(), cli, tt.args.name)
			if tt.wantErr {
				req.Error(err)
				return
			}
			req.NoError(err)
			req.Equal(tt.want, got)
		})
	}
}

func TestWaitForPodDeleted(t *testing.T) {
	scheme := scheme.Scheme
	require.NoError(t, corev1.AddToScheme(scheme))

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
		},
	}

	tests := []struct {
		name        string
		objects     []client.Object
		cancelCtx   bool
		wantErr     bool
		errContains string
	}{
		{
			name:    "pod already deleted",
			objects: []client.Object{},
			wantErr: false,
		},
		{
			name:    "pod gets deleted during wait",
			objects: []client.Object{pod},
			wantErr: false,
		},
		{
			name:        "context canceled",
			objects:     []client.Object{pod},
			cancelCtx:   true,
			wantErr:     true,
			errContains: "context canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			builder := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...)

			client := builder.Build()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if tt.name == "pod gets deleted during wait" {
				// For the deletion test, start a goroutine that deletes the pod
				// after a short delay
				go func() {
					time.Sleep(100 * time.Millisecond)
					err := client.Delete(ctx, pod)
					req.NoError(err)
				}()
			}

			if tt.cancelCtx {
				// For the cancellation test, cancel the context after a short delay
				go func() {
					time.Sleep(100 * time.Millisecond)
					cancel()
				}()
			}

			ku := &KubeUtils{}

			// Use a short backoff for tests
			opts := &WaitOptions{
				Backoff: &wait.Backoff{
					Steps:    10,
					Duration: 50 * time.Millisecond,
					Factor:   1.0,
					Jitter:   0.0,
				},
			}

			err := ku.WaitForPodDeleted(ctx, client, "test-namespace", "test-pod", opts)

			if tt.wantErr {
				req.Error(err)
				if tt.errContains != "" {
					req.Contains(err.Error(), tt.errContains)
				}
			} else {
				req.NoError(err)
			}
		})
	}
}
