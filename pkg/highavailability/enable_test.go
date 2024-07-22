package highavailability

import (
	"context"
	"github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/constants"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v12 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

func Test_canEnableHA(t *testing.T) {
	scheme := scheme.Scheme
	v1beta1.AddToScheme(scheme)
	controllerLabels := map[string]string{"node-role.kubernetes.io/control-plane": "true"}
	type args struct {
		kcli client.Client
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "high availability is not enabled and there is three or more controller nodes",
			args: args{
				kcli: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
					&v1beta1.Installation{
						ObjectMeta: v1.ObjectMeta{Name: "test-installation"},
						Spec:       v1beta1.InstallationSpec{HighAvailability: false},
					},
					&v12.Node{ObjectMeta: v1.ObjectMeta{Name: "node1", Labels: controllerLabels}},
					&v12.Node{ObjectMeta: v1.ObjectMeta{Name: "node2", Labels: controllerLabels}},
					&v12.Node{ObjectMeta: v1.ObjectMeta{Name: "node3", Labels: controllerLabels}},
				).Build(),
			},
			want: true,
		},
		{
			name: "high availability is not enabled and there is not three or more controller nodes",
			args: args{
				kcli: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
					&v1beta1.Installation{
						ObjectMeta: v1.ObjectMeta{Name: "test-installation"},
						Spec:       v1beta1.InstallationSpec{HighAvailability: false},
					},
					&v12.Node{ObjectMeta: v1.ObjectMeta{Name: "node1", Labels: controllerLabels}},
					&v12.Node{ObjectMeta: v1.ObjectMeta{Name: "node2", Labels: controllerLabels}},
					&v12.Node{ObjectMeta: v1.ObjectMeta{Name: "node3"}},
				).Build(),
			},
			want: false,
		},
		{
			name: "high availability is already enabled",
			args: args{
				kcli: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
					&v1beta1.Installation{
						ObjectMeta: v1.ObjectMeta{Name: "test-installation"},
						Spec:       v1beta1.InstallationSpec{HighAvailability: true},
					},
					&v12.Node{ObjectMeta: v1.ObjectMeta{Name: "node1", Labels: controllerLabels}},
					&v12.Node{ObjectMeta: v1.ObjectMeta{Name: "node2", Labels: controllerLabels}},
					&v12.Node{ObjectMeta: v1.ObjectMeta{Name: "node3", Labels: controllerLabels}},
				).Build(),
			},
			want: false,
		},
		{
			name: "high availability is not enabled and there is three or more controller nodes but a restore is in progress",
			args: args{
				kcli: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
					&v1beta1.Installation{
						ObjectMeta: v1.ObjectMeta{Name: "test-installation"},
						Spec:       v1beta1.InstallationSpec{HighAvailability: false},
					},
					&v12.ConfigMap{
						ObjectMeta: v1.ObjectMeta{Name: constants.EcRestoreStateCMName, Namespace: "embedded-cluster"},
					},
					&v12.Node{ObjectMeta: v1.ObjectMeta{Name: "node1", Labels: controllerLabels}},
					&v12.Node{ObjectMeta: v1.ObjectMeta{Name: "node2", Labels: controllerLabels}},
					&v12.Node{ObjectMeta: v1.ObjectMeta{Name: "node3", Labels: controllerLabels}},
				).Build(),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			ctx := context.Background()
			got, err := CanEnableHA(ctx, tt.args.kcli)
			if tt.wantErr {
				req.Error(err)
				return
			}
			req.NoError(err)
			req.Equal(tt.want, got)
		})
	}
}

func Test_enableHA(t *testing.T) {
	scheme := scheme.Scheme
	v1beta1.AddToScheme(scheme)
	type args struct {
		kcli client.Client
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "happy path airgap",
			args: args{
				kcli: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
					&v1beta1.Installation{
						ObjectMeta: v1.ObjectMeta{Name: "test-installation"},
						Spec: v1beta1.InstallationSpec{
							HighAvailability: false,
							AirGap:           true,
							Network:          &v1beta1.NetworkSpec{ServiceCIDR: "10.123.0.0/16"},
						},
						Status: v1beta1.InstallationStatus{
							Conditions: []v1.Condition{
								{
									Type:   "HighAvailability",
									Status: v1.ConditionTrue,
								},
							},
							State: v1beta1.InstallationStateInstalled,
						},
					},
					&corev1.Namespace{
						ObjectMeta: v1.ObjectMeta{Name: "seaweedfs"},
						Status: corev1.NamespaceStatus{
							Phase: corev1.NamespaceActive,
						},
					},
				).Build(),
			},
		},
		{
			name: "happy path online",
			args: args{
				kcli: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
					&v1beta1.Installation{
						ObjectMeta: v1.ObjectMeta{Name: "test-installation"},
						Spec: v1beta1.InstallationSpec{
							HighAvailability: false,
							AirGap:           false,
						},
						Status: v1beta1.InstallationStatus{
							Conditions: []v1.Condition{
								{
									Type:   "HighAvailability",
									Status: v1.ConditionTrue,
								},
							},
							State: v1beta1.InstallationStateInstalled,
						},
					},
				).Build(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			ctx := context.Background()
			err := EnableHA(ctx, tt.args.kcli)
			if tt.wantErr {
				req.Error(err)
				return
			}
			req.NoError(err)
			// validate that high availability is enabled
			var installation v1beta1.Installation
			err = tt.args.kcli.Get(ctx, client.ObjectKey{Name: "test-installation"}, &installation)
			req.NoError(err)
			req.True(installation.Spec.HighAvailability)
		})
	}
}
