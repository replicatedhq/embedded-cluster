package runtimeconfig

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEmbeddedClusterLogsPath tests the log path selection logic.
func TestEmbeddedClusterLogsPath(t *testing.T) {
	tests := []struct {
		name     string
		enableV3 string
		want     string
	}{
		{
			name:     "V2 returns static path",
			enableV3: "",
			want:     "/var/log/embedded-cluster",
		},
		{
			name:     "V2 with ENABLE_V3=0 returns static path",
			enableV3: "0",
			want:     "/var/log/embedded-cluster",
		},
		{
			name:     "V3 returns dynamic path with app slug",
			enableV3: "1",
			want:     filepath.Join("/var/log", AppSlug()),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ENABLE_V3", tt.enableV3)

			got := EmbeddedClusterLogsPath()

			assert.Equal(t, tt.want, got)
		})
	}
}
