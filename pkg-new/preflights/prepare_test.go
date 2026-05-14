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
		wantErr bool
	}{
		{
			name:    "empty version",
			version: "",
			want:    false,
			wantErr: false,
		},
		{
			name:    "invalid version",
			version: "not-a-version",
			want:    false,
			wantErr: true,
		},
		{
			name:    "k0s version 1.34",
			version: "v1.34.0+k0s.0",
			want:    false,
			wantErr: false,
		},
		{
			name:    "k0s version 1.35",
			version: "v1.35.0+k0s.0",
			want:    true,
			wantErr: false,
		},
		{
			name:    "k0s version 1.36",
			version: "v1.36.0+k0s.0",
			want:    true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := k8sVersionRequiresCgroupV2(tt.version)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
