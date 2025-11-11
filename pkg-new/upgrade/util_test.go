package upgrade

import (
	"testing"

	"github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/stretchr/testify/require"
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
