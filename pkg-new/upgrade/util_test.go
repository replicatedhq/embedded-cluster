package upgrade

import (
	"context"
	"testing"

	"github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_k0sVersionFromMetadata(t *testing.T) {
	tests := []struct {
		name string
		meta *types.ReleaseMetadata
		want string
	}{
		{
			name: "no version",
			meta: &types.ReleaseMetadata{},
			want: "",
		},
		{
			name: "no k0s version",
			meta: &types.ReleaseMetadata{
				Versions: map[string]string{},
			},
			want: "",
		},
		{
			name: "k0s version",
			meta: &types.ReleaseMetadata{
				Versions: map[string]string{
					"Kubernetes": "v1.29.9+k0s.0",
				},
			},
			want: "v1.29.9+k0s",
		},
		{
			name: "later k0s version",
			meta: &types.ReleaseMetadata{
				Versions: map[string]string{
					"Kubernetes": "v1.30.5+k0s.5",
				},
			},
			want: "v1.30.5+k0s",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			ver := k0sVersionFromMetadata(tt.meta)
			req.Equal(tt.want, ver)
		})
	}
}

func Test_clusterNodesMatchVersion(t *testing.T) {
	tests := []struct {
		name    string
		want    bool
		version string
		objects []runtime.Object
	}{
		{
			name:    "no nodes",
			want:    true,
			version: "irrelevant",
			objects: []runtime.Object{},
		},
		{
			name:    "one node, matches version",
			want:    true,
			version: "v1.29.9+k0s",
			objects: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							KubeletVersion: "v1.29.9+k0s",
						},
					},
				},
			},
		},
		{
			name:    "one node, doesn't match version",
			want:    false,
			version: "v1.29.9+k0s",
			objects: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							KubeletVersion: "v1.29.8+k0s",
						},
					},
				},
			},
		},
		{
			name:    "two nodes, one matches version",
			want:    false,
			version: "v1.30.5+k0s",
			objects: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							KubeletVersion: "v1.29.9+k0s",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							KubeletVersion: "v1.30.5+k0s",
						},
					},
				},
			},
		},
		{
			name:    "two nodes, both match version",
			want:    true,
			version: "v1.30.5+k0s",
			objects: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							KubeletVersion: "v1.30.5+k0s",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							KubeletVersion: "v1.30.5+k0s",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			cli := fake.NewFakeClient(tt.objects...)

			got, err := clusterNodesMatchVersion(context.Background(), cli, tt.version)
			req.NoError(err)
			req.Equal(tt.want, got)
		})
	}
}
