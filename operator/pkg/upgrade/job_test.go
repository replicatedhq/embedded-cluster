package upgrade

import (
	"context"
	clusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

func TestGetPreviousInstallation(t *testing.T) {
	scheme := scheme.Scheme
	clusterv1beta1.AddToScheme(scheme)

	tests := []struct {
		name    string
		in      *clusterv1beta1.Installation
		want    *clusterv1beta1.Installation
		wantErr bool
		objects []client.Object
	}{
		{
			name: "no installations at all",
			in: &clusterv1beta1.Installation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "20241002205018",
				},
			},
			want:    nil,
			wantErr: true,
			objects: []client.Object{},
		},
		{
			name: "no previous installation",
			in: &clusterv1beta1.Installation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "20241002205018",
				},
			},
			want:    nil,
			wantErr: true,
			objects: []client.Object{
				&clusterv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20241002205018",
					},
				},
			},
		},
		{
			name: "multiple previous installations",
			in: &clusterv1beta1.Installation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "20241002205018",
				},
			},
			want: &clusterv1beta1.Installation{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Installation",
					APIVersion: "v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "20230000000000",
					ResourceVersion: "999",
				},
			},
			wantErr: false,
			objects: []client.Object{
				&clusterv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20220000000000",
					},
				},
				&clusterv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20241002205018",
					},
				},
				&clusterv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20230000000000",
					},
				},
				&clusterv1beta1.Installation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Installation",
						APIVersion: "v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "20210000000000",
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
