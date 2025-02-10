package addons

import (
	"context"
	"testing"

	"github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v12 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_canEnableHA(t *testing.T) {
	scheme := scheme.Scheme
	v1beta1.AddToScheme(scheme)
	controllerLabels := map[string]string{"node-role.kubernetes.io/control-plane": "true"}
	type args struct {
		kcli client.Client
	}
	tests := []struct {
		name       string
		args       args
		want       bool
		wantReason string
		wantErr    bool
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
			want:       false,
			wantReason: "number of control plane nodes is less than 3",
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
			want:       false,
			wantReason: "already enabled",
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
			want:       false,
			wantReason: "a restore is in progress",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			assert := assert.New(t)
			ctx := context.Background()
			got, reason, err := CanEnableHA(ctx, tt.args.kcli)
			if tt.wantErr {
				req.Error(err)
				return
			}
			req.NoError(err)
			assert.Equal(tt.want, got)
			assert.Equal(tt.wantReason, reason)
		})
	}
}
