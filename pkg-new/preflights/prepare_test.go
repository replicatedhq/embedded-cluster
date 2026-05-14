package preflights

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_k8sVersionRequiresCgroupV2(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{
			name:    "empty version",
			version: "",
			want:    false,
		},
		{
			name:    "invalid version",
			version: "not-a-version",
			want:    false,
		},
		{
			name:    "k0s version 1.34",
			version: "v1.34.0+k0s.0",
			want:    false,
		},
		{
			name:    "k0s version 1.35",
			version: "v1.35.0+k0s.0",
			want:    true,
		},
		{
			name:    "k0s version 1.36",
			version: "v1.36.0+k0s.0",
			want:    true,
		},
		{
			name:    "EC version with k8s 1.34",
			version: "2.12.0+k8s-1.34-previous-stable",
			want:    false,
		},
		{
			name:    "EC version with k8s 1.35",
			version: "2.12.0+k8s-1.35",
			want:    true,
		},
		{
			name:    "EC version with k8s 1.36",
			version: "2.12.0+k8s-1.36-abcdef",
			want:    true,
		},
		{
			name:    "EC version without k8s metadata",
			version: "2.12.0",
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, k8sVersionRequiresCgroupV2(tt.version))
		})
	}
}
